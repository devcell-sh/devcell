package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// NixReapSweepScript removes stale GC roots and orphaned profile
// generations from the nix volume. It does NOT run nix-store --gc: that
// lives in a separate container step so the GC gets a fresh memory budget
// (the previous monolith OOM-killed on large stores).
//
// CELL-334: reads *-meta files to identify and clean up stale roots
// (roots whose metadata exists but whose profile target is dangling).
// Reports stale root count for drift visibility.
//
// Exported for test assertions.
const NixReapSweepScript = `set -e
PROTECTED=""
STALE=0
if [ -d /nix/var/nix/gcroots/devcell ]; then
  for root in /nix/var/nix/gcroots/devcell/*-profile /nix/var/nix/gcroots/devcell/*-generation; do
    [ -L "$root" ] || continue
    target=$(readlink "$root")
    if [ -d "$target" ]; then
      PROTECTED="$PROTECTED $target"
    else
      hash=$(basename "$root" | cut -d- -f1)
      echo "stale root: $root -> $target (removing)"
      rm -f "/nix/var/nix/gcroots/devcell/${hash}-profile"
      rm -f "/nix/var/nix/gcroots/devcell/${hash}-generation"
      rm -f "/nix/var/nix/gcroots/devcell/${hash}-meta"
      STALE=$((STALE + 1))
    fi
  done
  for meta in /nix/var/nix/gcroots/devcell/*-meta; do
    [ -f "$meta" ] || continue
    hash=$(basename "$meta" | cut -d- -f1)
    if [ ! -L "/nix/var/nix/gcroots/devcell/${hash}-profile" ]; then
      echo "orphaned metadata: $meta (removing)"
      rm -f "$meta"
    fi
  done
fi
if [ -z "$PROTECTED" ]; then
  echo "No project GC roots found in /nix/var/nix/gcroots/devcell/ — skipping safe prune (use --force for blanket cleanup)"
  exit 0
fi
if [ "$STALE" -gt 0 ]; then
  echo "Cleaned $STALE stale root(s)"
fi
REMOVED=0
for gen in /nix/var/nix/profiles/per-user/root/profile-*-link; do
  [ -L "$gen" ] || continue
  target=$(readlink "$gen")
  if ! echo "$PROTECTED" | grep -qF "$target"; then
    rm -v "$gen" && REMOVED=$((REMOVED + 1))
  fi
done
echo "Removed $REMOVED orphaned profile generations"`

// NixGCRootReportScript prints the current GC root state: how many roots,
// how many unique hashes, and details from each -meta file. Used by the
// prune preflight to give drift visibility before destructive operations.
// Exported for test assertions.
const NixGCRootReportScript = `set -e
echo "=== Nix GC Root Report ==="
ROOT_COUNT=0
HASHES=""
for root in /nix/var/nix/gcroots/devcell/*-profile; do
  [ -L "$root" ] || continue
  ROOT_COUNT=$((ROOT_COUNT + 1))
  h=$(basename "$root" | cut -d- -f1)
  case " $HASHES " in
    *" $h "*) ;;
    *) HASHES="$HASHES $h" ;;
  esac
done
UNIQUE=$(echo "$HASHES" | wc -w)
echo "Roots: $ROOT_COUNT (${UNIQUE} unique profile hash(es))"
for meta in /nix/var/nix/gcroots/devcell/*-meta; do
  [ -f "$meta" ] || continue
  h=$(basename "$meta" | cut -d- -f1)
  proj=$(grep '^project=' "$meta" 2>/dev/null | cut -d= -f2)
  stack=$(grep '^stack=' "$meta" 2>/dev/null | cut -d= -f2)
  stamped=$(grep '^stamped=' "$meta" 2>/dev/null | cut -d= -f2)
  echo "  $h: project=$proj stack=$stack stamped=$stamped"
done
if [ "$UNIQUE" -gt 1 ]; then
  echo ""
  echo "WARNING: config drift detected — $UNIQUE different profile hashes"
  echo "  Different hashes mean different nixpkgs revisions anchored on disk."
  echo "  Rebuild all cells with the same flake.lock to converge and reclaim space."
fi`

// `cell build prune` cleanup planner.
//
// Builders compose the ordered plan of commands for each prune mode.
// The runtime layer (RunDockerPrune / RunNixPrune) executes the plan
// after a confirmation prompt. All host-specific branching lives in the
// builders so unit tests can pin behavior without invoking real commands.
//
// See CELL-101 for the design rationale.

// DiskUsageRow is one row from `docker system df`.
type DiskUsageRow struct {
	Size        string // e.g. "11.5GB"
	Reclaimable string // e.g. "10.16GB (88%)"
	Total       int    // total count
	Active      int    // active (in-use) count
}

// RunningCell describes a running devcell container for the prune prompt.
type RunningCell struct {
	Name   string // container name (e.g. "devcell-modd-1113")
	Cell   string // cell name extracted from container name
	Size   string // writable layer size (e.g. "97.1MB")
	Uptime string // e.g. "Up 34 hours"
}

// DiskUsage holds pre-queried sizes from `docker system df` for display in
// the prune prompt. Zero value means the query failed or was skipped.
type DiskUsage struct {
	Images     DiskUsageRow
	Containers DiskUsageRow
	Volumes    DiskUsageRow
	BuildCache DiskUsageRow
	// Pinned explains why non-reclaimable space exists.
	Pinned []string
	// RunningCells lists running devcell containers for the table.
	RunningCells []RunningCell
}

// QueryDiskUsage runs `docker system df --format` and parses the output into
// a DiskUsage struct. Returns a zero DiskUsage (not an error) if the command
// fails or times out, so the caller can always embed it in PruneOpts.
func QueryDiskUsage(ctx context.Context) DiskUsage {
	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx, "docker", "system", "df", "--format",
		"{{.Type}}\t{{.TotalCount}}\t{{.Active}}\t{{.Size}}\t{{.Reclaimable}}").Output()
	if err != nil {
		return DiskUsage{}
	}
	var du DiskUsage
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		typ := fields[0]
		total := parseCount(fields[1])
		active := parseCount(fields[2])
		size := fields[3]
		recl := fields[4]
		row := DiskUsageRow{Size: size, Reclaimable: recl, Total: total, Active: active}
		switch {
		case strings.HasPrefix(typ, "Images"):
			du.Images = row
		case strings.HasPrefix(typ, "Containers"):
			du.Containers = row
		case strings.HasPrefix(typ, "Local Volumes"):
			du.Volumes = row
		case strings.HasPrefix(typ, "Build Cache"):
			du.BuildCache = row
		}
	}

	du.Pinned = buildPinnedExplanations(du)
	return du
}

// QueryRunningCells lists running devcell containers with their sizes.
func QueryRunningCells(ctx context.Context) []RunningCell {
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx, "docker", "ps",
		"--filter", "name=devcell-",
		"--format", "{{.Names}}\t{{.Size}}\t{{.Status}}").Output()
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil
	}
	var cells []RunningCell
	for _, line := range strings.Split(text, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		// Skip non-devcell containers (buildkit, etc.)
		if !strings.HasPrefix(name, "devcell-") {
			continue
		}
		size := fields[1]
		// Size format: "97.1MB (virtual 1.72GB)" — keep only writable part
		if idx := strings.Index(size, " ("); idx > 0 {
			size = size[:idx]
		}
		uptime := fields[2]
		// Status format: "Up 34 hours" — keep as-is
		cell := strings.TrimPrefix(name, "devcell-")
		// Strip the -run suffix if present
		cell = strings.TrimSuffix(cell, "-run")
		cells = append(cells, RunningCell{
			Name:   name,
			Cell:   cell,
			Size:   size,
			Uptime: uptime,
		})
	}
	return cells
}

// formatCellTable renders running cells as an aligned table string with
// indent for embedding in the prompt.
func formatCellTable(cells []RunningCell) string {
	// Find max widths for alignment
	cellW, sizeW := 4, 4 // "CELL", "SIZE"
	for _, c := range cells {
		if len(c.Cell) > cellW {
			cellW = len(c.Cell)
		}
		if len(c.Size) > sizeW {
			sizeW = len(c.Size)
		}
	}
	indent := "     "
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%-*s  %-*s  %s\n", indent, cellW, "CELL", sizeW, "SIZE", "UPTIME"))
	for _, c := range cells {
		b.WriteString(fmt.Sprintf("%s%-*s  %-*s  %s\n", indent, cellW, c.Cell, sizeW, c.Size, c.Uptime))
	}
	return b.String()
}

func buildPinnedExplanations(du DiskUsage) []string {
	var pinned []string
	if du.Containers.Active > 0 {
		pinned = append(pinned,
			fmt.Sprintf("%d running container(s) pin their images and volumes", du.Containers.Active))
	}
	if du.Images.Active > 0 && du.Images.Total > du.Images.Active {
		pinned = append(pinned,
			fmt.Sprintf("%d of %d images in use, %d unused",
				du.Images.Active, du.Images.Total, du.Images.Total-du.Images.Active))
	}
	if du.Volumes.Active > 0 {
		pinned = append(pinned,
			fmt.Sprintf("%d of %d volumes in use (includes nix store)",
				du.Volumes.Active, du.Volumes.Total))
	}
	return pinned
}

func parseCount(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// PruneOpts describes a `cell build prune` invocation.
type PruneOpts struct {
	// GOOS is the target operating system: "darwin" or "linux".
	// Tests pass this explicitly; the runtime call site passes runtime.GOOS.
	GOOS string

	// Force enables nuclear cleanup mode (docker desktop wipe / qcow nuke /
	// aggressive nix GC). Default mode is the standard prune sequence.
	Force bool

	// Rootless indicates a rootless Docker daemon on Linux. Only relevant
	// when GOOS == "linux" && Force. The runtime call site auto-detects
	// this via `docker info`.
	Rootless bool

	// NixOS indicates /etc/NIXOS is present. Only relevant when
	// GOOS == "linux" && Force; triggers system-profile cleanup.
	NixOS bool

	// HomeDir is the resolved user home (e.g. /home/warden). Used to
	// compose absolute paths in macOS Docker Desktop wipe and the
	// `~/.cache/nix` wipe on Linux. Tests pass a fixed value.
	HomeDir string

	// LinuxBuilderHost is the SSH target for the macOS linux-builder VM.
	// Default: "builder@linux-builder".
	LinuxBuilderHost string

	// LiveClosures are the running containers' resolved store paths
	// (CollectLiveClosures). When set, the prune plan stamps GC roots
	// for them before the safe GC step, so "running implies rooted" holds
	// by construction instead of being assumed (CELL-334 preflight gate).
	LiveClosures []LiveClosure

	// SkipReapSweep skips the reap/sweep step when the scan found nothing
	// reclaimable (all hashes are live, no stale roots, no orphaned gens).
	// GC still runs to reclaim unreferenced store paths.
	SkipReapSweep bool

	// DiskUsage holds pre-queried sizes for the prompt. Zero value means
	// sizes are omitted from the prompt text.
	DiskUsage DiskUsage
}

// PruneStep is one command in a prune plan.
type PruneStep struct {
	// Label is a short human-readable description shown in progress
	// spinners (e.g. "Pruning containers"). Every step must have one.
	Label string

	// Argv is the command and arguments.
	Argv []string

	// Func, when non-nil, is called instead of exec'ing Argv. Use this
	// to replace shell-substituted commands with pure Go. When set, Argv
	// is still joined for display (progress/dry-run output).
	Func func(ctx context.Context) error

	// IgnoreError lets the step fail without aborting the plan
	// (e.g. `docker rm` with no containers exits non-zero).
	IgnoreError bool

	// DryRun, when true, signals the runtime to print the argv as a
	// `# (dry-run)` comment instead of executing. Used for the macOS
	// linux-builder qcow nuke until paths are verified on a live setup.
	DryRun bool
}

// BuildDockerPruneSteps composes the docker prune plan. Default mode is
// the cleandocker sequence (identical on Darwin and Linux). Force mode
// branches by OS.
func BuildDockerPruneSteps(opts PruneOpts) []PruneStep {
	if !opts.Force {
		return []PruneStep{
			{Label: "Pruning containers", Argv: []string{"docker", "container", "prune", "-f"}},
			{Label: "Pruning images", Argv: []string{"docker", "image", "prune", "-af"}},
			{Label: "Pruning volumes", Argv: []string{"docker", "volume", "prune", "-f"}},
			{Label: "Pruning build cache", Argv: []string{"docker", "buildx", "prune", "-af"}},
		}
	}
	// Force mode: nuclear Docker reset. Branches by OS because the
	// daemon lives in different places.
	if opts.GOOS == "darwin" {
		// Docker Desktop on macOS: stop, wipe the VM data dir, start.
		// Mirrors the user's `cleandocker -f` zsh function exactly.
		wipePath := opts.HomeDir + "/Library/Containers/com.docker.docker/Data/vms/0/data"
		return []PruneStep{
			{Label: "Stopping Docker Desktop", Argv: []string{"docker", "desktop", "stop"}},
			{Label: "Wiping Docker data", Argv: []string{"rm", "-rf", wipePath}},
			{Label: "Starting Docker Desktop", Argv: []string{"docker", "desktop", "start"}},
		}
	}
	// Linux native: rootful uses /var/lib/docker via the system docker
	// service; rootless uses ~/.local/share/docker via the user service.
	if opts.Rootless {
		wipePath := opts.HomeDir + "/.local/share/docker"
		return []PruneStep{
			{Label: "Stopping Docker (rootless)", Argv: []string{"systemctl", "--user", "stop", "docker"}},
			{Label: "Wiping Docker data", Argv: []string{"rm", "-rf", wipePath}},
			{Label: "Starting Docker (rootless)", Argv: []string{"systemctl", "--user", "start", "docker"}},
		}
	}
	return []PruneStep{
		{Label: "Stopping Docker daemon", Argv: []string{"sudo", "systemctl", "stop", "docker", "docker.socket"}},
		{Label: "Wiping Docker data", Argv: []string{"sudo", "rm", "-rf", "/var/lib/docker"}},
		{Label: "Starting Docker daemon", Argv: []string{"sudo", "systemctl", "start", "docker"}},
	}
}

// BuildNixPruneSteps composes the nix prune plan. Default mode runs
// `nix-collect-garbage -d` + `nix-store --optimise` (via ssh on macOS,
// locally on Linux native). Force mode branches by OS (Darwin: dry-run
// qcow nuke plan; Linux: aggressive local GC).
func BuildNixPruneSteps(opts PruneOpts) []PruneStep {
	host := opts.LinuxBuilderHost
	if host == "" {
		host = "builder@linux-builder"
	}

	// Always clear the ephemeral registry blob cache.
	registryCleanup := PruneStep{
		Label:       "Cleaning registry cache",
		Argv:        []string{"rm", "-rf", opts.HomeDir + "/.devcell/registry"},
		IgnoreError: true,
	}

	if !opts.Force {
		// Safe mode targets the devcell-nix-store volume on every OS
		// (CELL-333): run safe project-aware GC inside a container with the
		// nix volume mounted. The root report is now shown in the preflight
		// (execCleanBuild) before the confirmation prompt, so it's not
		// repeated here. On macOS the nix store lives in this Docker
		// volume, not the linux-builder VM — GC-ing the VM never reclaims it.
		var steps []PruneStep
		// Preflight gate (CELL-334): stamp roots for every running
		// container before GC, so a running-but-unrooted cell cannot lose
		// its closure to the sweep below.
		if stamp := StampRootsScript(opts.LiveClosures); stamp != "" {
			steps = append(steps, PruneStep{Label: "Stamping GC roots for running containers", Argv: []string{
				"docker", "run", "--rm",
				"-v", DefaultNixStoreVolume + ":/nix",
				NixCoreImage,
				"/bin/sh", "-c", stamp,
			}})
		}
		if !opts.SkipReapSweep {
			steps = append(steps, PruneStep{Label: "Reaping stale roots and orphaned generations", Argv: []string{
				"docker", "run", "--rm",
				"-v", DefaultNixStoreVolume + ":/nix",
				NixCoreImage,
				"/bin/sh", "-c", NixReapSweepScript,
			}})
		}
		steps = append(steps,
			PruneStep{Label: "Running nix garbage collection", Argv: []string{
				"docker", "run", "--rm",
				"-v", DefaultNixStoreVolume + ":/nix",
				NixCoreImage,
				"/bin/sh", "-c", "nix-store --gc",
			}},
			registryCleanup,
		)
		return steps
	}
	// Force mode.
	if opts.GOOS == "darwin" {
		// Darwin: nuke linux-builder VM qcow + restart. Dry-run only until
		// paths are verified on a live setup (NukeBuilderVMEnabled gate).
		// The actual qcow path is resolved at runtime via launchctl/ps and
		// stamped into the printed plan; in the builder we use a clearly-
		// marked placeholder.
		const qcowPlaceholder = "<resolved-qcow-path>"
		const service = "system/org.nixos.linux-builder"
		plan := []PruneStep{
			{Label: "Stopping linux-builder VM", Argv: []string{"sudo", "launchctl", "bootout", service}, DryRun: true},
			{Label: "Deleting linux-builder qcow", Argv: []string{"sudo", "rm", "-f", qcowPlaceholder}, DryRun: true},
			{Label: "Restarting linux-builder VM", Argv: []string{"sudo", "launchctl", "kickstart", "-k", service}, DryRun: true},
			{
				Label: "Waiting for linux-builder SSH",
				Argv: []string{
					"/bin/sh", "-c",
					"until ssh -o ConnectTimeout=2 " + host + " true 2>/dev/null; do sleep 2; done",
				},
				DryRun: true,
			},
		}
		plan = append(plan, registryCleanup)
		return plan
	}
	// Linux native: aggressive local GC ceiling (no VM to nuke).
	// CRITICAL: never `rm -rf /nix/store` — would destroy NixOS or require
	// reinstall on standalone nix.
	steps := []PruneStep{
		{Label: "Deleting old nix generations", Argv: []string{"sudo", "nix-env", "--delete-generations", "old"}},
		{Label: "Running nix garbage collection", Argv: []string{"sudo", "nix-collect-garbage", "-d"}},
		{Label: "Optimising nix store", Argv: []string{"sudo", "nix-store", "--optimise"}},
		{Label: "Clearing nix cache", Argv: []string{"rm", "-rf", opts.HomeDir + "/.cache/nix"}},
	}
	if opts.NixOS {
		steps = append(steps, PruneStep{
			Label: "Deleting NixOS system generations",
			Argv: []string{
				"sudo", "nix-env", "--delete-generations", "old",
				"--profile", "/nix/var/nix/profiles/system",
			},
		})
	}
	steps = append(steps, registryCleanup)
	return steps
}

// dockerRemoveAllContainers lists all container IDs and force-removes them.
// Replaces the shell-dependent `sh -c "docker rm -f $(docker ps -aq)"` so
// devcell works on hosts without sh in PATH (NixOS, pure-nix setups).
func dockerRemoveAllContainers(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "docker", "ps", "-aq").Output()
	if err != nil {
		return nil
	}
	ids := strings.Fields(strings.TrimSpace(string(out)))
	if len(ids) == 0 {
		return nil
	}
	args := append([]string{"rm", "-f"}, ids...)
	return exec.CommandContext(ctx, "docker", args...).Run()
}

// RunPruneArgs bundles inputs to RunPrune. Keeps the call site readable
// when callers wire in real stdin/stdout/exec.
type RunPruneArgs struct {
	Opts    PruneOpts
	Exec    func(step PruneStep) error
	Out     io.Writer
	In      io.Reader
	SkipYes bool // --yes / -y
	IsTTY   bool // term.IsTerminal(int(os.Stdin.Fd()))
	Verbose bool // show each command + its output (--debug)

	// OnExecStart is called after the user confirms but before
	// the first step executes. Nil is safe.
	OnExecStart func()
	// OnStepStart is called before each step executes (including
	// dry-run steps). Receives the step's Label. Nil is safe.
	OnStepStart func(label string)
	// OnExecDone is called after all steps complete (success or
	// failure). Receives nil on success. Nil is safe.
	OnExecDone func(err error)
}

// RunPrune orchestrates: build the step plan, prompt the user, execute the
// steps. DryRun steps are printed but not executed. IgnoreError steps don't
// abort the loop on failure. Returns nil if the prompt was rejected (not an
// error — user intent).
func RunPrune(a RunPruneArgs) error {
	steps := append(BuildDockerPruneSteps(a.Opts), BuildNixPruneSteps(a.Opts)...)
	prompt := BuildPrunePrompt(a.Opts)
	if !ConfirmDestructive(a.Out, a.In, a.SkipYes, a.IsTTY, prompt) {
		fmt.Fprintln(a.Out, "Aborted, nothing was deleted.")
		return nil
	}
	if a.OnExecStart != nil {
		a.OnExecStart()
	}
	var execErr error
	defer func() {
		if a.OnExecDone != nil {
			a.OnExecDone(execErr)
		}
	}()
	for _, step := range steps {
		if a.OnStepStart != nil {
			a.OnStepStart(step.Label)
		}
		argv := strings.Join(step.Argv, " ")
		if step.DryRun {
			fmt.Fprintln(a.Out, "# (dry-run) "+argv)
			continue
		}
		if a.Verbose {
			fmt.Fprintln(a.Out, "→ "+argv)
		}
		if err := a.Exec(step); err != nil {
			if step.IgnoreError {
				if a.Verbose {
					fmt.Fprintf(a.Out, "  (ignored: %v)\n", err)
				}
				continue
			}
			execErr = fmt.Errorf("prune step failed: %s: %w", argv, err)
			return execErr
		}
	}
	return nil
}

// ConfirmDestructive prints `warning` to out, reads one line from in, and
// returns true iff the user typed y/Y/yes/YES (case-insensitive, trimmed).
//
// skipYes=true bypasses the prompt entirely (used for `-y` / `--yes`). The
// warning is not printed in this case — the user already opted in.
//
// isTTY=false without skipYes refuses to proceed (returns false after
// printing a refusal message that mentions --yes). This prevents accidental
// destruction when stdin is piped (e.g. `echo y | cell build prune --force`).
func ConfirmDestructive(out io.Writer, in io.Reader, skipYes bool, isTTY bool, warning string) bool {
	if skipYes {
		return true
	}
	if !isTTY {
		fmt.Fprintln(out, warning)
		fmt.Fprintln(out, "refusing to run destructive action without --yes on non-interactive stdin")
		return false
	}
	fmt.Fprintln(out, warning)
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		// EOF or read error — treat as rejection.
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

// BuildPrunePrompt returns the user-facing confirmation warning text for
// the given prune mode. All prompts share the format:
//
//	⚠  This will delete ALL <specific-list>.
//	   Target: <host-specific-detail>
//	   Continue? [y/N]
//
// The exact list and target line are mode-specific, per CELL-101.
func BuildPrunePrompt(opts PruneOpts) string {
	const tail = "   Continue? [y/N]"

	if !opts.Force {
		du := opts.DiskUsage
		sizeBlock := ""
		if du.Images.Size != "" || du.Volumes.Size != "" {
			sizeBlock = "\n   Current disk usage:\n"
			if du.Images.Size != "" {
				sizeBlock += fmt.Sprintf("     Images:      %-10s reclaimable: %s\n", du.Images.Size, du.Images.Reclaimable)
			}
			if du.Containers.Size != "" {
				sizeBlock += fmt.Sprintf("     Containers:  %-10s reclaimable: %s\n", du.Containers.Size, du.Containers.Reclaimable)
			}
			if du.Volumes.Size != "" {
				sizeBlock += fmt.Sprintf("     Volumes:     %-10s reclaimable: %s\n", du.Volumes.Size, du.Volumes.Reclaimable)
			}
			if du.BuildCache.Size != "" {
				sizeBlock += fmt.Sprintf("     Build cache: %-10s reclaimable: %s\n", du.BuildCache.Size, du.BuildCache.Reclaimable)
			}
			if len(du.Pinned) > 0 || len(du.RunningCells) > 0 {
				sizeBlock += "\n   Non-reclaimable:\n"
				for _, reason := range du.Pinned {
					sizeBlock += "     - " + reason + "\n"
				}
				if len(du.RunningCells) > 0 {
					sizeBlock += "\n   Running cells:\n"
					sizeBlock += formatCellTable(du.RunningCells)
				}
			}
		}
		return "⚠  This will:\n" +
			"   Docker: remove stopped containers, unused images, dangling volumes, BuildKit cache.\n" +
			"           Running containers are not touched.\n" +
			"   Nix:    remove orphaned profile generations and unreferenced store paths.\n" +
			"           Running containers' nix closures are protected.\n" +
			"   Target: local Docker daemon + " + DefaultNixStoreVolume + " volume (/nix)\n" +
			sizeBlock +
			tail
	}

	// Force mode: nuclear reset of both Docker and nix.
	host := opts.LinuxBuilderHost
	if host == "" {
		host = "builder@linux-builder"
	}

	if opts.GOOS == "darwin" {
		wipe := opts.HomeDir + "/Library/Containers/com.docker.docker/Data/vms/0/data"
		return fmt.Sprintf(
			"⚠  This will delete ALL Docker and nix state:\n"+
				"   Docker: STOP Docker Desktop, wipe its data directory, restart.\n"+
				"           Target: %s\n"+
				"   Nix:    STOP linux-builder VM, DELETE its qcow disk image,\n"+
				"           restart. VM rebuilds on next build.\n"+
				"           Target: linux-builder VM (%s)\n"+
				tail,
			wipe, host,
		)
	}

	// Linux force.
	dockerTarget := "/var/lib/docker"
	modeNote := "(rootful, via systemctl + sudo)"
	if opts.Rootless {
		dockerTarget = opts.HomeDir + "/.local/share/docker"
		modeNote = "(rootless, via systemctl --user)"
	}
	nixosLine := ""
	if opts.NixOS {
		nixosLine = "   Also: NixOS system-profile old generations.\n"
	}
	return fmt.Sprintf(
		"⚠  This will delete ALL Docker and nix state:\n"+
			"   Docker: STOP daemon, wipe data directory, restart.\n"+
			"           Target: %s %s\n"+
			"   Nix:    delete old profile generations, aggressive GC, optimise,\n"+
			"           wipe %s/.cache/nix.\n"+
			"%s"+
			"   Note: --force cannot wipe /nix/store safely; this is the aggressive\n"+
			"   GC ceiling. For total reset, reinstall nix manually.\n"+
			tail,
		dockerTarget, modeNote, opts.HomeDir, nixosLine,
	)
}

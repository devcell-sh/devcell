package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// SafeNixGCScript is the shell script that performs project-aware nix
// garbage collection. It removes only orphaned profile generations that
// aren't protected by project-scoped GC roots, then runs nix-store --gc.
//
// CELL-334: also reads *-meta files to identify and clean up stale roots
// (roots whose metadata exists but whose profile target is dangling).
// Reports stale root count for drift visibility.
//
// Exported for test assertions.
const SafeNixGCScript = `set -e
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
echo "Removed $REMOVED orphaned profile generations"
nix-store --gc`

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

// PruneOpts describes a `cell build prune` invocation.
type PruneOpts struct {
	// GOOS is the target operating system: "darwin" or "linux".
	// Tests pass this explicitly; the runtime call site passes runtime.GOOS.
	GOOS string

	// Force enables nuclear cleanup mode (docker desktop wipe / qcow nuke /
	// aggressive nix GC). Default mode is the standard prune sequence.
	Force bool

	// Pure is deprecated: the default now prunes both Docker and nix.
	// Retained for backwards compatibility; ignored by RunPrune.
	Pure bool

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
	// (CollectLiveClosures). When set, the pure prune plan stamps GC roots
	// for them before the safe GC step, so "running implies rooted" holds
	// by construction instead of being assumed (CELL-334 preflight gate).
	LiveClosures []LiveClosure
}

// PruneStep is one command in a prune plan.
type PruneStep struct {
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
			{
				Argv:        []string{"docker", "rm", "-f", "$(docker ps -aq)"},
				Func:        dockerRemoveAllContainers,
				IgnoreError: true,
			},
			{Argv: []string{"docker", "system", "prune", "-af"}},
			{Argv: []string{"docker", "volume", "prune", "-f"}},
			{Argv: []string{"docker", "buildx", "prune", "-af"}},
		}
	}
	// Force mode: nuclear Docker reset. Branches by OS because the
	// daemon lives in different places.
	if opts.GOOS == "darwin" {
		// Docker Desktop on macOS: stop, wipe the VM data dir, start.
		// Mirrors the user's `cleandocker -f` zsh function exactly.
		wipePath := opts.HomeDir + "/Library/Containers/com.docker.docker/Data/vms/0/data"
		return []PruneStep{
			{Argv: []string{"docker", "desktop", "stop"}},
			{Argv: []string{"rm", "-rf", wipePath}},
			{Argv: []string{"docker", "desktop", "start"}},
		}
	}
	// Linux native: rootful uses /var/lib/docker via the system docker
	// service; rootless uses ~/.local/share/docker via the user service.
	if opts.Rootless {
		wipePath := opts.HomeDir + "/.local/share/docker"
		return []PruneStep{
			{Argv: []string{"systemctl", "--user", "stop", "docker"}},
			{Argv: []string{"rm", "-rf", wipePath}},
			{Argv: []string{"systemctl", "--user", "start", "docker"}},
		}
	}
	return []PruneStep{
		{Argv: []string{"sudo", "systemctl", "stop", "docker", "docker.socket"}},
		{Argv: []string{"sudo", "rm", "-rf", "/var/lib/docker"}},
		{Argv: []string{"sudo", "systemctl", "start", "docker"}},
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
		Argv:        []string{"rm", "-rf", opts.HomeDir + "/.devcell/registry"},
		IgnoreError: true,
	}

	if !opts.Force {
		// Safe mode targets the devcell-nix-store volume on every OS
		// (CELL-333): show root report (drift detection, CELL-334), then run
		// safe project-aware GC inside a container with the nix volume
		// mounted. On macOS the thin-mode store lives in this Docker volume,
		// not the linux-builder VM — GC-ing the VM never reclaims it.
		steps := []PruneStep{
			{Argv: []string{
				"docker", "run", "--rm",
				"-v", DefaultThinStoreVolume + ":/nix",
				NixCoreImage,
				"/bin/sh", "-c", NixGCRootReportScript,
			}, IgnoreError: true},
		}
		// Preflight gate (CELL-334): stamp roots for every running
		// container before GC, so a running-but-unrooted cell cannot lose
		// its closure to the sweep below.
		if stamp := StampRootsScript(opts.LiveClosures); stamp != "" {
			steps = append(steps, PruneStep{Argv: []string{
				"docker", "run", "--rm",
				"-v", DefaultThinStoreVolume + ":/nix",
				NixCoreImage,
				"/bin/sh", "-c", stamp,
			}})
		}
		steps = append(steps,
			PruneStep{Argv: []string{
				"docker", "run", "--rm",
				"-v", DefaultThinStoreVolume + ":/nix",
				NixCoreImage,
				"/bin/sh", "-c", SafeNixGCScript,
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
			{Argv: []string{"sudo", "launchctl", "bootout", service}, DryRun: true},
			{Argv: []string{"sudo", "rm", "-f", qcowPlaceholder}, DryRun: true},
			{Argv: []string{"sudo", "launchctl", "kickstart", "-k", service}, DryRun: true},
			{
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
		{Argv: []string{"sudo", "nix-env", "--delete-generations", "old"}},
		{Argv: []string{"sudo", "nix-collect-garbage", "-d"}},
		{Argv: []string{"sudo", "nix-store", "--optimise"}},
		{Argv: []string{"rm", "-rf", opts.HomeDir + "/.cache/nix"}},
	}
	if opts.NixOS {
		steps = append(steps, PruneStep{
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
	for _, step := range steps {
		argv := strings.Join(step.Argv, " ")
		if step.DryRun {
			fmt.Fprintln(a.Out, "# (dry-run) "+argv)
			continue
		}
		fmt.Fprintln(a.Out, "→ "+argv)
		if err := a.Exec(step); err != nil {
			if step.IgnoreError {
				fmt.Fprintf(a.Out, "  (ignored: %v)\n", err)
				continue
			}
			return fmt.Errorf("prune step failed: %s: %w", argv, err)
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
		return "⚠  This will:\n" +
			"   Docker: remove stopped containers, unused images, volumes, BuildKit cache.\n" +
			"   Nix:    remove orphaned profile generations and unreferenced store paths.\n" +
			"           Running containers and their closures are protected.\n" +
			"   Target: local Docker daemon + " + DefaultThinStoreVolume + " volume (/nix)\n" +
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

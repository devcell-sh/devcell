// Package docker is the docker engine: it builds `docker run`, `docker exec`
// and thin-image build argv, manages images, the shared nix-store volume and
// devcell containers, watches in-container boot progress, and implements
// `cell build df`, `cell build prune` and `cell cleanup`.
package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
)

const (
	// DefaultRegistry is the fallback registry prefix for devcell images.
	DefaultRegistry = "ghcr.io/devcell-sh/devcell"

	// BootContainerPath is the container-side path where the host boot
	// directory is bind-mounted. The Go-generated init script lives here.
	BootContainerPath = "/tmp/devcell-boot"
)

// Registry is the active container registry. Set via cfg.ResolvedRegistry()
// at startup; defaults to DefaultRegistry.
var Registry = DefaultRegistry

// Stack is the resolved nix stack name (e.g. "ultimate", "go").
// Set from CellConfig at startup; defaults to "base".
var Stack = "base"

// Modules is the list of extra nix modules composed on top of the stack.
// Set from CellConfig at startup.
var Modules []string

// PerCellImage tags user images per cell instead of per stack.
// Set from CellConfig at startup; defaults to false (stack-based).
var PerCellImage bool

// FlakeNixImage returns the lightweight nixos/nix image used for flake
// lock/update and stack discovery. Decoupled from the binary version so
// dev builds with -dirty tags don't require a registry push.
func FlakeNixImage() string {
	return cfg.NixSection{}.ResolvedImage()
}

// UserImageTag returns the user's local image tag — the bare-name local
// devcell image. Unchanged across the 2026-05-15 flip: this name is the
// user's "current image" concept and existing local images keep working.
//
// Default (stack-based): devcell-user:<stack> or devcell-user:<stack>-<mod1>-<mod2>-<sha8>
// Legacy (per_cell_image=true): devcell-user:<cell>
//
// Used by:
//   - `cell <agent> --impure` (PickImageTag(true) → bare local tag)
//   - `cell build --impure` (docker build → tags this name)
//
// Override with DEVCELL_USER_IMAGE env var.
func UserImageTag() string {
	if tag := os.Getenv("DEVCELL_USER_IMAGE"); tag != "" {
		return tag
	}
	if PerCellImage {
		return "devcell-user:" + config.ResolveCellName(os.Getenv)
	}
	tag := Stack
	if tag == "" {
		tag = "base"
	}
	if len(Modules) > 0 {
		sorted := make([]string, len(Modules))
		copy(sorted, Modules)
		sort.Strings(sorted)
		tag += "-" + strings.Join(sorted, "-")
		h := sha256.Sum256([]byte(strings.Join(sorted, ",")))
		tag += "-" + hex.EncodeToString(h[:])[:8]
	}
	return "devcell-user:" + tag
}

// UserImageTagPure returns the local tag for nix2container-built (pure)
// images — the DEFAULT path after the 2026-05-15 flip (CELL-189). Same
// repo as UserImageTag with a "-pure" suffix.
//
// Example: UserImageTag()="devcell-user:ultimate" → UserImageTagPure()="devcell-user:ultimate-pure".
func UserImageTagPure() string {
	if tag := os.Getenv("DEVCELL_USER_IMAGE_PURE"); tag != "" {
		return tag
	}
	return UserImageTag() + "-pure"
}

// ResolveBuildTag returns the tag a `cell build` invocation should use:
// custom (typically from --image) when non-empty, falling back to the
// auto-derived stack tag. Trims whitespace on custom to forgive copy-paste.
func ResolveBuildTag(custom, derived string) string {
	if t := strings.TrimSpace(custom); t != "" {
		return t
	}
	return derived
}

// UserImageTagThin returns the local tag for thin images — nix store lives
// on a Docker named volume, not baked into the image.
//
// Resolution order:
//  1. DEVCELL_USER_IMAGE_THIN — legacy explicit override
//  2. DEVCELL_USER_IMAGE — modern unified override; used as-is (no suffix).
//     CELL-286 prep: every devcell image we publish is thin, so the
//     `-thin` suffix on the env-driven path is redundant. CI sets
//     `DEVCELL_USER_IMAGE=ghcr.io/devcell-sh/devcell:v<ver>-<arch>` and
//     expects this exact tag at runtime (no suffix appended).
//  3. UserImageTag() + "-thin" — local-dev fallback, preserves the suffix
//     convention so `devcell-user:<stack>-thin` doesn't collide with
//     `-pure`/`-impure` legacy local tags.
//
// Example (no env): UserImageTag()="devcell-user:ultimate" → "devcell-user:ultimate-thin".
func UserImageTagThin() string {
	if tag := os.Getenv("DEVCELL_USER_IMAGE_THIN"); tag != "" {
		return tag
	}
	if tag := os.Getenv("DEVCELL_USER_IMAGE"); tag != "" {
		return tag
	}
	return UserImageTag() + "-thin"
}

// PickImageTagThin returns the thin image tag for runtime callers.
func PickImageTagThin() string {
	return UserImageTagThin()
}

// PickImageTag is the single seam every runtime caller (`cell claude`,
// `cell shell`, `cell codex`, `cell gemini`, …) uses to decide which local
// image variant to exec into.
//
// After the 2026-05-15 flip (CELL-183) + CELL-165 vocab rename, pure is the
// default and `impure` (was `debian`) is the opt-in legacy path:
//
//	impure=false (default):    UserImageTagPure() (nix2container, devcell-user:<stack>-pure)
//	impure=true  (--impure):   UserImageTag()    (bare Dockerfile-built, devcell-user:<stack>)
func PickImageTag(impure bool) string {
	if impure {
		return UserImageTag()
	}
	return UserImageTagPure()
}

// FS abstracts filesystem stat for testability.
type FS interface {
	Stat(path string) error
}

// FSFunc is a function that implements FS.
type FSFunc func(string) error

func (f FSFunc) Stat(path string) error { return f(path) }

// OsFS is the real filesystem implementation.
var OsFS FS = FSFunc(func(path string) error {
	_, err := os.Stat(path)
	return err
})

// RunSpec holds everything needed to build the docker run argv.
type RunSpec struct {
	Config       config.Config
	CellCfg      cfg.CellConfig
	Binary       string
	DefaultFlags []string
	UserArgs     []string
	Debug        bool                // pass DEVCELL_DEBUG=true into the container
	NixDaemon    bool                // pass DEVCELL_NIX_DAEMON=true into the container
	NoPorts      bool                // skip all -p port mappings (user ports and GUI)
	TrustFlake   bool                // pass DEVCELL_FLAKE_TRUST=1 into the container
	Image        string              // image ID or tag to run; defaults to UserImageTag
	ExtraEnv     map[string]string   // additional env vars injected by the command handler
	InheritEnv   []string            // env var names to inherit from host (passed as -e KEY with no value)
	Getenv       func(string) string // env lookup; defaults to os.Getenv when nil
	GitConfig    func(string) string // host `git config` lookup (cell.HostGitConfig); nil skips that tier
	ThinImage    bool                // when true, mount devcell-nix-store volume for /nix
	BootDir      string              // CELL-264: host-side boot dir for fsnotify sentinels; empty disables the bind-mount
	TTY          bool                // allocate a pseudo-TTY (-it); set from isatty check on stdin
	Detach       bool                // run container in detached mode (-d); set by `cell start`
	NoSecrets    bool                // skip `op run --` prefix and all secrets injection
}

func (s RunSpec) getenv(key string) string {
	if s.Getenv != nil {
		return s.Getenv(key)
	}
	return os.Getenv(key)
}

// BuildArgv constructs the full docker run argv for the given spec.
// It is pure given injectable FS and LookPath.
func BuildArgv(spec RunSpec, fs FS, lookPath func(string) (string, error)) []string {
	c := spec.Config

	var argv []string

	// 1Password passthrough (suppressed by --no-secrets)
	if !spec.NoSecrets {
		if opPath, err := lookPath("op"); err == nil && opPath != "" {
			argv = append(argv, "op", "run", "--")
		}
	}

	dockerRunFlags := []string{"--rm", "--shm-size=" + spec.CellCfg.Docker.ResolvedShmSize(), "--device=/dev/fuse"}
	if spec.Detach {
		dockerRunFlags = append(dockerRunFlags, "-d")
	} else if spec.TTY {
		dockerRunFlags = append(dockerRunFlags, "-it")
	}
	if mem := spec.CellCfg.Docker.ResolvedMemLimit(); mem != "0" {
		dockerRunFlags = append(dockerRunFlags, "--memory="+mem)
	}
	if cpu := spec.CellCfg.Docker.ResolvedCPULimit(); cpu != "0" {
		dockerRunFlags = append(dockerRunFlags, "--cpus="+cpu)
	}
	// KVM passthrough for QEMU guests (Windows cells). Must be --device, not
	// a -v bind-mount: the mount creates the node but the cgroup device
	// controller still denies open(2) (EPERM). Requires nested virtualization
	// on the daemon host — on Colima that is `vmType: vz` +
	// `nestedVirtualization: true`; without it docker run fails loudly.
	if spec.CellCfg.Cell.ResolvedKVM() {
		dockerRunFlags = append(dockerRunFlags, "--device=/dev/kvm")
	}
	for _, cap := range spec.CellCfg.Docker.CapAdd {
		dockerRunFlags = append(dockerRunFlags, "--cap-add="+cap)
	}
	wgEnabled := cfg.WireguardEnabled(spec.CellCfg)
	if wgEnabled && !spec.CellCfg.Docker.Privileged {
		hasNetAdmin := false
		for _, cap := range spec.CellCfg.Docker.CapAdd {
			if cap == "NET_ADMIN" {
				hasNetAdmin = true
				break
			}
		}
		if !hasNetAdmin {
			dockerRunFlags = append(dockerRunFlags, "--cap-add=NET_ADMIN")
		}
	}
	if wgEnabled {
		dockerRunFlags = append(dockerRunFlags, "--device=/dev/net/tun")
		dockerRunFlags = append(dockerRunFlags, "--sysctl", "net.ipv4.conf.all.src_valid_mark=1")
	}
	if spec.CellCfg.Docker.Privileged {
		dockerRunFlags = append(dockerRunFlags, "--privileged")
	}
	argv = append(argv, "docker", "run")
	argv = append(argv, dockerRunFlags...)

	// Identity
	argv = append(argv, "--name", c.ContainerName)
	argv = append(argv, "--hostname", spec.CellCfg.Cell.ResolvedHostname(c.Hostname))
	argv = append(argv, "--user", "0")
	argv = append(argv, "--group-add", "0")

	// Labels for VNC lookup: filter by basedir+cellid without inspecting all containers
	argv = append(argv, "--label", "devcell.basedir="+c.BaseDir)
	argv = append(argv, "--label", "devcell.cellid="+c.Bunk)

	// Container-layout env vars. The cell-level ones (APP_NAME,
	// DEVCELL_CELL_NAME, IS_SANDBOX, WORKSPACE, git identity, TZ, locale,
	// [env], [mise]) come from cell.GuestEnv further down.
	e := func(k, v string) { argv = append(argv, "-e", k+"="+v) }
	e("HOST_USER", c.HostUser)
	e("HOME", "/home/"+c.HostUser)
	e("TERM", os.Getenv("TERM"))
	e("HISTFILE", "/home/"+c.HostUser+"/zsh_history_"+c.AppName)
	e("TMPDIR", "/home/"+c.HostUser+"/tmp")
	e("CODEX_OSS_BASE_URL", envOrDefault("CODEX_OSS_BASE_URL", "http://host.docker.internal:1234/v1"))
	e("DEVCELL_HOST_PROJECT_DIR", c.BaseDir)

	v := func(mount string) { argv = append(argv, "-v", mount) }

	// Optional .env file — resolve self-referencing vars (KEY=${KEY}) by passing
	// -e KEY so Docker inherits the real value from the host environment.
	// Literal KEY=value lines are passed as-is via -e KEY=value.
	// Comments and blank lines are skipped.
	envFile := filepath.Join(c.BaseDir, ".env.devcell")
	if envData, err := os.ReadFile(envFile); err == nil {
		for _, line := range strings.Split(string(envData), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && parts[1] == "${"+parts[0]+"}" {
				// Self-referencing: KEY=${KEY} → inherit from host env
				argv = append(argv, "-e", parts[0])
			} else {
				argv = append(argv, "-e", line)
			}
		}
	}

	// GUI flag — only publish VNC port when GUI is enabled (default: true)
	if spec.CellCfg.GUI.ResolvedEnabled() {
		argv = append(argv, "-e", "DEVCELL_GUI_ENABLED=true")
		argv = append(argv, "-e", "DEVCELL_WM="+spec.CellCfg.GUI.ResolvedWM())
		argv = append(argv, "-e", "DEVCELL_RESOLUTION="+spec.CellCfg.GUI.ResolvedFramebufferResolution())
		argv = append(argv, "-e", fmt.Sprintf("DEVCELL_DPI=%d", spec.CellCfg.GUI.ResolvedDPI()))
		argv = append(argv, "-e", fmt.Sprintf("DEVCELL_SCALE=%d", spec.CellCfg.GUI.ResolvedScale()))
		argv = append(argv, "-e", "EXT_VNC_PORT="+c.VNCPort)
		argv = append(argv, "-e", "EXT_RDP_PORT="+c.RDPPort)
	}

	// Debug flag — enables verbose entrypoint logging inside the container
	if spec.Debug {
		argv = append(argv, "-e", "DEVCELL_DEBUG=true")
	}

	// Nix daemon — enables in-container package installation via nix-daemon
	if spec.NixDaemon {
		argv = append(argv, "-e", "DEVCELL_NIX_DAEMON=true")
	}

	// Project flake trust — user confirmed host-side that flake.nix packages should be installed
	if spec.TrustFlake {
		argv = append(argv, "-e", "DEVCELL_FLAKE_TRUST=1")
	}

	// Pass the image tag/ID into the container for debug logging
	if spec.Debug && spec.Image != "" {
		argv = append(argv, "-e", "DEVCELL_IMAGE="+spec.Image)
	}

	// Build provenance from OCI manifest → container env. The entrypoint's
	// "User image:" log line surfaces these so users can confirm at boot
	// which build/commit they're running, without spawning a `docker inspect`
	// from inside the container. Reads labels via the single `docker image
	// inspect` call below — cheap (~ms, no container spawn).
	if meta := ImageMetadataFromContainer(context.Background()); meta.BuildDate != "" {
		argv = append(argv, "-e", "DEVCELL_BUILD_DATE="+meta.BuildDate)
		if meta.GitCommit != "" && meta.GitCommit != "unknown" {
			argv = append(argv, "-e", "DEVCELL_BUILD_REV="+meta.GitCommit)
		}
	}

	// AWS read-only credential scoping — nix-managed config with credential_process
	if spec.CellCfg.Aws.ResolvedReadOnly() {
		e("AWS_CONFIG_FILE", "/opt/devcell/.aws/config")
		e("AWS_READ_OPERATIONS_ONLY", "true")
		e("READ_OPERATIONS_ONLY", "true") // consumed by aws-api MCP server
	}

	// Stealth identity — drives CDP userAgentMetadata + JS spoofs inside container
	e("DEVCELL_STEALTH_ARCH", spec.CellCfg.Stealth.ResolvedArch())
	e("DEVCELL_STEALTH_PLATFORM", spec.CellCfg.Stealth.ResolvedPlatform())

	// [mcp] enabled — runtime MCP server selection (fragments filter at start)
	if len(spec.CellCfg.Mcp.Enabled) > 0 {
		e("DEVCELL_MCP_ENABLED", strings.Join(spec.CellCfg.Mcp.Enabled, ","))
	}

	// [packages.python] / [packages.node] — installed by mise at container start
	if tools := spec.CellCfg.Packages.MiseTools(); tools != "" {
		e("DEVCELL_MISE_PACKAGES", tools)
	}

	// Cell-level guest env, shared with the tart and winkit engines.
	// Emitted after .env.devcell and the DEVCELL_* flags above so [env] can
	// override any of them; ExtraEnv and InheritEnv below still win over it.
	for _, kv := range cell.GuestEnv(cell.EnvInput{
		CellName:  c.CellName,
		AppName:   c.AppName,
		Workspace: "/" + c.AppName,
		Git:       spec.CellCfg.Git,
		Timezone:  spec.CellCfg.Cell.Timezone,
		Locale:    spec.CellCfg.Cell.Locale,
		Env:       spec.CellCfg.Env,
		Mise:      spec.CellCfg.Mise,
		Getenv:    spec.getenv,
		GitConfig: spec.GitConfig,
	}) {
		argv = append(argv, "-e", kv)
	}

	// Command-specific extra env vars (e.g. OPENCODE_CONFIG_CONTENT)
	for k, v := range spec.ExtraEnv {
		argv = append(argv, "-e", k+"="+v)
	}

	// Inherit env vars from host (secrets resolved by caller, set via os.Setenv)
	for _, k := range spec.InheritEnv {
		argv = append(argv, "-e", k)
	}

	// Tell the entrypoint which env vars are op-resolved secrets (for Playwright MCP)
	if len(spec.InheritEnv) > 0 {
		argv = append(argv, "-e", "DEVCELL_SECRET_KEYS="+strings.Join(spec.InheritEnv, ","))
	}

	// Standard volumes
	v(c.BaseDir + ":" + c.BaseDir)
	v(c.BaseDir + ":/" + c.AppName)
	v(c.CellHome + ":/home/" + c.HostUser)
	v("/var/run/docker.sock:/var/run/docker.sock")
	v(c.HostHome + "/.claude/commands:/home/" + c.HostUser + "/.claude/commands")
	v(c.HostHome + "/.claude/agents:/home/" + c.HostUser + "/.claude/agents:ro")
	v(c.HostHome + "/.claude/skills:/home/" + c.HostUser + "/.claude/skills")
	v(c.HostHome + "/.agents:/home/" + c.HostUser + "/.agents:ro")
	v(c.HostHome + "/.claude/agents:/home/" + c.HostUser + "/.config/opencode/agents:ro")
	v(c.ConfigDir + ":/etc/devcell/config")
	v(c.ConfigDir + ":/home/" + c.HostUser + "/.config/devcell")

	// Thin image: nix store lives on a named Docker volume, not baked into the image
	if spec.ThinImage {
		v(ThinStoreVolume() + ":/nix")
	}

	// Boot dir bind-mount: hosts the Go-generated init script AND
	// in-container progress sentinel files (CELL-264).
	if spec.BootDir != "" {
		v(spec.BootDir + ":" + BootContainerPath)
		e("DEVCELL_BOOT_DIR", BootContainerPath)
	}

	// cfg [[volumes]] entries — skip any whose container path duplicates
	// a standard mount (e.g. BaseDir identity mount) to avoid Docker's
	// "Duplicate mount point" error.
	stdMounts := map[string]bool{
		c.BaseDir:              true,
		"/" + c.AppName:        true,
		"/home/" + c.HostUser:  true,
		"/var/run/docker.sock": true,
		"/home/" + c.HostUser + "/.claude/commands":        true,
		"/home/" + c.HostUser + "/.claude/agents":          true,
		"/home/" + c.HostUser + "/.claude/skills":          true,
		"/home/" + c.HostUser + "/.agents":                 true,
		"/home/" + c.HostUser + "/.config/opencode/agents": true,
		"/etc/devcell/config":                              true,
		"/home/" + c.HostUser + "/.config/devcell":         true,
	}
	for _, vol := range spec.CellCfg.Volumes {
		cp := vol.ContainerPath()
		if stdMounts[cp] {
			continue
		}
		argv = append(argv, "-v", vol.Resolved())
	}

	// --no-ports: skip all -p mappings (user ports and GUI).
	if !spec.NoPorts {
		// [ports].publish_ip — host interface prefix for `docker run -p`.
		// Defaults to "0.0.0.0" (set by ResolvedPublishIP) so cells are reachable
		// from other hosts on the LAN regardless of dockerd bind defaults.
		publishPrefix := spec.CellCfg.Ports.ResolvedPublishIP() + ":"

		// cfg [ports] entries — resolve "8080:" auto-port syntax
		taken := config.DockerAllocatedPorts()
		for _, port := range spec.CellCfg.Ports.Forward {
			resolved := config.ResolveForwardEntry(port, taken)
			argv = append(argv, "-p", publishPrefix+resolved)
		}

		// GUI port mapping
		if spec.CellCfg.GUI.ResolvedEnabled() {
			argv = append(argv, "-p", publishPrefix+c.VNCPort+":5900")
			argv = append(argv, "-p", publishPrefix+c.RDPPort+":3389")
		}
	}

	// Wireguard env + config mount. The tunnel secrets are forwarded by
	// name only (-e KEY) so their values never appear in the docker argv;
	// the entrypoint fragment writes them to the /run/secrets tmpfs.
	if wgEnabled {
		argv = append(argv, "-e", "DEVCELL_WG_ENABLED=1")
		for _, k := range []string{"WG_PRIVATE_KEY", "WG_PRESHARED_KEY"} {
			if spec.getenv(k) != "" {
				argv = append(argv, "-e", k)
			}
		}
		wgDir := filepath.Join(c.CellHome, ".wg")
		argv = append(argv, "-v", wgDir+":/home/"+c.HostUser+"/.devcell/"+c.CellName+"/.wg:ro")
	}

	// In-memory secrets mount — Playwright MCP reads .secrets-playwright from here
	argv = append(argv, "--tmpfs", "/run/secrets:mode=700,noexec,nosuid,size=1m")

	// Network
	argv = append(argv, "--network", "devcell-network")

	// MAC address — pinned across restarts for infra-side identity persistence
	// (bot-protection rate-limit keys, persistent-ban identification, stable
	// WebRTC-leaked IP via stable network slot). Honored on user-defined bridge
	// networks like devcell-network. Empty → docker auto-assigns per launch.
	if mac := spec.CellCfg.Cell.MacAddress; mac != "" {
		argv = append(argv, "--mac-address", mac)
	}

	// Workdir
	argv = append(argv, "--workdir", "/"+c.AppName)

	// Go-generated entrypoint: s6 activation + gosu drop to user + exec CMD.
	// The init script is written to bootDir by Run() before docker run.
	if spec.BootDir != "" {
		argv = append(argv, "--entrypoint", BootContainerPath+"/entrypoint.sh")
	}

	// Image — use pinned digest when available, fall back to mutable tag
	image := spec.Image
	if image == "" {
		image = UserImageTag()
	}
	argv = append(argv, image)

	// Binary and flags as CMD — the entrypoint execs into them via gosu.
	if spec.Binary != "" {
		argv = append(argv, spec.Binary)
		argv = append(argv, spec.DefaultFlags...)
		argv = append(argv, spec.UserArgs...)
	}

	return argv
}

// RemoveOrphanedContainer removes a stopped container with the given name if it exists.
// Returns nil if the container doesn't exist or was successfully removed.
// Returns an error if the container is currently running.
func RemoveOrphanedContainer(ctx context.Context, name string) error {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Status}}", name).Output()
	if err != nil {
		// Container doesn't exist — nothing to do.
		return nil
	}
	status := strings.TrimSpace(string(out))
	if status == "running" {
		return fmt.Errorf("container %q is already running — stop it first with: docker stop %s", name, name)
	}
	if err := exec.CommandContext(ctx, "docker", "rm", name).Run(); err != nil {
		return fmt.Errorf("remove orphaned container %q: %w", name, err)
	}
	return nil
}

// ContainerRunning checks if a container with the given name is currently running.
func ContainerRunning(ctx context.Context, name string) bool {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Status}}", name).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "running"
}

// EnsureNetwork creates the devcell-network docker network if it doesn't exist.
func EnsureNetwork(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "create", "devcell-network")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	// Ignore error — network likely already exists.
	_ = cmd.Run()
	return nil
}

// DetectArch returns "aarch64" or "x86_64". Respects DEVCELL_ARCH env
// override ("amd64"→"x86_64", "arm64"→"aarch64") for cross-architecture builds.
func DetectArch() string {
	if v := os.Getenv("DEVCELL_ARCH"); v != "" {
		switch v {
		case "amd64", "x86_64":
			return "x86_64"
		case "arm64", "aarch64":
			return "aarch64"
		}
	}
	if runtime.GOARCH == "arm64" {
		return "aarch64"
	}
	return "x86_64"
}

// DockerPlatform returns the docker --platform value for the given nix arch.
func DockerPlatform(arch string) string {
	if arch == "aarch64" {
		return "linux/arm64"
	}
	return "linux/amd64"
}

// ImageExists returns true if a Docker image with the given tag exists locally.
func ImageExists(ctx context.Context, tag string) bool {
	return exec.CommandContext(ctx, "docker", "image", "inspect", tag).Run() == nil
}

// ImageExistsForPlatform returns true if a Docker image with the given tag
// exists locally AND matches the requested platform (e.g. "linux/amd64").
// Empty platform falls back to ImageExists (host default).
func ImageExistsForPlatform(ctx context.Context, tag, platform string) bool {
	if platform == "" {
		return ImageExists(ctx, tag)
	}
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect",
		"--format", "{{.Os}}/{{.Architecture}}", tag).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == platform
}

// PullImageForPlatform pulls a Docker image for a specific platform.
// Empty platform uses Docker's default (host architecture). When verbose is
// true, docker pull output is streamed to os.Stderr.
func PullImageForPlatform(ctx context.Context, tag, platform string, verbose bool) error {
	args := []string{"pull"}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	args = append(args, tag)
	cmd := exec.CommandContext(ctx, "docker", args...)
	if verbose {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	return cmd.Run()
}

// LocalImageIDFor returns the full image ID (sha256:...) for tag. Used to pin
// the running container to the exact image just built, rather than the
// mutable tag which could race with a concurrent build.
func LocalImageIDFor(ctx context.Context, tag string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect",
		tag, "--format", "{{.Id}}").Output()
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", tag, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ImageLabel returns one label of a local image, or "" when the image or
// the label is missing.
func ImageLabel(ctx context.Context, image, key string) string {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", image,
		"--format", "{{index .Config.Labels \""+key+"\"}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// LocalImageSize returns the on-daemon size in bytes of the local image with
// the given tag. Used post-build to show "Loaded: 1.4 GB" alongside the
// spinner success — skopeo's per-blob progress is empty when stdout isn't a
// TTY, so we synthesize a summary from `docker image inspect` instead.
//
// Returns 0 on any error; callers treat 0 as "size unknown, skip the summary".
func LocalImageSize(ctx context.Context, tag string) int64 {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect",
		tag, "--format", "{{.Size}}").Output()
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ImageMetadata holds structured build metadata from /etc/devcell/metadata.json.
type ImageMetadata struct {
	BaseImage string   `json:"base_image"`
	Stack     string   `json:"stack"`
	Modules   []string `json:"modules"`
	GitCommit string   `json:"git_commit"`
	BuildDate string   `json:"build_date"`
	Packages  int      `json:"packages"`
}

// ParseImageMetadata parses JSON into ImageMetadata. Returns zero value on error.
func ParseImageMetadata(data []byte) ImageMetadata {
	var m ImageMetadata
	json.Unmarshal(data, &m)
	return m
}

// ImageMetadataFromContainer reads build metadata for the current launch's image.
//
// Source-of-truth flip (2026-05-16): the date/rev now come from the OCI
// image manifest (labels + Created field) rather than /etc/devcell/metadata.json
// inside the image. Why: when metadata.json carried a real per-build timestamp,
// every `cell build` invocation perturbed homeRoot's tar hash and forced
// skopeo to re-push the ~3.9GB customization layer even when no source had
// changed. Pinning metadata.json to static placeholders eliminates that;
// the date moves to OCI manifest labels (which only affect the tiny manifest
// blob, not layer content).
//
// Reads:
//   - .Created          — OCI manifest creation timestamp (= our buildDate)
//   - .Config.Labels    — devcell.stack, org.opencontainers.image.revision, etc.
//   - .Config.Env       — DEVCELL_STACK (falls back to DEVCELL_PROFILE for older images)
//
// Falls back to a runtime `cat /etc/devcell/metadata.json` for older images
// that predate the manifest-based stamping.
func ImageMetadataFromContainer(ctx context.Context) ImageMetadata {
	tag := PickImageTag(false) // false = pure (default after CELL-183)
	if !ImageExists(ctx, tag) {
		// Pure image not built/pulled yet; fall back to the debian tag if
		// that's all we have locally.
		tag = UserImageTag()
	}

	// Fast path — single `docker inspect`, no container spawn. JSON output
	// covers everything we need from labels + the OCI manifest's Created.
	type inspectConfig struct {
		Labels map[string]string `json:"Labels"`
		Env    []string          `json:"Env"`
	}
	type inspectImage struct {
		Created string        `json:"Created"`
		Config  inspectConfig `json:"Config"`
	}

	out, err := exec.CommandContext(ctx,
		"docker", "image", "inspect", tag, "--format", "{{json .}}",
	).Output()
	if err == nil && len(out) > 0 {
		var arr []inspectImage
		// `docker image inspect` returns a JSON array; the --format we
		// pass actually returns a single object per image, so try both.
		if jsonErr := json.Unmarshal(out, &arr); jsonErr == nil && len(arr) > 0 {
			return imageMetadataFromInspect(arr[0].Created, arr[0].Config.Labels, arr[0].Config.Env)
		}
		var single inspectImage
		if jsonErr := json.Unmarshal(out, &single); jsonErr == nil {
			return imageMetadataFromInspect(single.Created, single.Config.Labels, single.Config.Env)
		}
	}

	// Legacy fallback for older images without OCI labels — runtime read.
	legacyOut, legacyErr := exec.CommandContext(ctx, "docker", "run", "--rm", "--entrypoint", "sh",
		tag, "-c",
		"cat /etc/devcell/metadata.json 2>/dev/null",
	).Output()
	if legacyErr != nil || len(legacyOut) == 0 {
		return ImageMetadata{}
	}
	return ParseImageMetadata(legacyOut)
}

// imageMetadataFromInspect synthesises ImageMetadata from `docker image inspect`
// output. Pure helper, no I/O.
func imageMetadataFromInspect(created string, labels map[string]string, env []string) ImageMetadata {
	m := ImageMetadata{
		BaseImage: labels["devcell.built-with"], // "nix2container" / ""
		Stack:     labels["devcell.stack"],
		GitCommit: labels["org.opencontainers.image.revision"],
		BuildDate: labels["org.opencontainers.image.created"],
	}
	// BuildDate fallback: if no label, use the OCI Created field — both
	// surface the same thing in our build but the Created field is set
	// at all pure builds whereas the label is a 2026-05-16+ addition.
	if m.BuildDate == "" {
		m.BuildDate = created
	}
	// Stack fallback: image config Env carries DEVCELL_STACK=<stack> (plain
	// name, no prefix). Older images only carry DEVCELL_PROFILE=devcell-<stack>;
	// prefer DEVCELL_STACK when both are present.
	if m.Stack == "" {
		for _, kv := range env {
			if strings.HasPrefix(kv, "DEVCELL_STACK=") {
				m.Stack = strings.TrimPrefix(kv, "DEVCELL_STACK=")
				break
			}
		}
	}
	if m.Stack == "" {
		for _, kv := range env {
			if strings.HasPrefix(kv, "DEVCELL_PROFILE=devcell-") {
				m.Stack = strings.TrimPrefix(kv, "DEVCELL_PROFILE=devcell-")
				break
			}
		}
	}
	if m.BaseImage == "" {
		m.BaseImage = "nix2container"
	}
	return m
}

// formatImageVersionUser is the pure formatting half of ImageVersions —
// extracted so tests can exercise it without mocking docker. Returns the
// string for the "User image: <X>" log line, "" when only placeholders
// are present (caller then falls through to legacy file read).
func formatImageVersionUser(m ImageMetadata) string {
	hasCommit := m.GitCommit != "" && m.GitCommit != "unknown" && m.GitCommit != "nix2container"
	hasDate := m.BuildDate != "" && m.BuildDate != "1970-01-01T00:00:00Z" && m.BuildDate != "0001-01-01T00:00:00Z"
	switch {
	case hasCommit && hasDate:
		return m.GitCommit + " built " + m.BuildDate
	case hasDate:
		return "built " + m.BuildDate
	case hasCommit:
		return m.GitCommit
	}
	return ""
}

// ImageVersions reads build metadata from the user image.
// Returns (base, user) strings for backward compatibility with callers.
// Format: user = "<commit> built <date>" when both known,
//
//	"built <date>" when only date,
//	"<commit>" when only commit,
//	"" otherwise (falls through to legacy file read).
func ImageVersions(ctx context.Context) (base, user string) {
	m := ImageMetadataFromContainer(ctx)
	if u := formatImageVersionUser(m); u != "" {
		return m.BaseImage, u
	}
	// Fallback: legacy files (pre-metadata.json images). Uses same tag
	// preference as ImageMetadataFromContainer above.
	tag := PickImageTag(false)
	if !ImageExists(ctx, tag) {
		tag = UserImageTag()
	}
	out, err := exec.CommandContext(ctx, "docker", "run", "--rm", "--entrypoint", "sh",
		tag, "-c",
		"cat /etc/devcell/base-image-version 2>/dev/null; echo '---'; cat /etc/devcell/user-image-version 2>/dev/null",
	).Output()
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "---", 2)
	if len(parts) == 2 {
		base = strings.TrimSpace(parts[0])
		user = strings.TrimSpace(parts[1])
	}
	return
}

// UpdateFlakeLock runs nix flake lock (or update) inside a lightweight
// nixos/nix container with configDir bind-mounted.
func UpdateFlakeLock(ctx context.Context, configDir string, lockOnly bool, verbose bool, out io.Writer) error {
	nixCmd := "nix flake update"
	if lockOnly {
		nixCmd = "nix flake lock"
	}
	args := []string{
		"run", "--rm",
		"-v", configDir + ":/work",
		"-e", "NIX_CONFIG=experimental-features = nix-command flakes",
		"--entrypoint", "sh",
		FlakeNixImage(),
		"-c", "cd /work && " + nixCmd,
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	if verbose {
		cmd.Stdout = out
		cmd.Stderr = out
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = out
	}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("nix flake: interrupted")
		}
		return fmt.Errorf("nix flake: %w", err)
	}
	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// PrepareWireguard writes WireGuard config files for each enabled entry
// to <cellHome>/.wg/<name>.conf. PrivateKey lines are stripped from the
// config; a PostUp directive loads the key from /run/secrets/wg-private-key
// at runtime. No-op when no entries are enabled.
func PrepareWireguard(cellHome string, cellCfg cfg.CellConfig) error {
	if !cfg.WireguardEnabled(cellCfg) {
		return nil
	}
	wgDir := filepath.Join(cellHome, ".wg")
	if err := os.MkdirAll(wgDir, 0700); err != nil {
		return fmt.Errorf("create wireguard dir: %w", err)
	}
	for _, entry := range cellCfg.Wireguard {
		if !entry.Enabled {
			continue
		}
		conf := rewriteWireguardConfig(entry.Config)
		path := filepath.Join(wgDir, entry.Name+".conf")
		if err := os.WriteFile(path, []byte(conf), 0600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

const (
	wgPrivateKeyPostUp       = "PostUp = wg set %i private-key /run/secrets/wg-private-key\n"
	wgPresharedKeyPostUpPre  = "PostUp = [ -f /run/secrets/wg-preshared-key ] && wg set %i peer "
	wgPresharedKeyPostUpPost = " preshared-key /run/secrets/wg-preshared-key || true\n"
)

// rewriteWireguardConfig strips secrets from the config and replaces them
// with PostUp directives that load them from /run/secrets at runtime:
//
//   - PrivateKey is removed from [Interface]; a PostUp loads
//     /run/secrets/wg-private-key (WG_PRIVATE_KEY, required).
//   - PresharedKey is removed from every [Peer]; one guarded PostUp per peer
//     applies /run/secrets/wg-preshared-key (WG_PRESHARED_KEY, optional) to
//     that peer's PublicKey. The guard keeps the config valid when no PSK is
//     supplied. A single env var means every peer gets the same PSK.
//
// wg-quick runs PostUp after `wg setconf`, so the peers exist when the
// preshared key is applied.
func rewriteWireguardConfig(raw string) string {
	lines := strings.Split(raw, "\n")

	// First pass: collect peer public keys so the [Interface] PostUp lines
	// can reference them before the [Peer] sections appear.
	var peerKeys []string
	inPeer := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if section, ok := wgSectionHeader(trimmed); ok {
			inPeer = section == "Peer"
			continue
		}
		if inPeer {
			if key, val, ok := wgKeyValue(trimmed); ok && key == "PublicKey" && val != "" {
				peerKeys = append(peerKeys, val)
			}
		}
	}

	writePostUps := func(out *strings.Builder) {
		out.WriteString(wgPrivateKeyPostUp)
		for _, pub := range peerKeys {
			out.WriteString(wgPresharedKeyPostUpPre + pub + wgPresharedKeyPostUpPost)
		}
	}

	var out strings.Builder
	inInterface := false
	inPeer = false
	postUpAdded := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if section, ok := wgSectionHeader(trimmed); ok {
			if inInterface && !postUpAdded {
				writePostUps(&out)
				postUpAdded = true
			}
			inInterface = section == "Interface"
			inPeer = section == "Peer"
		}
		key, _, isKV := wgKeyValue(trimmed)
		if inInterface && isKV && key == "PrivateKey" {
			continue
		}
		if inPeer && isKV && key == "PresharedKey" {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if inInterface && !postUpAdded {
		writePostUps(&out)
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

// wgSectionHeader reports whether a trimmed line is an INI section header
// such as "[Interface]" and returns the section name.
func wgSectionHeader(trimmed string) (string, bool) {
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return strings.TrimSpace(trimmed[1 : len(trimmed)-1]), true
	}
	return "", false
}

// wgKeyValue splits a trimmed "Key = value" line. Comments and blank lines
// are not key/value pairs.
func wgKeyValue(trimmed string) (key, value string, ok bool) {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	k, v, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", false
	}
	return strings.TrimSpace(k), strings.TrimSpace(v), true
}

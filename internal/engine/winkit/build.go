//go:build darwin || linux

package winkit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	gowinkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/build"
	"github.com/devcell-sh/go-winkit/build/buildopts"
	"github.com/devcell-sh/go-winkit/build/imageformat"
	"github.com/devcell-sh/go-winkit/vm/qemu"
	"github.com/devcell-sh/go-winkit/wsl"

	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/ux"
)

// winkitBuildFunc is the function used to build guest images. Defaults to
// gowinkit.Build; tests replace it to capture the config without running
// a real build.
var winkitBuildFunc = gowinkit.Build

// legacyPEArtifactName is what PE builds against winkit before the
// full/full-wsl/pe/pe-wsl stage names wrote.
const legacyPEArtifactName = "winkit-core.qcow2"

// buildOpts selects winkit's build stage for g, both with a nix WSL1
// distro: pe-wsl for WindowsPE, full-wsl for WindowsFull.
func buildOpts(g guest, nixHome string) *buildopts.BuildOpts {
	opts := &buildopts.BuildOpts{
		PE: !g.full,
		WSL: &buildopts.WSLConfig{
			Image:   "nix",
			NixHome: nixHome,
		},
	}
	if png, err := RenderWallpaper(nixHome, ""); err == nil && len(png) > 0 {
		opts.WallpaperName = wallpaperPNGName
		opts.WallpaperData = png
		ux.Debugf("wallpaper: rendered static PNG (%d bytes)", len(png))
	}
	return opts
}

// artifactPath is where a build of g writes its image, named the way
// `winkit build` names its default output for the same stage
// (winkit-pe-wsl.qcow2, winkit-full-wsl.qcow2). A PE build's data disk and
// manifest sit next to it.
func artifactPath(home string, g guest, stack string, modules []string) string {
	stage := string(buildOpts(g, "").Stage())
	return filepath.Join(templateDir(home, g, stack, modules), imageformat.DefaultOutputName(stage, imageformat.Qcow2))
}

// buildConfig assembles a go-winkit build.Config for guest g of cell c.
// windowsISO and virtioISO are resolved paths passed in by the caller.
// pr is the PhaseRunner for UX spinner updates (may be nil).
func buildConfig(c engine.Cell, g guest, windowsISO, virtioISO string, pr *ux.PhaseRunner) build.Config {
	nixHome := c.Config.Nix.NixhomePath
	if nixHome == "" {
		nixHome = os.Getenv("DEVCELL_NIXHOME")
	}
	opts := buildOpts(g, nixHome)
	if c.Name != "" {
		opts.Hostname = c.Name
	}
	opts.Ports = allocateBuildPorts()
	workDir := buildWorkDir(c.BaseDir)
	bc := build.Config{
		Dest:              artifactPath(c.HostHome, g, c.Stack, c.Modules),
		CacheDir:          CacheDir(c.HostHome),
		WorkDir:           workDir,
		WindowsISO:        windowsISO,
		VirtIOISO:         virtioISO,
		Opts:              opts,
		Logger:            buildLogger(pr),
		StructuredLogPath: filepath.Join(workDir, "build.jsonl"),
		Accel:             os.Getenv("DEVCELL_WINKIT_ACCEL"),
		DisplayType:       vncDisplayType(),
	}
	if nixHome != "" {
		bc.NixHomeOpts = wslNixHomeOpts(c.Stack)
	}
	return bc
}

func vncDisplayType() string {
	v := os.Getenv("DEVCELL_WINKIT_VNC")
	if v == "" {
		return ""
	}
	return "vnc=:" + v
}

// buildWorkDir returns a timestamped directory under
// <projectDir>/.devcell/debug/ for build intermediates (screenshots,
// serial logs, guest event stream). Falls back to os.TempDir when
// projectDir is empty.
func buildWorkDir(projectDir string) string {
	ts := time.Now().UTC().Format("20060102T150405Z")
	if projectDir == "" {
		return filepath.Join(os.TempDir(), "winkit-build-"+ts)
	}
	return filepath.Join(projectDir, ".devcell", "debug", ts+"-build")
}

// wslNixHomeOpts returns NixHomeOpts for the community-home WSL flake
// attribute matching the given stack. The community-home flake exposes WSL
// configs as "wsl-<shortStack>[-aarch64]" with user "nixos".
func wslNixHomeOpts(stack string) *wsl.NixHomeOpts {
	short := strings.TrimPrefix(stack, "devcell-")
	attr := "wsl-" + short
	if runtime.GOARCH == "arm64" {
		attr += "-aarch64"
	}
	return &wsl.NixHomeOpts{
		Attr: attr,
		User: "nixos",
	}
}

// allocateBuildPorts picks free host ports for the build VM's gosshd,
// OpenSSH, and RDP forwards. Binding to :0 lets the kernel assign a
// free port; closing before QEMU starts leaves a brief race window,
// but it is the standard Go idiom and far better than hardcoded ports
// that collide with running cells.
func allocateBuildPorts() buildopts.Ports {
	return buildopts.Ports{
		Gossh:   freePort(),
		OpenSSH: freePort(),
		RDP:     freePort(),
	}
}

// freePort asks the kernel for a free TCP port on loopback.
var freePort = defaultFreePort

func defaultFreePort() uint16 {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return uint16(port)
}

// buildLogger returns an slog.Logger that routes to ux.Debugf for debug
// output and updates the PhaseRunner spinner with major milestones even
// in non-debug mode. pr may be nil (no spinner updates).
func buildLogger(pr *ux.PhaseRunner) *slog.Logger {
	return slog.New(&uxDebugHandler{runner: pr})
}

// uxDebugHandler is an slog.Handler that forwards records to ux.Debugf.
// Guest-event lines (JSONL from the pe-agent) are parsed for a cleaner
// display: "winkit: [event] msg" instead of raw JSON. When runner is set,
// major milestones also update the PhaseRunner spinner for non-debug UX.
type uxDebugHandler struct {
	runner *ux.PhaseRunner
}

func (h *uxDebugHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return ux.Verbose || h.runner != nil
}
func (h *uxDebugHandler) Handle(_ context.Context, r slog.Record) error {
	var source string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "source" {
			source = a.Value.String()
		}
		return true
	})
	if source == "guest-event" {
		if msg := formatGuestEvent(r.Message); msg != "" {
			ux.Debugf("winkit: %s", msg)
			h.updateSpinner(msg)
			return nil
		}
	}
	var b strings.Builder
	b.WriteString("winkit: ")
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	ux.Debugf("%s", b.String())
	h.updateSpinner(r.Message)
	return nil
}

// updateSpinner sets the PhaseRunner's spinner text to show build progress.
func (h *uxDebugHandler) updateSpinner(msg string) {
	if h.runner == nil {
		return
	}
	h.runner.UpdateText(msg)
}
func (h *uxDebugHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *uxDebugHandler) WithGroup(string) slog.Handler       { return h }

// guestEvent is the JSONL schema written by go-winkit's pe-agent.
type guestEvent struct {
	TS    string `json:"ts"`
	Event string `json:"event"`
	Msg   string `json:"msg"`
	Line  string `json:"line"`
	Src   string `json:"src"`
}

// formatGuestEvent parses a JSONL line from guest.jsonl and returns a
// human-readable string. Returns "" if the line is not valid JSON.
func formatGuestEvent(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "{") {
		return ""
	}
	var ev guestEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return ""
	}
	msg := ev.Msg
	if msg == "" {
		msg = ev.Line
	}
	if msg == "" {
		return ""
	}
	if ev.Event != "" {
		return fmt.Sprintf("[%s] %s", ev.Event, msg)
	}
	return msg
}

// imagePath returns the image to boot for guest g, stack and modules. A PE
// template built under the legacy name is used until a rebuild writes the
// stage-named artifact; that name never held a Full image.
func imagePath(home string, g guest, stack string, modules []string) string {
	current := artifactPath(home, g, stack, modules)
	if g.full {
		return current
	}
	if _, err := os.Stat(current); err == nil {
		return current
	}
	legacy := filepath.Join(filepath.Dir(current), legacyPEArtifactName)
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return current
}

// startOpts assembles gowinkit.StartOpts for booting guest g of cell c.
// winkit keeps the VM's state (PID, ports) in the cell's instance dir, where
// DiscoverRunningVMs finds it.
func startOpts(c engine.Cell, g guest, sshPort, rdpPort, vncPort uint16) gowinkit.StartOpts {
	return gowinkit.StartOpts{
		Image:    imagePath(c.HostHome, g, c.Stack, c.Modules),
		Name:     c.Name,
		StateDir: InstanceDir(c.HostHome, c.Name),
		SSHPort:  sshPort,
		RDPPort:  rdpPort,
		VNCPort:  vncPort,
		CPUs:     uint(c.Config.Cell.ResolvedWinkitCPUs()),
		MemoryGB: uint64(c.Config.Cell.ResolvedWinkitMemoryGB()),
	}
}

// guestCommand assembles the Windows command that runs the agent in guest
// g's WSL1 distro. User args are forwarded via `wsl -d winkit -- <args>`,
// which hands the rest of the line to the distro's shell, so args are
// quoted for a POSIX shell. With no args, an interactive WSL shell opens.
//
// PE runs the command under cmd.exe, which passes that line through as is.
// Full Windows runs it under PowerShell, the Windows OpenSSH default shell;
// the stop-parsing token --% makes PowerShell do the same.
func guestCommand(g guest, env, argv []string) string {
	if len(env) == 0 && len(argv) == 0 {
		return "wsl -d winkit"
	}
	var parts []string
	if len(env) > 0 {
		parts = append(parts, "env")
		parts = append(parts, env...)
	}
	parts = append(parts, argv...)
	for i, p := range parts {
		parts[i] = cell.ShellQuote(p)
	}
	wsl := "wsl -d winkit -- "
	if g.full {
		wsl = "wsl --% -d winkit -- "
	}
	return wsl + strings.Join(parts, " ")
}

// guestInvocation merges the forwarded env with the agent's extra env
// (sorted for stable output) and builds the agent argv.
func guestInvocation(env []string, extraEnv map[string]string, binary string, defaultFlags, userArgs []string) ([]string, []string) {
	keys := make([]string, 0, len(extraEnv))
	for k := range extraEnv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	merged := append([]string(nil), env...)
	for _, k := range keys {
		merged = append(merged, k+"="+extraEnv[k])
	}
	argv := append([]string{binary}, defaultFlags...)
	return merged, append(argv, userArgs...)
}

// Build builds the cell's Windows guest image with go-winkit: a WinPE boot
// volume (stage pe-wsl) or a full Windows install (full-wsl), each with a
// WSL1 nix distro. Installer media (Windows ISO, VirtIO drivers) are
// downloaded automatically when missing. opts.Stack overrides
// opts.Cell.Stack. opts.Update and opts.Force set NoCache on the go-winkit
// build, forcing a fresh WSL distro. opts.NoCache and opts.Stage are not
// honored.
func (Engine) Build(ctx context.Context, opts engine.BuildOpts) error {
	g, err := guestFor(opts.Cell.Guest)
	if err != nil {
		return err
	}
	c := opts.Cell.WithStack(opts.Stack)
	if opts.DryRun {
		fmt.Printf("%s build (dry-run)\n", g.label())
		fmt.Printf("  dest:       %s\n", artifactPath(c.HostHome, g, c.Stack, c.Modules))
		fmt.Printf("  cache:      %s\n", CacheDir(c.HostHome))
		fmt.Printf("  windowsISO: %s\n", c.Config.Cell.ResolvedWinkitWindowsISO())
		fmt.Printf("  virtioISO:  %s\n", VirtioISOPath(c.HostHome))
		return nil
	}

	pr := &ux.PhaseRunner{}

	var windowsISO, virtioISO string
	if err := pr.PhaseDetailed("Resolving installer media", func() (string, error) {
		obs := &phaseObserver{logf: ux.Debugf, runner: pr}
		var mediaErr error
		windowsISO, virtioISO, mediaErr = ensureMedia(ctx, c, obs)
		if mediaErr != nil {
			return "", mediaErr
		}
		return fmt.Sprintf("Windows=%s VirtIO=%s", filepath.Base(windowsISO), filepath.Base(virtioISO)), nil
	}); err != nil {
		return err
	}

	buildCfg := buildConfig(c, g, windowsISO, virtioISO, pr)
	if opts.Update || opts.Force {
		buildCfg.NoCache = true
	}

	if err := pr.Phase("Preparing template directory", func() error {
		return os.MkdirAll(filepath.Dir(buildCfg.Dest), 0o755)
	}); err != nil {
		return fmt.Errorf("creating template dir: %w", err)
	}

	ux.Debugf("%s build: dest=%s windowsISO=%s virtioISO=%s noCache=%v",
		g.label(), buildCfg.Dest, buildCfg.WindowsISO, buildCfg.VirtIOISO, buildCfg.NoCache)
	ux.Debugf("build ports: gosshd=%d openssh=%d rdp=%d",
		buildCfg.Opts.Ports.Gossh, buildCfg.Opts.Ports.OpenSSH, buildCfg.Opts.Ports.RDP)

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Screenshots: capture QMP screendumps every 30s into the debug dir,
	// matching what `winkit build --debug` does via its CLI layer.
	installOut := filepath.Join(buildCfg.WorkDir, "install")
	shotDir := filepath.Join(buildCfg.WorkDir, "screenshots")
	stopShots := make(chan struct{})
	shotsDone := make(chan struct{})
	if ux.Verbose {
		if mkErr := os.MkdirAll(shotDir, 0o755); mkErr == nil {
			qmpSock := qemu.QMPSocketPath(qemu.Spec{
				VMName: "winkit-install", QMPSocketDir: installOut,
			})
			go qemu.CaptureScreenshots(qmpSock, shotDir, stopShots, shotsDone, func(f string, a ...any) {
				ux.Debugf("winkit: %s", fmt.Sprintf(f, a...))
			})
			ux.Debugf("saving VM screenshots to %s", shotDir)
		} else {
			close(shotsDone)
		}
	} else {
		close(shotsDone)
	}
	defer func() { close(stopShots); <-shotsDone }()

	if err := pr.PhaseDetailed("Building "+g.label()+" image", func() (string, error) {
		if buildErr := winkitBuildFunc(ctx, buildCfg); buildErr != nil {
			return "", fmt.Errorf("%w (debug: %s)", buildErr, buildCfg.WorkDir)
		}
		return buildCfg.Dest, nil
	}); err != nil {
		return err
	}

	// Merge the guest event stream into the structured build log.
	if buildCfg.StructuredLogPath != "" {
		guestLog := filepath.Join(buildCfg.WorkDir, "guest.jsonl")
		if _, statErr := os.Stat(guestLog); statErr == nil {
			if f, openErr := os.OpenFile(buildCfg.StructuredLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); openErr == nil {
				if mergeErr := build.MergeGuestLog(f, buildCfg.WorkDir); mergeErr != nil {
					ux.Debugf("guest log merge: %v", mergeErr)
				}
				f.Close()
			}
		}
	}

	// Sanity: a full install writes many GB; anything near the empty-qcow2
	// floor means the OS never landed. PE builds are small by design.
	if g.full {
		const minInstalledBytes = 4 * 1024 * 1024 * 1024
		if fi, statErr := os.Stat(buildCfg.Dest); statErr != nil {
			return fmt.Errorf("produced disk missing: %w (debug: %s)", statErr, buildCfg.WorkDir)
		} else if fi.Size() < minInstalledBytes {
			return fmt.Errorf("produced disk too small (%.1f GB) — install likely did not complete (debug: %s)",
				float64(fi.Size())/(1<<30), buildCfg.WorkDir)
		}
	}

	return nil
}

// ensureMedia resolves the Windows and VirtIO ISOs, downloading them
// automatically when they are not cached or configured. This gives
// `cell build` the same self-contained UX as docker's build path.
func ensureMedia(ctx context.Context, c engine.Cell, obs Observer) (windowsISO, virtioISO string, err error) {
	windowsISO, err = ResolveWindowsISO(
		cfg.Getenv("DEVCELL_WINKIT_WINDOWS_ISO"),
		c.Config.Cell.ResolvedWinkitWindowsISO(),
		c.HostHome,
	)
	if err != nil {
		ux.Debugf("winkit: Windows ISO not cached, downloading")
		windowsISO, err = DownloadWindowsISO(ctx, c.HostHome, "en-us", obs)
		if err != nil {
			return "", "", fmt.Errorf("downloading Windows ISO: %w", err)
		}
	}

	virtioISO = VirtioISOPath(c.HostHome)
	if _, statErr := os.Stat(virtioISO); statErr != nil {
		ux.Debugf("winkit: VirtIO drivers not cached, downloading")
		virtioISO, err = DownloadVirtioDrivers(ctx, c.HostHome, false, obs)
		if err != nil {
			return "", "", fmt.Errorf("downloading VirtIO drivers: %w", err)
		}
	}

	return windowsISO, virtioISO, nil
}

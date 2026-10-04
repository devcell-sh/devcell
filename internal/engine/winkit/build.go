//go:build darwin || linux

package winkit

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	gowinkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/build"
	"github.com/devcell-sh/go-winkit/build/buildopts"
	"github.com/devcell-sh/go-winkit/build/imageformat"

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
	return &buildopts.BuildOpts{
		PE: !g.full,
		WSL: &buildopts.WSLConfig{
			Image:   "nix",
			NixHome: nixHome,
		},
	}
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
func buildConfig(c engine.Cell, g guest, windowsISO, virtioISO string) build.Config {
	return build.Config{
		Dest:       artifactPath(c.HostHome, g, c.Stack, c.Modules),
		CacheDir:   CacheDir(c.HostHome),
		WindowsISO: windowsISO,
		VirtIOISO:  virtioISO,
		Opts:       buildOpts(g, os.Getenv("DEVCELL_NIXHOME")),
	}
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
// WSL1 nix distro. The installer media come from `cell init`; Build does
// not download them. opts.Stack overrides opts.Cell.Stack. opts.Force,
// opts.NoCache, opts.Update and opts.Stage are not honored.
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

	windowsISO, err := ResolveWindowsISO(
		cfg.Getenv("DEVCELL_WINKIT_WINDOWS_ISO"),
		c.Config.Cell.ResolvedWinkitWindowsISO(),
		c.HostHome,
	)
	if err != nil {
		return fmt.Errorf("resolving Windows ISO: %w", err)
	}

	virtioISO := VirtioISOPath(c.HostHome)
	if _, statErr := os.Stat(virtioISO); statErr != nil {
		return fmt.Errorf("VirtIO drivers not found at %s: run `cell init --engine winkit` first", virtioISO)
	}

	buildCfg := buildConfig(c, g, windowsISO, virtioISO)

	if err := os.MkdirAll(filepath.Dir(buildCfg.Dest), 0o755); err != nil {
		return fmt.Errorf("creating template dir: %w", err)
	}

	ux.Debugf("%s build: dest=%s windowsISO=%s virtioISO=%s", g.label(), buildCfg.Dest, buildCfg.WindowsISO, buildCfg.VirtIOISO)

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return winkitBuildFunc(ctx, buildCfg)
}

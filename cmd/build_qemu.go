//go:build darwin || linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	winkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/build"
	"github.com/devcell-sh/go-winkit/build/buildopts"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/vm/qemu"
)

// winkitBuildFunc is the function used to build PE images. Defaults to
// winkit.Build; tests replace it to capture the config without running
// a real build.
var winkitBuildFunc = winkit.Build

// peBuildConfig assembles a go-winkit build.Config for a PE+WSL1+Nix image
// from devcell's cell config. windowsISO and virtioISO are resolved paths
// passed in by the caller.
func peBuildConfig(cellName, hostHome, stack string, cellCfg cfg.CellSection, windowsISO, virtioISO string) build.Config {
	modules := cellCfg.Modules
	templateDir := qemu.TemplateDir(hostHome, stack, modules)
	dest := filepath.Join(templateDir, "winkit-core.qcow2")
	cacheDir := qemu.CacheDir(hostHome)

	nixHome := os.Getenv("DEVCELL_NIXHOME")

	return build.Config{
		Dest:       dest,
		CacheDir:   cacheDir,
		WindowsISO: windowsISO,
		VirtIOISO:  virtioISO,
		Opts: &buildopts.BuildOpts{
			PE: true,
			WSL: &buildopts.WSLConfig{
				Image:   "nix",
				NixHome: nixHome,
			},
		},
	}
}

// peImagePath returns the path to the PE+WSL1 boot volume artifact for the
// given stack and modules.
func peImagePath(hostHome, stack string, modules []string) string {
	templateDir := qemu.TemplateDir(hostHome, stack, modules)
	return filepath.Join(templateDir, "winkit-core.qcow2")
}

// peStartOpts assembles winkit.StartOpts for booting a PE+WSL1+Nix image.
func peStartOpts(cellName, hostHome, stack string, cellCfg cfg.CellSection, sshPort, rdpPort uint16) winkit.StartOpts {
	return winkit.StartOpts{
		Image:    peImagePath(hostHome, stack, cellCfg.Modules),
		Name:     cellName,
		SSHPort:  sshPort,
		RDPPort:  rdpPort,
		CPUs:     uint(cellCfg.ResolvedQemuCPUs()),
		MemoryGB: uint64(cellCfg.ResolvedQemuMemoryGB()),
	}
}

// peGuestCommand assembles the PowerShell command that runs inside the PE
// guest. User args are forwarded into WSL1 via `wsl -d winkit -- <args>`.
// With no args, an interactive WSL shell is opened.
func peGuestCommand(userArgs []string) string {
	if len(userArgs) == 0 {
		return "wsl -d winkit"
	}
	return "wsl -d winkit -- " + strings.Join(userArgs, " ")
}

// runBuildQemu builds a Windows VM image using WinPE + WSL1 + Nix via go-winkit.
func runBuildQemu(cellName, hostHome, baseDir, stack string, force, noCache, dryRun bool, cellCfg cfg.CellSection) error {
	if dryRun {
		dest := peImagePath(hostHome, stack, cellCfg.Modules)
		fmt.Printf("PE+WSL1 build (dry-run)\n")
		fmt.Printf("  dest:       %s\n", dest)
		fmt.Printf("  cache:      %s\n", qemu.CacheDir(hostHome))
		fmt.Printf("  windowsISO: %s\n", cellCfg.ResolvedQemuWindowsISO())
		fmt.Printf("  virtioISO:  %s\n", qemu.VirtioISOPath(hostHome))
		return nil
	}

	windowsISO, err := qemu.ResolveWindowsISO(
		os.Getenv("DEVCELL_QEMU_WINDOWS_ISO"),
		cellCfg.ResolvedQemuWindowsISO(),
		hostHome,
	)
	if err != nil {
		return fmt.Errorf("resolving Windows ISO: %w", err)
	}

	virtioISO := qemu.VirtioISOPath(hostHome)
	if _, statErr := os.Stat(virtioISO); statErr != nil {
		return fmt.Errorf("VirtIO drivers not found at %s: run `cell init --engine=qemu` first", virtioISO)
	}

	buildCfg := peBuildConfig(cellName, hostHome, stack, cellCfg, windowsISO, virtioISO)

	if err := os.MkdirAll(filepath.Dir(buildCfg.Dest), 0o755); err != nil {
		return fmt.Errorf("creating template dir: %w", err)
	}

	ux.Debugf("PE+WSL1 build: dest=%s windowsISO=%s virtioISO=%s", buildCfg.Dest, buildCfg.WindowsISO, buildCfg.VirtIOISO)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return winkitBuildFunc(ctx, buildCfg)
}

//go:build darwin || linux

package main

import (
	"fmt"
	"os"
	"path/filepath"

	winkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/build"
	"github.com/devcell-sh/go-winkit/build/buildopts"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/vm/qemu"
)

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

// runBuildQemu builds a Windows VM image using WinPE + WSL1 + Nix via go-winkit.
func runBuildQemu(cellName, hostHome, baseDir, stack string, force, noCache, dryRun bool, cellCfg cfg.CellSection) error {
	return fmt.Errorf("PE+WSL1 build path not yet implemented")
}

//go:build darwin || linux

package main

import (
	"testing"

	"github.com/devcell-sh/go-winkit/build/buildopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestPEBuildConfig_SetsWinPEWithWSL1Nix(t *testing.T) {
	cellCfg := cfg.CellSection{Stack: "base"}
	c := peBuildConfig("test-cell", "/home/testuser", "base", cellCfg, "/fake/windows.iso", "/fake/virtio.iso")

	require.NotNil(t, c.Opts, "build opts must be set")
	assert.True(t, c.Opts.PE, "PE mode must be enabled for --os=windows")
	require.NotNil(t, c.Opts.WSL, "WSL config must be set for PE+WSL1+Nix")
	assert.Equal(t, "nix", c.Opts.WSL.Image, "WSL image must be nix")
}

func TestPEBuildConfig_PathsUseQemuCacheDir(t *testing.T) {
	cellCfg := cfg.CellSection{Stack: "base"}
	c := peBuildConfig("test-cell", "/home/testuser", "base", cellCfg, "/fake/windows.iso", "/fake/virtio.iso")

	assert.Contains(t, c.CacheDir, ".devcell/cache/qemu",
		"cache dir must use the QEMU media cache")
	assert.Contains(t, c.Dest, "winkit-core.qcow2",
		"dest must use PE artifact name")
	assert.Equal(t, "/fake/windows.iso", c.WindowsISO)
	assert.Equal(t, "/fake/virtio.iso", c.VirtIOISO)
}

func TestPEBuildConfig_StageIsPE(t *testing.T) {
	cellCfg := cfg.CellSection{}
	c := peBuildConfig("test-cell", "/home/testuser", "base", cellCfg, "/fake/w.iso", "/fake/v.iso")

	assert.Equal(t, buildopts.StagePE, c.Opts.Stage(),
		"build stage must resolve to PE")
}

func TestPEBuildConfig_NixHomeFromEnv(t *testing.T) {
	t.Setenv("DEVCELL_NIXHOME", "/path/to/nixhome")
	cellCfg := cfg.CellSection{}
	c := peBuildConfig("test-cell", "/home/testuser", "base", cellCfg, "/fake/w.iso", "/fake/v.iso")

	assert.Equal(t, "/path/to/nixhome", c.Opts.WSL.NixHome,
		"NixHome must be set from DEVCELL_NIXHOME env var")
}

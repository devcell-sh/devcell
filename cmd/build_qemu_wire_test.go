//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devcell-sh/go-winkit/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestRunBuildQemu_DryRun_PrintsConfig(t *testing.T) {
	cellCfg := cfg.CellSection{Stack: "base"}
	err := runBuildQemu("test-cell", "/home/test", "/tmp/project", "base", false, false, true, cellCfg)
	assert.NoError(t, err, "dry-run must not error")
}

func TestRunBuildQemu_MissingWindowsISO_ReturnsError(t *testing.T) {
	t.Setenv("DEVCELL_QEMU_WINDOWS_ISO", "")
	cellCfg := cfg.CellSection{Stack: "base"}
	err := runBuildQemu("test-cell", t.TempDir(), "/tmp/project", "base", false, false, false, cellCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Windows", "error must mention Windows ISO")
}

func TestRunBuildQemu_CallsBuildWithCorrectConfig(t *testing.T) {
	tmpHome := t.TempDir()
	cacheDir := filepath.Join(tmpHome, ".devcell", "cache", "qemu")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))

	// Create fake ISOs that pass validation (ISO 9660 magic at offset 0x8001)
	fakeISO := make([]byte, 0x8006)
	copy(fakeISO[0x8001:], "CD001")

	winISO := filepath.Join(cacheDir, "windows-arm64-en-us.iso")
	require.NoError(t, os.WriteFile(winISO, fakeISO, 0o644))
	require.NoError(t, os.WriteFile(winISO+".done", []byte("ok"), 0o644))

	virtioISO := filepath.Join(cacheDir, "virtio-win.iso")
	require.NoError(t, os.WriteFile(virtioISO, fakeISO, 0o644))
	require.NoError(t, os.WriteFile(virtioISO+".done", []byte("ok"), 0o644))

	t.Setenv("DEVCELL_QEMU_WINDOWS_ISO", winISO)
	t.Setenv("DEVCELL_NIXHOME", "/fake/nixhome")

	var capturedCfg build.Config
	origBuildFunc := winkitBuildFunc
	winkitBuildFunc = func(_ context.Context, c build.Config) error {
		capturedCfg = c
		return nil
	}
	defer func() { winkitBuildFunc = origBuildFunc }()

	cellCfg := cfg.CellSection{Stack: "base"}
	err := runBuildQemu("test-cell", tmpHome, "/tmp/project", "base", false, false, false, cellCfg)
	require.NoError(t, err)

	assert.Equal(t, winISO, capturedCfg.WindowsISO)
	assert.Equal(t, virtioISO, capturedCfg.VirtIOISO)
	assert.True(t, capturedCfg.Opts.PE, "PE mode must be enabled")
	assert.Equal(t, "nix", capturedCfg.Opts.WSL.Image)
	assert.Equal(t, "/fake/nixhome", capturedCfg.Opts.WSL.NixHome)
	assert.Contains(t, capturedCfg.Dest, "winkit-core.qcow2")
}

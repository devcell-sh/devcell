//go:build darwin || linux

package winkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devcell-sh/go-winkit/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/engine"
)

func TestRunBuildQemu_DryRun_PrintsConfig(t *testing.T) {
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: peCell("/home/test"), DryRun: true})
	assert.NoError(t, err, "dry-run must not error")
}

func TestRunBuildQemu_MissingWindowsISO_ReturnsError(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", "")
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: peCell(t.TempDir())})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Windows", "error must mention Windows ISO")
}

func TestRunBuildQemu_CallsBuildWithCorrectConfig(t *testing.T) {
	tmpHome := t.TempDir()
	winISO, virtioISO := fakeMedia(t, tmpHome)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)
	t.Setenv("DEVCELL_NIXHOME", "/fake/nixhome")

	var capturedCfg build.Config
	stubBuild(t, func(_ context.Context, c build.Config) error {
		capturedCfg = c
		return nil
	})

	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: peCell(tmpHome)})
	require.NoError(t, err)

	assert.Equal(t, winISO, capturedCfg.WindowsISO)
	assert.Equal(t, virtioISO, capturedCfg.VirtIOISO)
	assert.True(t, capturedCfg.Opts.PE, "PE mode must be enabled")
	assert.Equal(t, "nix", capturedCfg.Opts.WSL.Image)
	assert.Equal(t, "/fake/nixhome", capturedCfg.Opts.WSL.NixHome)
	assert.Equal(t, imagePath(tmpHome, guestPE, "base", nil), capturedCfg.Dest)
	assert.Equal(t, "test-cell", capturedCfg.Opts.Hostname,
		"Hostname must be set from cell name")
}

// --stack builds that stack's image instead of the cell's resolved one.
func TestBuild_StackOverridesCellStack(t *testing.T) {
	tmpHome := t.TempDir()
	winISO, _ := fakeMedia(t, tmpHome)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	var capturedCfg build.Config
	stubBuild(t, func(_ context.Context, c build.Config) error {
		capturedCfg = c
		return nil
	})

	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: peCell(tmpHome), Stack: "go"})
	require.NoError(t, err)
	assert.Equal(t, imagePath(tmpHome, guestPE, "go", nil), capturedCfg.Dest)
}

// fakeMedia plants a Windows ISO and VirtIO drivers ISO that pass
// validation (ISO 9660 magic at offset 0x8001) in home's media cache and
// returns their paths.
func fakeMedia(t *testing.T, home string) (windowsISO, virtioISO string) {
	t.Helper()
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	cacheDir := filepath.Join(home, ".devcell", "cache", "qemu")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))

	fakeISO := make([]byte, 0x8006)
	copy(fakeISO[0x8001:], "CD001")

	windowsISO = filepath.Join(cacheDir, "windows-arm64-en-us.iso")
	require.NoError(t, os.WriteFile(windowsISO, fakeISO, 0o644))
	require.NoError(t, os.WriteFile(windowsISO+".done", []byte("ok"), 0o644))

	virtioISO = filepath.Join(cacheDir, "virtio-win.iso")
	require.NoError(t, os.WriteFile(virtioISO, fakeISO, 0o644))
	require.NoError(t, os.WriteFile(virtioISO+".done", []byte("ok"), 0o644))
	return windowsISO, virtioISO
}

// stubBuild replaces the winkit build for one test.
func stubBuild(t *testing.T, fn func(context.Context, build.Config) error) {
	t.Helper()
	orig := winkitBuildFunc
	winkitBuildFunc = fn
	t.Cleanup(func() { winkitBuildFunc = orig })
}

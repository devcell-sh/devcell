//go:build darwin || linux

package winkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devcell-sh/go-winkit/build/buildopts"
	"github.com/devcell-sh/go-winkit/build/imageformat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/engine"
)

func peCell(home string) engine.Cell {
	return engine.Cell{Name: "test-cell", HostHome: home, Stack: "base", Guest: engine.WindowsPE}
}

func TestPEBuildConfig_SetsWinPEWithWSL1Nix(t *testing.T) {
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/windows.iso", "/fake/virtio.iso", nil)

	require.NotNil(t, c.Opts, "build opts must be set")
	assert.True(t, c.Opts.PE, "PE mode must be enabled for --os=windows")
	require.NotNil(t, c.Opts.WSL, "WSL config must be set for PE+WSL1+Nix")
	assert.Equal(t, "nix", c.Opts.WSL.Image, "WSL image must be nix")
}

func TestPEBuildConfig_PathsUseQemuCacheDir(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/windows.iso", "/fake/virtio.iso", nil)

	assert.Contains(t, c.CacheDir, ".devcell/cache/qemu",
		"cache dir must use the QEMU media cache")
	assert.Equal(t, "winkit-pe-wsl.qcow2", filepath.Base(c.Dest),
		"dest must be named after the build stage")
	assert.Equal(t, "/fake/windows.iso", c.WindowsISO)
	assert.Equal(t, "/fake/virtio.iso", c.VirtIOISO)
}

func TestPEBuildConfig_StageIsPEWSL(t *testing.T) {
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Equal(t, buildopts.StagePEWSL, c.Opts.Stage(),
		"build stage must resolve to PE+WSL")
}

// The artifact is named the way `winkit build` names its default output for
// the same stage, so a devcell template dir reads the same as a winkit one.
func TestPEArtifactName_MatchesWinkitDefaultForStage(t *testing.T) {
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	want := imageformat.DefaultOutputName(string(c.Opts.Stage()), imageformat.Qcow2)
	assert.Equal(t, want, filepath.Base(c.Dest))
}

// A rebuild always writes the stage-named artifact, even over a template dir
// that still holds one under the old name.
func TestPEBuildConfig_DestIgnoresLegacyArtifact(t *testing.T) {
	home := t.TempDir()
	legacy := writeLegacyPEArtifact(t, home, "base", nil)

	c := buildConfig(peCell(home), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.NotEqual(t, legacy, c.Dest)
	assert.Equal(t, "winkit-pe-wsl.qcow2", filepath.Base(c.Dest))
}

func TestPEImagePath_NamedAfterStage(t *testing.T) {
	home := t.TempDir()

	got := imagePath(home, guestPE, "base", nil)

	assert.Equal(t, filepath.Join(templateDir(home, guestPE, "base", nil), "winkit-pe-wsl.qcow2"), got)
}

// Templates built before the stage rename hold winkit-core.qcow2. Booting
// must keep finding them rather than demanding a multi-hour rebuild.
func TestPEImagePath_FallsBackToLegacyArtifact(t *testing.T) {
	home := t.TempDir()
	legacy := writeLegacyPEArtifact(t, home, "base", nil)

	assert.Equal(t, legacy, imagePath(home, guestPE, "base", nil))
}

func TestPEImagePath_PrefersStageNameOverLegacy(t *testing.T) {
	home := t.TempDir()
	writeLegacyPEArtifact(t, home, "base", nil)
	current := filepath.Join(templateDir(home, guestPE, "base", nil), "winkit-pe-wsl.qcow2")
	require.NoError(t, os.WriteFile(current, []byte("qcow2"), 0o644))

	assert.Equal(t, current, imagePath(home, guestPE, "base", nil))
}

func writeLegacyPEArtifact(t *testing.T, home, stack string, modules []string) string {
	t.Helper()
	dir := templateDir(home, guestPE, stack, modules)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	p := filepath.Join(dir, "winkit-core.qcow2")
	require.NoError(t, os.WriteFile(p, []byte("qcow2"), 0o644))
	return p
}

func TestPEBuildConfig_NixHomeFromEnv(t *testing.T) {
	t.Setenv("DEVCELL_NIXHOME", "/path/to/nixhome")
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Equal(t, "/path/to/nixhome", c.Opts.WSL.NixHome,
		"NixHome must be set from DEVCELL_NIXHOME env var")
}

func TestWslNixHomeOpts_DerivesAttrFromStack(t *testing.T) {
	cases := []struct {
		stack    string
		wantAttr string
	}{
		{"devcell-ultimate", "wsl-ultimate"},
		{"devcell-base", "wsl-base"},
		{"devcell-go", "wsl-go"},
		{"custom-stack", "wsl-custom-stack"},
	}
	for _, tc := range cases {
		opts := wslNixHomeOpts(tc.stack)
		assert.Equal(t, "nixos", opts.User, "WSL flake user must be nixos")
		assert.Contains(t, opts.Attr, tc.wantAttr,
			"attr must derive from stack %s", tc.stack)
	}
}

func TestBuildConfig_SetsNixHomeOptsWhenNixHomePresent(t *testing.T) {
	cell := peCell("/home/testuser")
	cell.Config.Nix.NixhomePath = "/path/to/nixhome"
	cell.Stack = "devcell-ultimate"
	c := buildConfig(cell, guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	require.NotNil(t, c.NixHomeOpts, "NixHomeOpts must be set when nixhome is configured")
	assert.Equal(t, "nixos", c.NixHomeOpts.User)
	assert.Contains(t, c.NixHomeOpts.Attr, "wsl-ultimate")
}

func TestBuildConfig_NoNixHomeOptsWhenNoNixHome(t *testing.T) {
	t.Setenv("DEVCELL_NIXHOME", "")
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Nil(t, c.NixHomeOpts, "NixHomeOpts must be nil when no nixhome configured")
}

func TestBuildWorkDir_UnderProjectDebugDir(t *testing.T) {
	dir := buildWorkDir("/projects/myapp")

	assert.Contains(t, dir, filepath.Join("/projects/myapp", ".devcell", "debug"))
	assert.Contains(t, dir, "-build")
}

func TestBuildWorkDir_FallsBackToTempDir(t *testing.T) {
	dir := buildWorkDir("")

	assert.Contains(t, dir, "winkit-build-")
}

func TestBuildConfig_SetsWorkDir(t *testing.T) {
	cell := peCell("/home/testuser")
	cell.BaseDir = "/projects/myapp"
	c := buildConfig(cell, guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Contains(t, c.WorkDir, filepath.Join("/projects/myapp", ".devcell", "debug"))
}

func TestBuildConfig_SetsStructuredLogPath(t *testing.T) {
	cell := peCell("/home/testuser")
	cell.BaseDir = "/projects/myapp"
	c := buildConfig(cell, guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.NotEmpty(t, c.StructuredLogPath, "StructuredLogPath must be set")
	assert.Contains(t, c.StructuredLogPath, c.WorkDir,
		"structured log must live inside WorkDir")
	assert.Contains(t, c.StructuredLogPath, "build.jsonl")
}

func TestBuildConfig_SetsAccelFromEnv(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_ACCEL", "tcg")
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Equal(t, "tcg", c.Accel,
		"Accel must be set from DEVCELL_WINKIT_ACCEL")
}

func TestBuildConfig_AccelEmptyByDefault(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_ACCEL", "")
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Empty(t, c.Accel,
		"Accel must be empty by default (go-winkit picks the best)")
}

func TestBuildConfig_SetsDisplayTypeFromEnv(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_VNC", "0")
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Equal(t, "vnc=:0", c.DisplayType,
		"DisplayType must be set from DEVCELL_WINKIT_VNC")
}

func TestBuildConfig_DisplayTypeEmptyByDefault(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_VNC", "")
	c := buildConfig(peCell("/home/testuser"), guestPE, "/fake/w.iso", "/fake/v.iso", nil)

	assert.Empty(t, c.DisplayType,
		"DisplayType must be empty by default (headless)")
}

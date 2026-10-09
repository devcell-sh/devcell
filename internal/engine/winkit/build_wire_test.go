//go:build darwin || linux

package winkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/devcell-sh/go-winkit/build"
	"github.com/devcell-sh/go-winkit/media"
	"github.com/devcell-sh/go-winkit/media/uupdump"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/engine"
)

func TestRunBuildQemu_DryRun_PrintsConfig(t *testing.T) {
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: peCell("/home/test"), DryRun: true})
	assert.NoError(t, err, "dry-run must not error")
}

func TestRunBuildQemu_DownloadFailureReturnsError(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", "")
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	stubFetchWindows(t, func(context.Context, media.FetchOptions) (media.FetchResult, error) {
		return media.FetchResult{}, fmt.Errorf("network unavailable")
	})
	stubFetchFull(t, func(context.Context, uupdump.FetchConfig) (string, error) {
		return "", fmt.Errorf("network unavailable")
	})
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
	assert.NotNil(t, capturedCfg.Logger, "Logger must be set so go-winkit emits build progress")
	assert.Equal(t, imagePath(tmpHome, guestPE, "base", nil), capturedCfg.Dest)
	assert.Equal(t, "test-cell", capturedCfg.Opts.Hostname,
		"Hostname must be set from cell name")
	assert.NotZero(t, capturedCfg.Opts.Ports.Gossh,
		"gosshd port must be auto-allocated, not the hardcoded default")
	assert.NotZero(t, capturedCfg.Opts.Ports.OpenSSH,
		"OpenSSH port must be auto-allocated")
	assert.NotZero(t, capturedCfg.Opts.Ports.RDP,
		"RDP port must be auto-allocated")
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

// --update forces a cache-free rebuild of the WSL distro.
func TestBuild_UpdateSetsNoCache(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	var got build.Config
	stubBuild(t, func(_ context.Context, c build.Config) error {
		got = c
		return nil
	})

	err := Engine{}.Build(context.Background(), engine.BuildOpts{
		Cell:   peCell(home),
		Update: true,
	})
	require.NoError(t, err)
	assert.True(t, got.NoCache, "--update must set NoCache on go-winkit build")
}

// --force replaces an existing image.
func TestBuild_ForceSetsNoCache(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	var got build.Config
	stubBuild(t, func(_ context.Context, c build.Config) error {
		got = c
		return nil
	})

	err := Engine{}.Build(context.Background(), engine.BuildOpts{
		Cell:  peCell(home),
		Force: true,
	})
	require.NoError(t, err)
	assert.True(t, got.NoCache, "--force must set NoCache on go-winkit build")
}

// Build ports are auto-allocated from the kernel, not hardcoded. Two
// sequential builds must not get identical port sets (would collide).
func TestBuild_PortsAreAutoAllocated(t *testing.T) {
	ports := allocateBuildPorts()
	assert.NotZero(t, ports.Gossh, "gosshd port must be allocated")
	assert.NotZero(t, ports.OpenSSH, "OpenSSH port must be allocated")
	assert.NotZero(t, ports.RDP, "RDP port must be allocated")
	assert.NotEqual(t, ports.Gossh, ports.OpenSSH, "ports must be distinct")
	assert.NotEqual(t, ports.Gossh, ports.RDP, "ports must be distinct")
	assert.NotEqual(t, ports.OpenSSH, ports.RDP, "ports must be distinct")
}

func TestFormatGuestEvent_ParsesMsg(t *testing.T) {
	got := formatGuestEvent(`{"ts":"2026-10-05T18:11:23Z","event":"init-wpeinit","msg":"running wpeinit","dir":"out"}`)
	assert.Equal(t, "[init-wpeinit] running wpeinit", got)
}

func TestFormatGuestEvent_ParsesLine(t *testing.T) {
	got := formatGuestEvent(`{"ts":"2026-10-05T18:11:23Z","event":"setupact","line":"Installing drivers","src":"X:\\panther\\setupact.log"}`)
	assert.Equal(t, "[setupact] Installing drivers", got)
}

func TestFormatGuestEvent_NonJSON(t *testing.T) {
	assert.Empty(t, formatGuestEvent("not json at all"))
}

func TestFormatGuestEvent_EmptyMsg(t *testing.T) {
	assert.Empty(t, formatGuestEvent(`{"ts":"2026-10-05T18:11:23Z","event":"foo"}`))
}

// Build auto-downloads the Windows ISO when it is not cached or configured,
// instead of telling the user to run `cell init` first.
func TestBuild_AutoDownloadsWhenISONotCached(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", "")
	t.Setenv("DEVCELL_WINKIT_CACHE_DIR", "")
	t.Setenv("DEVCELL_NIXHOME", "")

	fakeISO := make([]byte, 0x8006)
	copy(fakeISO[0x8001:], "CD001")

	stubFetchWindows(t, func(_ context.Context, opts media.FetchOptions) (media.FetchResult, error) {
		isoPath := filepath.Join(opts.CacheDir, "windows-arm64-en-us.iso")
		require.NoError(t, os.MkdirAll(opts.CacheDir, 0o755))
		require.NoError(t, os.WriteFile(isoPath, fakeISO, 0o644))
		return media.FetchResult{Path: isoPath}, nil
	})
	stubFetchVirtio(t, func(_ context.Context, opts media.FetchOptions) (string, error) {
		isoPath := filepath.Join(opts.CacheDir, "virtio-win.iso")
		require.NoError(t, os.MkdirAll(opts.CacheDir, 0o755))
		require.NoError(t, os.WriteFile(isoPath, fakeISO, 0o644))
		return isoPath, nil
	})

	var built bool
	stubBuild(t, func(_ context.Context, c build.Config) error {
		built = true
		assert.NotEmpty(t, c.WindowsISO, "Windows ISO must be resolved")
		assert.NotEmpty(t, c.VirtIOISO, "VirtIO ISO must be resolved")
		return nil
	})

	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: peCell(home)})
	require.NoError(t, err)
	assert.True(t, built, "build must run after auto-downloading media")
}

func stubFetchWindows(t *testing.T, fn func(context.Context, media.FetchOptions) (media.FetchResult, error)) {
	t.Helper()
	orig := fetchWindowsMedia
	fetchWindowsMedia = fn
	t.Cleanup(func() { fetchWindowsMedia = orig })
}

func stubFetchFull(t *testing.T, fn func(context.Context, uupdump.FetchConfig) (string, error)) {
	t.Helper()
	orig := fetchFullMedia
	fetchFullMedia = fn
	t.Cleanup(func() { fetchFullMedia = orig })
}

func stubFetchVirtio(t *testing.T, fn func(context.Context, media.FetchOptions) (string, error)) {
	t.Helper()
	orig := fetchVirtioMedia
	fetchVirtioMedia = fn
	t.Cleanup(func() { fetchVirtioMedia = orig })
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

func TestBuild_SetsStructuredLogPath(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	var got build.Config
	stubBuild(t, func(_ context.Context, c build.Config) error {
		got = c
		return nil
	})

	cell := peCell(home)
	cell.BaseDir = t.TempDir()
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: cell})
	require.NoError(t, err)
	assert.Contains(t, got.StructuredLogPath, "build.jsonl",
		"StructuredLogPath must point to build.jsonl inside WorkDir")
}

func TestBuild_DiskSanityCheck_FailsWhenTooSmall(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	stubBuild(t, func(_ context.Context, c build.Config) error {
		// Write a tiny file at Dest to simulate a failed install.
		require.NoError(t, os.MkdirAll(filepath.Dir(c.Dest), 0o755))
		require.NoError(t, os.WriteFile(c.Dest, make([]byte, 1024), 0o644))
		return nil
	})

	cell := engine.Cell{
		Name: "test-cell", HostHome: home, Stack: "base",
		Guest: engine.WindowsFull,
	}
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: cell})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too small",
		"build must fail when produced disk is suspiciously small")
}

func TestBuild_DiskSanityCheck_SkippedForPE(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	stubBuild(t, func(_ context.Context, c build.Config) error {
		// Write a tiny file at Dest — this is fine for PE.
		require.NoError(t, os.MkdirAll(filepath.Dir(c.Dest), 0o755))
		require.NoError(t, os.WriteFile(c.Dest, make([]byte, 1024), 0o644))
		return nil
	})

	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: peCell(home)})
	require.NoError(t, err, "PE builds must skip the disk size sanity check")
}

func TestBuild_ErrorReportsDebugDir(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	stubBuild(t, func(_ context.Context, c build.Config) error {
		return fmt.Errorf("install failed")
	})

	cell := peCell(home)
	cell.BaseDir = t.TempDir()
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: cell})
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".devcell/debug",
		"error must include the debug dir path for post-mortem")
}

func TestBuild_GuestLogMerged(t *testing.T) {
	home := t.TempDir()
	winISO, _ := fakeMedia(t, home)
	t.Setenv("DEVCELL_WINKIT_WINDOWS_ISO", winISO)

	var capturedCfg build.Config
	stubBuild(t, func(_ context.Context, c build.Config) error {
		capturedCfg = c
		// Simulate go-winkit writing a guest.jsonl.
		require.NoError(t, os.MkdirAll(c.WorkDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(c.WorkDir, "guest.jsonl"),
			[]byte(`{"ts":"2026-10-06","event":"test","msg":"hello"}`+"\n"),
			0o644,
		))
		return nil
	})

	cell := peCell(home)
	cell.BaseDir = t.TempDir()
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Cell: cell})
	require.NoError(t, err)

	// The structured log path should exist and contain the merged guest data.
	logPath := capturedCfg.StructuredLogPath
	assert.FileExists(t, logPath, "build.jsonl must exist")
	data, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "hello",
		"guest events must be merged into the structured log")
}

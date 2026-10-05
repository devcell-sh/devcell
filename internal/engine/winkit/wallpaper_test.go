//go:build darwin || linux

package winkit

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderWallpaper_EmptyNixHome(t *testing.T) {
	png, err := RenderWallpaper("", "test-cell")
	assert.NoError(t, err)
	assert.Nil(t, png)
}

func TestRenderWallpaper_MissingSVG(t *testing.T) {
	png, err := RenderWallpaper(t.TempDir(), "test-cell")
	assert.NoError(t, err)
	assert.Nil(t, png)
}

func TestRenderWallpaper_SubstitutesCellID(t *testing.T) {
	dir := t.TempDir()
	svgDir := filepath.Join(dir, filepath.Dir(wallpaperSVGRel))
	require.NoError(t, os.MkdirAll(svgDir, 0o755))
	svg := `<svg><text>{{CELL_ID}}</text></svg>`
	require.NoError(t, os.WriteFile(filepath.Join(dir, wallpaperSVGRel), []byte(svg), 0o644))

	if _, err := lookPathRsvg(); err != nil {
		t.Skip("rsvg-convert not in PATH")
	}

	png, err := RenderWallpaper(dir, "my-cell")
	require.NoError(t, err)
	assert.NotEmpty(t, png, "should produce PNG output")
	assert.True(t, len(png) > 8, "PNG should be more than a header")
}

func TestSetWallpaperScript_ContainsBase64(t *testing.T) {
	data := []byte("fake-png-data")
	script := SetWallpaperScript(data)
	assert.Contains(t, script, "FromBase64String")
	assert.Contains(t, script, "SystemParametersInfo")
	assert.Contains(t, script, `C:\Windows\Web\Wallpaper\winkit`)
}

func TestSetWallpaperGuestCommand_PEReturnsEmpty(t *testing.T) {
	cmd := SetWallpaperGuestCommand(guestPE, []byte("png"))
	assert.Empty(t, cmd, "PE mode should return empty (no Explorer shell)")
}

func TestSetWallpaperGuestCommand_FullReturnsPowerShell(t *testing.T) {
	cmd := SetWallpaperGuestCommand(guestFull, []byte("png"))
	assert.True(t, strings.HasPrefix(cmd, "powershell.exe"), "full mode should invoke powershell.exe")
	assert.Contains(t, cmd, "SystemParametersInfo")
}

func TestSetRuntimeWallpaper_SkipsWhenNoNixHome(t *testing.T) {
	t.Setenv("DEVCELL_NIXHOME", "")
	var logged []string
	logf := func(format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	}
	ch := sshChannel{port: 12345, user: "test", password: "test"}
	setRuntimeWallpaper(context.Background(), ch, "cell", logf)
	assert.NotEmpty(t, logged)
	assert.Contains(t, logged[0], "skipped")
}

type mockSession struct {
	cmd      string
	exitCode int
}

func (m *mockSession) RunStream(_ context.Context, cmd string, _, _ io.Writer) (int, error) {
	m.cmd = cmd
	return m.exitCode, nil
}

func (m *mockSession) Close() error { return nil }

func TestSetRuntimeWallpaper_ExecutesOnFull(t *testing.T) {
	dir := t.TempDir()
	svgDir := filepath.Join(dir, filepath.Dir(wallpaperSVGRel))
	require.NoError(t, os.MkdirAll(svgDir, 0o755))
	svg := `<svg><text>{{CELL_ID}}</text></svg>`
	require.NoError(t, os.WriteFile(filepath.Join(dir, wallpaperSVGRel), []byte(svg), 0o644))
	t.Setenv("DEVCELL_NIXHOME", dir)

	if _, err := lookPathRsvg(); err != nil {
		t.Skip("rsvg-convert not in PATH")
	}

	ms := &mockSession{}
	origDial := dialGuest
	dialGuest = func(_ context.Context, _ sshChannel) (guestSession, error) {
		return ms, nil
	}
	t.Cleanup(func() { dialGuest = origDial })

	var logged []string
	logf := func(format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	}
	ch := sshChannel{port: 12345, user: "test", password: "test"}
	setRuntimeWallpaper(context.Background(), ch, "work", logf)

	assert.NotEmpty(t, ms.cmd, "should have executed a command over SSH")
	assert.Contains(t, ms.cmd, "powershell.exe")
	assert.Contains(t, ms.cmd, "SystemParametersInfo")
}

func lookPathRsvg() (string, error) {
	return exec.LookPath("rsvg-convert")
}

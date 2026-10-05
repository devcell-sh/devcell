//go:build darwin || linux

package winkit

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DimmKirr/devcell/internal/ux"
)

const (
	wallpaperSVGRel  = "modules/desktop/themes/main/svg/wallpaper.svg"
	wallpaperPNGName = "wallpaper.png"
	wallpaperWidth   = "3840"
	wallpaperHeight  = "2160"
)

// RenderWallpaper renders the devcell wallpaper SVG to a PNG, substituting
// cellID for the {{CELL_ID}} placeholder. An empty cellID strips it.
// Returns nil, nil when the SVG or rsvg-convert is not available.
func RenderWallpaper(nixHome, cellID string) ([]byte, error) {
	if nixHome == "" {
		return nil, nil
	}
	svgPath := filepath.Join(nixHome, wallpaperSVGRel)
	if _, err := os.Stat(svgPath); err != nil {
		ux.Debugf("wallpaper: SVG not found at %s", svgPath)
		return nil, nil
	}
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		ux.Debugf("wallpaper: rsvg-convert not in PATH")
		return nil, nil
	}

	svg, err := os.ReadFile(svgPath)
	if err != nil {
		return nil, fmt.Errorf("reading wallpaper SVG: %w", err)
	}
	replaced := strings.ReplaceAll(string(svg), "{{CELL_ID}}", cellID)

	cmd := exec.Command("rsvg-convert", "-w", wallpaperWidth, "-h", wallpaperHeight)
	cmd.Stdin = strings.NewReader(replaced)
	png, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rsvg-convert: %w", err)
	}
	return png, nil
}

// SetWallpaperScript returns a PowerShell command that decodes a base64
// PNG, writes it to C:\Windows\Web\Wallpaper\winkit\wallpaper.png, and
// applies it as the desktop wallpaper via SystemParametersInfo.
func SetWallpaperScript(pngData []byte) string {
	b64 := base64.StdEncoding.EncodeToString(pngData)
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$wpDir = 'C:\Windows\Web\Wallpaper\winkit'
if (-not (Test-Path $wpDir)) { New-Item -ItemType Directory -Path $wpDir -Force | Out-Null }
$wpPath = Join-Path $wpDir 'wallpaper.png'
[IO.File]::WriteAllBytes($wpPath, [Convert]::FromBase64String('%s'))
Set-ItemProperty -Path 'HKCU:\Control Panel\Desktop' -Name Wallpaper -Value $wpPath -Type String
Set-ItemProperty -Path 'HKCU:\Control Panel\Desktop' -Name WallpaperStyle -Value '10' -Type String
Set-ItemProperty -Path 'HKCU:\Control Panel\Desktop' -Name TileWallpaper -Value '0' -Type String
if (-not ('WinkitWallpaper' -as [type])) {
    Add-Type -TypeDefinition 'using System.Runtime.InteropServices; public class WinkitWallpaper { [DllImport("user32.dll", CharSet = CharSet.Auto)] public static extern int SystemParametersInfo(int uAction, int uParam, string lpvParam, int fuWinIni); }'
}
[WinkitWallpaper]::SystemParametersInfo(0x0014, 0, $wpPath, 0x03) | Out-Null`, b64)
}

// SetWallpaperGuestCommand wraps SetWallpaperScript in a guest-executable
// command for a full Windows install (runs via Windows OpenSSH as the
// session user). PE mode is not supported (no Explorer shell).
func SetWallpaperGuestCommand(g guest, pngData []byte) string {
	if !g.full {
		return ""
	}
	ps1 := SetWallpaperScript(pngData)
	return `powershell.exe -NoProfile -Command "` + strings.ReplaceAll(ps1, `"`, `\"`) + `"`
}

// setRuntimeWallpaper renders the cell wallpaper with cellName baked in and
// applies it to the running Windows guest over SSH. Best-effort: logs
// failures but never returns an error.
func setRuntimeWallpaper(ctx context.Context, ch sshChannel, cellName string, logf func(string, ...any)) {
	nixHome := os.Getenv("DEVCELL_NIXHOME")
	png, err := RenderWallpaper(nixHome, cellName)
	if err != nil {
		logf("wallpaper: render failed: %v", err)
		return
	}
	if len(png) == 0 {
		logf("wallpaper: skipped (no SVG or rsvg-convert)")
		return
	}

	cmd := SetWallpaperGuestCommand(guestFull, png)
	if cmd == "" {
		return
	}

	logf("wallpaper: setting runtime wallpaper for cell %q (%d bytes PNG)", cellName, len(png))
	s, err := dialGuest(ctx, ch)
	if err != nil {
		logf("wallpaper: SSH dial failed: %v", err)
		return
	}
	defer s.Close()

	exitCode, err := s.RunStream(ctx, cmd, io.Discard, io.Discard)
	if err != nil {
		logf("wallpaper: command failed: %v", err)
		return
	}
	if exitCode != 0 {
		logf("wallpaper: command exited %d", exitCode)
		return
	}
	logf("wallpaper: runtime wallpaper set for cell %q", cellName)
}

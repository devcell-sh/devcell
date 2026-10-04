package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/gui"
	"github.com/DimmKirr/devcell/internal/telemetry"
	"github.com/spf13/cobra"
)

var vncCmd = &cobra.Command{
	Use:   "vnc [app-name or suffix]",
	Short: "Open VNC connection to the running devcell container",
	Long: `Open a VNC connection to a running devcell container.

When multiple containers are running, specify which one by app name or
just the numeric suffix:

    cell vnc devcell-271
    cell vnc 271`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runVNC,
	ValidArgsFunction: completeRunningApps,
}

func init() {
	vncCmd.Flags().Bool("list", false, "list all running cell containers and their VNC ports")
	vncCmd.Flags().Bool("global", false, "include docker cells of all projects, not just the current one")
	vncCmd.Flags().String("viewer", "", "VNC viewer: royaltsx, tigervnc, screensharing (macOS)")
}

func runVNC(cmd *cobra.Command, args []string) error {
	applyOutputFlagsWithLog("vnc")
	list, _ := cmd.Flags().GetBool("list")
	vncGlobal, _ = cmd.Flags().GetBool("global")
	vncViewer, _ = cmd.Flags().GetString("viewer")

	telemetry.Track("vnc", map[string]any{"viewer": vncViewer, "list": list, "global": vncGlobal})

	if list {
		return listCells(gui.VNC, vncGlobal)
	}
	// The VNC viewers need no config; it is read only to find the cell.
	port, err := cellPort(gui.VNC, args, vncGlobal, config.LoadFromOS)
	if err != nil {
		return err
	}
	return openVNC(port)
}

var vncGlobal bool // set by --global flag

// vncViewer is set by the --viewer flag.
var vncViewer string

// openVNC dispatches to the selected VNC viewer.
// Default: Royal TSX (darwin) → TigerVNC → macOS Screen Sharing (darwin).
func openVNC(port string) error {
	switch vncViewer {
	case "royaltsx":
		return openVNCRoyalTSX(port)
	case "tigervnc":
		return openVNCTigerVNC(port)
	case "screensharing":
		return openVNCScreenSharing(port)
	case "":
		// Auto: Royal TSX → TigerVNC → Screen Sharing
		if runtime.GOOS == "darwin" && gui.HasRoyalTSX() {
			guiDebug(gui.VNC, "auto-detected Royal TSX")
			return openVNCRoyalTSX(port)
		}
		if path, err := exec.LookPath("vncviewer"); err == nil {
			guiDebug(gui.VNC, "auto-detected TigerVNC at %s", path)
			return openVNCTigerVNC(port)
		}
		if runtime.GOOS == "darwin" {
			guiDebug(gui.VNC, "falling back to macOS Screen Sharing")
			fmt.Fprintf(os.Stderr, "Tip: for a better VNC experience, install one of:\n"+
				"  1. Royal TSX  — https://royalapps.com/ts/mac\n"+
				"  2. TigerVNC   — brew install tiger-vnc\n\n")
			return openVNCScreenSharing(port)
		}
		return fmt.Errorf("no VNC viewer found — install one of:\n\n" +
			"  TigerVNC:\n" +
			"    Debian:  sudo apt install tigervnc-viewer\n" +
			"    Fedora:  sudo dnf install tigervnc\n" +
			"    Arch:    sudo pacman -S tigervnc\n")
	default:
		return fmt.Errorf("unknown viewer %q — use royaltsx, tigervnc, or screensharing", vncViewer)
	}
}

func openVNCRoyalTSX(port string) error {
	guiDebug(gui.VNC, "opening Royal TSX VNC for port %s", port)
	return openURL(gui.RoyalTSXVNCUrl(port))
}

func openVNCTigerVNC(port string) error {
	guiDebug(gui.VNC, "opening TigerVNC for port %s", port)
	cmd := exec.Command("vncviewer", "-passwd", gui.VNCPasswdFile(), "127.0.0.1:"+port)
	if runtime.GOOS == "darwin" {
		return cmd.Start()
	}
	return cmd.Run()
}

func openVNCScreenSharing(port string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("screensharing viewer is only available on macOS")
	}
	guiDebug(gui.VNC, "opening macOS Screen Sharing for port %s", port)
	return openURL(gui.VNCUrl(port))
}

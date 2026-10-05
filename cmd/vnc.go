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
	ep, err := cellEndpoint(gui.VNC, args, vncGlobal, config.LoadFromOS)
	if err != nil {
		return err
	}
	return openVNC(ep)
}

var vncGlobal bool // set by --global flag

// vncViewer is set by the --viewer flag.
var vncViewer string

// openVNC dispatches to the selected VNC viewer.
// Default: Royal TSX (darwin) → TigerVNC → macOS Screen Sharing (darwin).
func openVNC(ep gui.CellEndpoint) error {
	switch vncViewer {
	case "royaltsx":
		return openVNCRoyalTSX(ep)
	case "tigervnc":
		return openVNCTigerVNC(ep)
	case "screensharing":
		return openVNCScreenSharing(ep)
	case "":
		// Auto: Royal TSX → TigerVNC (non-tart only) → Screen Sharing
		if runtime.GOOS == "darwin" && gui.HasRoyalTSX() {
			guiDebug(gui.VNC, "auto-detected Royal TSX")
			return openVNCRoyalTSX(ep)
		}
		if ep.Engine != "tart" {
			if path, err := exec.LookPath("vncviewer"); err == nil {
				guiDebug(gui.VNC, "auto-detected TigerVNC at %s", path)
				return openVNCTigerVNC(ep)
			}
		}
		if runtime.GOOS == "darwin" {
			guiDebug(gui.VNC, "falling back to macOS Screen Sharing")
			fmt.Fprintf(os.Stderr, "Tip: for a better VNC experience, install one of:\n"+
				"  1. Royal TSX  — https://royalapps.com/ts/mac\n"+
				"  2. TigerVNC   — brew install tiger-vnc\n\n")
			return openVNCScreenSharing(ep)
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

// vncCredentials returns (user, password) for VNC auth.
// Tart macOS VMs use ARD auth ($USER:admin); others use VNC legacy ("":vnc).
func vncCredentials(ep gui.CellEndpoint) (string, string) {
	if ep.Engine == "tart" {
		user := ep.User
		if user == "" {
			user = "admin"
		}
		return user, gui.TartVNCPassword
	}
	return "", "vnc"
}

func openVNCRoyalTSX(ep gui.CellEndpoint) error {
	guiDebug(gui.VNC, "opening Royal TSX VNC for %s", ep.Addr())
	user, pass := vncCredentials(ep)
	return openURL(gui.RoyalTSXVNCUrl(ep.Host, ep.Port, user, pass))
}

func openVNCTigerVNC(ep gui.CellEndpoint) error {
	if ep.Engine == "tart" {
		return fmt.Errorf("TigerVNC does not support ARD auth — use screensharing or royaltsx for macOS cells")
	}
	guiDebug(gui.VNC, "opening TigerVNC for %s", ep.Addr())
	cmd := exec.Command("vncviewer", "-passwd", gui.VNCPasswdFile(), ep.Addr())
	if runtime.GOOS == "darwin" {
		return cmd.Start()
	}
	return cmd.Run()
}

func openVNCScreenSharing(ep gui.CellEndpoint) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("screensharing viewer is only available on macOS")
	}
	guiDebug(gui.VNC, "opening macOS Screen Sharing for %s", ep.Addr())
	user, pass := vncCredentials(ep)
	return openURL(gui.VNCUrl(ep.Host, ep.Port, user, pass))
}

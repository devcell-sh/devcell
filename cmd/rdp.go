package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/gui"
	"github.com/DimmKirr/devcell/internal/telemetry"
	"github.com/spf13/cobra"
)

var rdpCmd = &cobra.Command{
	Use:   "rdp [app-name or suffix]",
	Short: "Open RDP connection to the running devcell container",
	Long: `Open an RDP connection to a running devcell container.

When multiple containers are running, specify which one by app name or
just the numeric suffix:

    cell rdp devcell-271
    cell rdp 271`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runRDP,
	ValidArgsFunction: completeRunningApps,
}

func init() {
	rdpCmd.Flags().Bool("list", false, "list all running cell containers and their RDP ports")
	rdpCmd.Flags().Bool("global", false, "include docker cells of all projects, not just the current one")
	rdpCmd.Flags().Bool("fullscreen", false, "open RDP session in fullscreen mode")
	rdpCmd.Flags().String("viewer", "", "RDP viewer: freerdp (default), macrdp, royaltsx")
}

func runRDP(cmd *cobra.Command, args []string) error {
	applyOutputFlagsWithLog("rdp")
	list, _ := cmd.Flags().GetBool("list")
	rdpGlobal, _ = cmd.Flags().GetBool("global")
	rdpFullscreen, _ = cmd.Flags().GetBool("fullscreen")
	rdpViewer, _ = cmd.Flags().GetString("viewer")

	telemetry.Track("rdp", map[string]any{"viewer": rdpViewer, "list": list, "global": rdpGlobal, "fullscreen": rdpFullscreen})

	if list {
		return listCells(gui.RDP, rdpGlobal)
	}
	// The RDP viewers need the config (user, xrdp cert), so load it up front.
	c, err := config.LoadFromOS()
	if err != nil {
		return err
	}
	port, err := cellPort(gui.RDP, args, rdpGlobal, loadedConfig(c))
	if err != nil {
		return err
	}
	return openRDP(c, port)
}

var (
	rdpGlobal     bool   // set by --global flag
	rdpFullscreen bool   // set by --fullscreen flag
	rdpViewer     string // set by --viewer flag
)

// openRDP dispatches to the selected viewer.
// Default: FreeRDP → macOS Windows App fallback (darwin only).
func openRDP(c config.Config, port string) error {
	switch rdpViewer {
	case "macrdp":
		return openMacRDP(port)
	case "royaltsx":
		return openRoyalTSX(c, port)
	case "freerdp":
		return openFreeRDP(c, port)
	case "":
		// Auto: Royal TSX (darwin) → FreeRDP → macOS Windows App (darwin)
		if runtime.GOOS == "darwin" && gui.HasRoyalTSX() {
			guiDebug(gui.RDP, "auto-detected Royal TSX")
			return openRoyalTSX(c, port)
		}
		if client, found := gui.FindClient(); found {
			return openFreeRDPWith(c, port, client)
		}
		if runtime.GOOS == "darwin" {
			guiDebug(gui.RDP, "no Royal TSX or FreeRDP found, falling back to macOS Windows App")
			fmt.Fprintf(os.Stderr, "Tip: install Royal TSX or FreeRDP for a better experience:\n  brew install freerdp\n\n")
			return openMacRDP(port)
		}
		return fmt.Errorf("%s", gui.InstallHint())
	default:
		return fmt.Errorf("unknown viewer %q — use freerdp, macrdp, or royaltsx", rdpViewer)
	}
}

// openFreeRDP connects via FreeRDP (auto-login, clipboard, cert verification).
func openFreeRDP(c config.Config, port string) error {
	client, found := gui.FindClient()
	if !found {
		return fmt.Errorf("%s", gui.InstallHint())
	}
	return openFreeRDPWith(c, port, client)
}

func openFreeRDPWith(c config.Config, port string, client gui.ClientBinary) error {
	certFlag := gui.CertFlag(c.ConfigDir)
	guiDebug(gui.RDP, "using %s (%s), cert: %s", client.Name, client.Path, certFlag)
	args := []string{
		"/v:127.0.0.1:" + port,
		"/u:" + c.HostUser,
		"/p:rdp",
		"/admin",
		certFlag,
		"+clipboard",
		"/log-level:FATAL",
	}
	if rdpFullscreen {
		args = append(args, "/f", "/smart-sizing")
	} else {
		args = append(args, "/w:1920", "/h:1080")
	}
	cmd := exec.Command(client.Path, args...)
	if runtime.GOOS == "darwin" && strings.HasPrefix(client.Name, "sdl-") {
		fmt.Fprintf(os.Stderr, "Using SDL on macOS — the screen may flicker for a moment, this is normal.\n")
	}
	if runtime.GOOS == "darwin" {
		return cmd.Start()
	}
	return cmd.Run()
}

// openMacRDP opens the connection via macOS Windows App (rdp:// URI).
func openMacRDP(port string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("macrdp viewer is only available on macOS")
	}
	guiDebug(gui.RDP, "opening macOS Windows App for port %s", port)
	return openURL(gui.RDPUrl(port))
}

// openRoyalTSX opens the connection via Royal TSX (rtsx:// URI).
func openRoyalTSX(c config.Config, port string) error {
	guiDebug(gui.RDP, "opening Royal TSX for port %s", port)
	return openURL(gui.RoyalTSXUrl(port, c.HostUser, "rdp"))
}

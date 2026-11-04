package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"runtime"

	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/spf13/cobra"
)

var psGlobal bool

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "Show running cells in the current workspace",
	Long: `Show running DevCell containers, filtered to the current workspace by default.
Use --global to show cells across all workspaces.`,
	RunE: runPS,
}

func init() {
	psCmd.Flags().BoolVar(&psGlobal, "global", false, "show cells from all workspaces")
	rootCmd.AddCommand(psCmd)
}

func runPS(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()
	telemetry.Track("ps", map[string]any{"global": psGlobal})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cells, err := docker.ListCells(ctx, false)
	if err != nil {
		return err
	}

	if !psGlobal {
		ws := config.ResolveWorkspaceName(os.Getenv)
		cells = docker.FilterByWorkspace(cells, ws, true)
	}

	homes := hostHomes()
	headers := []string{"CELL", "STATUS", "UPTIME", "PROJECT"}
	rows := make([][]string, 0, len(cells))
	for _, c := range cells {
		rows = append(rows, []string{
			c.Name,
			c.Status,
			c.Uptime,
			docker.ShortenPath(c.Project, homes...),
		})
	}
	ux.PrintTable(headers, rows)
	return nil
}

// hostHomes returns candidate home directory prefixes for path shortening.
// Inside a container, $HOME is /home/<user>, but Docker labels store the
// host path (e.g. /Users/<user> on macOS). Include both so paths shorten
// regardless of where the command runs.
func hostHomes() []string {
	home := os.Getenv("HOME")
	user := os.Getenv("HOST_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	homes := []string{home}
	if runtime.GOOS == "linux" && user != "" {
		macHome := "/Users/" + user
		if macHome != home {
			homes = append(homes, macHome)
		}
	}
	return homes
}

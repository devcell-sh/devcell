package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cells with running status",
	Long: `Enumerate all known cells across all workspaces, including stopped containers.
Shows the engine type, running status, and project for each cell.`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()
	telemetry.Track("list", nil)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cells, err := docker.ListCells(ctx, true)
	if err != nil {
		return err
	}

	homes := hostHomes()
	headers := []string{"CELL", "ENGINE", "STATUS", "PROJECT", "UPTIME"}
	rows := make([][]string, 0, len(cells))
	for _, c := range cells {
		uptime := c.Uptime
		if c.Status != "running" {
			uptime = "—"
		}
		rows = append(rows, []string{
			friendlyCellName(c.Name),
			"docker",
			c.Status,
			docker.ShortenPath(c.Project, homes...),
			uptime,
		})
	}
	ux.PrintTable(headers, rows)
	return nil
}

// friendlyCellName strips the "devcell-" prefix from container names.
func friendlyCellName(name string) string {
	return strings.TrimPrefix(name, "devcell-")
}

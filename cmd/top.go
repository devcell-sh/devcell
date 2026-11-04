package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/spf13/cobra"
)

var topGlobal bool

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Show cells ranked by CPU usage",
	Long: `Show running DevCell containers ranked by CPU usage (highest first).
Filtered to the current workspace by default; use --global for all workspaces.`,
	RunE: runTop,
}

func init() {
	topCmd.Flags().BoolVar(&topGlobal, "global", false, "show cells from all workspaces")
	rootCmd.AddCommand(topCmd)
}

func runTop(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()
	telemetry.Track("top", map[string]any{"global": topGlobal})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cells, err := docker.ListCells(ctx, false)
	if err != nil {
		return err
	}

	if !topGlobal {
		ws := config.ResolveWorkspaceName(os.Getenv)
		cells = docker.FilterByWorkspace(cells, ws, true)
	}
	if len(cells) == 0 {
		fmt.Println("No running cells")
		return nil
	}

	names := make([]string, len(cells))
	projectByName := make(map[string]string, len(cells))
	homes := hostHomes()
	for i, c := range cells {
		names[i] = c.Name
		projectByName[c.Name] = docker.ShortenPath(c.Project, homes...)
	}

	stats, err := docker.ListCellStats(ctx, names)
	if err != nil {
		return err
	}
	docker.SortStatsByCPU(stats)

	headers := []string{"CELL", "CPU%", "MEM", "MEM%", "PROJECT"}
	rows := make([][]string, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, []string{
			s.Name,
			fmt.Sprintf("%.1f%%", s.CPUPercent),
			s.MemUsage,
			fmt.Sprintf("%.1f%%", s.MemPercent),
			projectByName[s.Name],
		})
	}
	ux.PrintTable(headers, rows)
	return nil
}

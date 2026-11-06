package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var reapCmd = &cobra.Command{
	Use:   "reap",
	Short: "Stop stale oneshot containers with no active sessions",
	Long: `Find and stop devcell containers that are running but have no active
agent or shell session. Only oneshot containers (started by cell claude,
cell codex, etc.) are considered; persistent containers (cell start) are
never touched.

A container is stale when it was started for an agent session, but the
session has ended or was interrupted without stopping the container.

Use --dry-run to see what would be stopped.`,
	RunE: runReap,
}

func init() {
	reapCmd.Flags().Bool("dry-run", false, "show stale containers without stopping them")
	reapCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	rootCmd.AddCommand(reapCmd)
}

func runReap(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	telemetry.Track("reap", map[string]any{"dry_run": dryRun})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	stale, err := docker.FindStaleCells(ctx, "")
	if err != nil {
		return err
	}

	if len(stale) == 0 {
		fmt.Println("No stale containers found.")
		return nil
	}

	homes := hostHomes()
	headers := []string{"CELL", "WORKSPACE", "UPTIME", "PROJECT"}
	rows := make([][]string, 0, len(stale))
	for _, s := range stale {
		rows = append(rows, []string{
			s.Name,
			s.Workspace,
			s.Uptime,
			docker.ShortenPath(s.Project, homes...),
		})
	}
	ux.PrintTable(headers, rows)

	if dryRun {
		fmt.Printf("\n%d stale container(s) found (dry run, not stopping).\n", len(stale))
		return nil
	}

	isTTY := isatty.IsTerminal(os.Stdin.Fd())
	if !yes && isTTY {
		fmt.Printf("\nStop %d stale container(s)? [y/N] ", len(stale))
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	errs := docker.ReapStaleCells(ctx, stale)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  error: %v\n", e)
		}
		return fmt.Errorf("failed to stop %d container(s)", len(errs))
	}

	fmt.Printf("Stopped %d stale container(s).\n", len(stale))
	return nil
}

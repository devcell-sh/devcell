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

// `devcell cleanup` — reap GC roots that no RUNNING container references.
// See CELL-334. Retention rule: "in use" = running (docker ps), not merely
// existing. Roots of stopped cells are reaped; those cells rebuild via the
// hydration gate (CELL-38) on next start.
var cleanupCmd = &cobra.Command{
	Use:        "cleanup",
	Short:      "Reap nix GC roots no running cell references (shared volume hygiene)",
	Deprecated: "use `cell build prune` instead (it now prunes both Docker and nix)",
	RunE:       runCleanup,
}

func init() {
	cleanupCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, _ []string) error {
	telemetry.Track("cleanup", nil)
	yes, _ := cmd.Flags().GetBool("yes")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	closures, failures, err := docker.CollectLiveClosures(
		func() ([]string, error) { return docker.DockerRunningDevcellContainers(ctx) },
		func(container, link string) (string, error) {
			return docker.DockerResolveContainerLink(ctx, container, link)
		},
		ux.Debugf,
	)
	if err != nil {
		return err
	}
	if len(failures) > 0 {
		isTTY := isatty.IsTerminal(os.Stdin.Fd())
		if !docker.ConfirmDestructive(os.Stdout, os.Stdin, yes, isTTY, docker.BuildStopBrokenPrompt(failures)) {
			return fmt.Errorf("aborted: %d container(s) with unresolvable closures", len(failures))
		}
		for _, f := range failures {
			docker.DockerStopContainer(ctx, f.Container)
		}
	}

	return docker.RunCleanup(docker.RunCleanupArgs{
		Closures: closures,
		Exec:     func(step docker.PruneStep) error { return execStep(ctx, step) },
		Out:      os.Stdout,
		In:       os.Stdin,
		SkipYes:  yes,
		IsTTY:    isatty.IsTerminal(os.Stdin.Fd()),
	})
}

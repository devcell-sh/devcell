package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var cleanBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Prune Docker daemon and nix store",
	Long: `Prune the local Docker daemon and nix store.

By default, runs a safe prune of both targets:
  - Docker: removes stopped containers, unused images, volumes, BuildKit cache.
  - Nix:    removes orphaned profile generations and unreferenced store paths.
            Running containers and their nix closures are always protected.

--force escalates to a nuclear reset of both:
  - Docker: stops the daemon, wipes its data directory, restarts it.
  - Nix:    macOS:    DRY-RUN ONLY — prints the qcow nuke + launchctl restart plan.
            Linux:    Aggressive GC ceiling — delete old generations, collect-garbage,
                      optimise, wipe ~/.cache/nix. Does NOT rm -rf /nix/store.`,
	RunE: runCleanBuild,
}

func init() {
	cleanBuildCmd.Flags().Bool("force", false, "nuclear cleanup (wipe daemon data dir / nuke linux-builder VM / aggressive nix GC)")
	cleanBuildCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cleanCmd.AddCommand(cleanBuildCmd)
}

func runCleanBuild(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	yes, _ := cmd.Flags().GetBool("yes")

	telemetry.Track("clean_build", map[string]any{"force": force})

	return execCleanBuild(force, yes)
}

func execCleanBuild(force, yes bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	homeDir, _ := os.UserHomeDir()
	verbose := ux.LogPlainText

	// Preflight: query Docker state (can be slow if daemon is busy).
	var preflight *ux.ProgressSpinner
	if !verbose {
		preflight = ux.NewProgressSpinner("Querying Docker state…")
	}
	du := docker.QueryDiskUsage(ctx)
	du.RunningCells = docker.QueryRunningCells(ctx)
	opts := docker.PruneOpts{
		GOOS:      runtime.GOOS,
		Force:     force,
		HomeDir:   homeDir,
		Rootless:  detectRootlessDocker(ctx),
		NixOS:     fileExists("/etc/NIXOS"),
		DiskUsage: du,
	}

	// Nix store report: show hash table before the confirmation prompt
	// so the user sees what's on the volume before deciding to prune.
	if preflight != nil {
		preflight.UpdateText("Querying nix store…")
	}
	nixReport, nixOK := docker.CollectNixStore(ctx)

	// Detect stale containers before pruning
	if preflight != nil {
		preflight.UpdateText("Checking for stale containers…")
	}
	stale, staleErr := docker.FindStaleCells(ctx, "")
	if staleErr == nil && len(stale) > 0 {
		if preflight != nil {
			preflight.Stop()
			preflight = nil
		}
		isTTY := isatty.IsTerminal(os.Stdin.Fd())
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
		fmt.Printf("\n%d stale container(s) with no active session:\n", len(stale))
		ux.PrintTable(headers, rows)

		shouldStop := yes
		if !yes && isTTY {
			fmt.Printf("\nStop stale containers before pruning? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			shouldStop = answer == "y" || answer == "Y"
		}
		if shouldStop {
			errs := docker.ReapStaleCells(ctx, stale)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "  error: %v\n", e)
				}
			} else {
				fmt.Printf("Stopped %d stale container(s).\n", len(stale))
			}
		}
	}

	if !force {
		if preflight != nil {
			preflight.UpdateText("Resolving live nix closures…")
		}
		closures, failures, err := docker.CollectLiveClosures(
			func() ([]string, error) { return docker.DockerRunningDevcellContainers(ctx) },
			func(container, link string) (string, error) {
				return docker.DockerResolveContainerLink(ctx, container, link)
			},
			ux.Debugf,
		)
		if err != nil {
			if preflight != nil {
				preflight.Fail("Preflight failed")
			}
			return err
		}
		if len(failures) > 0 {
			if preflight != nil {
				preflight.Stop()
				preflight = nil
			}
			isTTY := isatty.IsTerminal(os.Stdin.Fd())
			if !docker.ConfirmDestructive(os.Stdout, os.Stdin, yes, isTTY, docker.BuildStopBrokenPrompt(failures)) {
				return fmt.Errorf("aborted: %d container(s) with unresolvable closures", len(failures))
			}
			for _, f := range failures {
				docker.DockerStopContainer(ctx, f.Container)
			}
		}
		opts.LiveClosures = closures
	}

	if preflight != nil {
		preflight.Success("Docker state collected")
	}

	if nixOK {
		docker.FormatNixStoreSection(nixReport, os.Stdout)
	}

	var sp *ux.ProgressSpinner
	return docker.RunPrune(docker.RunPruneArgs{
		Opts:    opts,
		Exec:    func(step docker.PruneStep) error { return execStep(ctx, step, verbose) },
		Out:     os.Stdout,
		In:      os.Stdin,
		SkipYes: yes,
		IsTTY:   isatty.IsTerminal(os.Stdin.Fd()),
		Verbose: verbose,
		OnExecStart: func() {
			if !verbose {
				sp = ux.NewProgressSpinner("Pruning…")
			}
		},
		OnStepStart: func(label string) {
			if sp != nil {
				sp.UpdateText(label)
			}
		},
		OnExecDone: func(err error) {
			if sp == nil {
				return
			}
			if err != nil {
				sp.Fail("Prune failed")
			} else {
				sp.Success("Pruned Docker and nix")
			}
		},
	})
}

func execStep(ctx context.Context, step docker.PruneStep, verbose bool) error {
	if step.Func != nil {
		return step.Func(ctx)
	}
	if len(step.Argv) == 0 {
		return nil
	}
	c := exec.CommandContext(ctx, step.Argv[0], step.Argv[1:]...)
	if verbose {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}
	return c.Run()
}

func detectRootlessDocker(ctx context.Context) bool {
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx, "docker", "info", "--format", "{{.SecurityOptions}}").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "rootless")
}

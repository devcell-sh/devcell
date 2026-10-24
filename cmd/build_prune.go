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

	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// `devcell build prune` — standalone cleanup subcommand. See CELL-101.
//
// Default mode: safe prune of both Docker daemon (stopped containers, images,
// volumes, BuildKit cache) and the nix store (orphaned profiles, unreferenced
// store paths). Running containers and their nix closures are always protected.
//
// `--force` escalates to a nuclear reset of both Docker and nix.
//
// `--pure` is deprecated: it used to select nix-only mode but the default now
// covers both. Passing --pure still works (runs the combined prune) but warns.
var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune Docker daemon and nix store. --force for nuclear.",
	Long: `Prune the local Docker daemon and nix store.

By default, runs a safe prune of both targets:
  - Docker: removes stopped containers, unused images, volumes, BuildKit cache.
  - Nix:    removes orphaned profile generations and unreferenced store paths.
            Running containers and their nix closures are always protected.

--force escalates to a nuclear reset of both:
  - Docker: stops the daemon, wipes its data directory, restarts it.
  - Nix:    macOS:    DRY-RUN ONLY — prints the qcow nuke + launchctl restart plan.
            Linux:    Aggressive GC ceiling — delete old generations, collect-garbage,
                      optimise, wipe ~/.cache/nix. Does NOT rm -rf /nix/store.

--pure is deprecated: the default now covers both Docker and nix.

A "This will delete ALL …" prompt confirms every destructive op. Pass -y to skip.
Non-interactive stdin without -y refuses (prevents accidental pipelined destruction).`,
	RunE: runBuildPrune,
}

func init() {
	pruneCmd.Flags().Bool("pure", false, "deprecated: the default now prunes both Docker and nix")
	_ = pruneCmd.Flags().MarkDeprecated("pure", "the default now prunes both Docker and nix")
	pruneCmd.Flags().Bool("force", false, "nuclear cleanup (wipe daemon data dir / nuke linux-builder VM / aggressive nix GC)")
	pruneCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	buildCmd.AddCommand(pruneCmd)
}

func runBuildPrune(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	yes, _ := cmd.Flags().GetBool("yes")

	telemetry.Track("build_prune", map[string]any{"force": force})

	homeDir, _ := os.UserHomeDir()
	opts := docker.PruneOpts{
		GOOS:    runtime.GOOS,
		Force:   force,
		HomeDir: homeDir,
		// Auto-detect rootless Docker and NixOS at runtime.
		Rootless: detectRootlessDocker(),
		NixOS:    fileExists("/etc/NIXOS"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Preflight gate: resolve every RUNNING cell's closure so the nix
	// prune plan can stamp its GC roots before the sweep. Containers whose
	// closure cannot be resolved are offered for force-stop.
	if !force {
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
		opts.LiveClosures = closures
	}

	return docker.RunPrune(docker.RunPruneArgs{
		Opts:    opts,
		Exec:    func(step docker.PruneStep) error { return execStep(ctx, step) },
		Out:     os.Stdout,
		In:      os.Stdin,
		SkipYes: yes,
		IsTTY:   isatty.IsTerminal(os.Stdin.Fd()),
	})
}

func execStep(ctx context.Context, step docker.PruneStep) error {
	if step.Func != nil {
		return step.Func(ctx)
	}
	if len(step.Argv) == 0 {
		return nil
	}
	c := exec.CommandContext(ctx, step.Argv[0], step.Argv[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func detectRootlessDocker() bool {
	out, err := exec.Command("docker", "info", "--format", "{{.SecurityOptions}}").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "rootless")
}

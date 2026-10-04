package main

import (
	"context"
	"fmt"
	"os"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/telemetry"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build (or rebuild) the local devcell image",
	RunE:  runBuild,
}

func init() {
	buildCmd.Flags().Bool("update", false, "update nix flake inputs and rebuild without cache")
	buildCmd.Flags().String("stack", "", "override [cell].stack for this build (base, dev, go, node, python, fullstack, electronics, ultimate, bbb)")
	buildCmd.Flags().String("image", "", "override the built image tag (e.g. devcell-user:dev-thin); env DEVCELL_BUILD_IMAGE has lower precedence")
	buildCmd.Flags().Bool("force", false, "recreate VM even if it already exists (tart only)")
	buildCmd.Flags().Bool("no-cache", false, "re-download OCI image, bypassing tart cache (tart only)")
	buildCmd.Flags().String("stage", "full", `build stage: "base" (infra only) or "full" (default, includes stack activation) (tart only)`)
	buildCmd.Flags().Int("build-threads", 0, "nix download threads (max-substitution-jobs + http-connections); default 128; env DEVCELL_BUILD_THREADS")
}

func runBuild(cmd *cobra.Command, _ []string) error {
	applyOutputFlagsWithLog("build")

	c, err := config.LoadFromOS()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cellCfgForEngine := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
	engineName, err := resolveEngine(os.Stderr, cellCfgForEngine)
	if err != nil {
		return err
	}

	telemetry.Track("build", map[string]any{
		"engine":     string(engineName),
		"subcommand": "build",
		"update":     scanFlag("--update"),
		"no_cache":   scanFlag("--no-cache"),
		"force":      scanFlag("--force"),
		"stage":      cmd.Flags().Lookup("stage").Value.String(),
	})

	cellCfg, err := cfg.LoadFromOSWithDirs(c.ConfigDir, c.BaseDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	e, err := engine.For(engineName)
	if err != nil {
		return err
	}
	return e.Build(context.Background(), buildOpts(cmd, engineCell(c, cellCfg)))
}

// buildOpts is the engine.BuildOpts for `cell build` on cell c: every build
// flag, whichever engines read it.
func buildOpts(cmd *cobra.Command, c engine.Cell) engine.BuildOpts {
	flags := cmd.Flags()
	update, _ := flags.GetBool("update")
	force, _ := flags.GetBool("force")
	noCache, _ := flags.GetBool("no-cache")
	threads, _ := flags.GetInt("build-threads")
	return engine.BuildOpts{
		Cell:         c,
		Stack:        flags.Lookup("stack").Value.String(),
		Update:       update,
		DryRun:       scanFlag("--dry-run"),
		Force:        force,
		NoCache:      noCache,
		Stage:        flags.Lookup("stage").Value.String(),
		Image:        flags.Lookup("image").Value.String(),
		BuildThreads: threads,
	}
}

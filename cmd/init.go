package main

import (
	"context"
	"fmt"
	"os"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/telemetry"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .devcell.toml and .devcell/ build context in current directory",
	RunE:  runInit,
	Args:  cobra.NoArgs,
}

func init() {
	initCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompts and proceed with defaults")
	initCmd.Flags().Bool("force", false, "Overwrite existing files and update flake inputs (implies --update)")
	initCmd.Flags().Bool("update", false, "update nix flake inputs (pull latest) instead of just resolving")
	initCmd.Flags().Bool("upgrade", false, "rewrite deprecated keys in devcell.toml and .devcell.toml to the current syntax, then exit (same as `cell config migrate`)")
	initCmd.Flags().Bool("no-cache", false, "Force re-download of cached IPSW restore image (tart only)")
	initCmd.Flags().String("stack", "", "stack name (base, dev [seed], ultimate [general development], bbb [specialist tools]; legacy: go, node, python, fullstack, electronics)")
	initCmd.Flags().StringSlice("modules", nil, "explicit module list (comma-separated, e.g. go,infra,electronics)")
}

func runInit(cmd *cobra.Command, _ []string) error {
	applyOutputFlagsWithLog("init")

	// --upgrade is a config rewrite, not a re-scaffold: migrate the files
	// and stop before any engine or flake work.
	if upgrade, _ := cmd.Flags().GetBool("upgrade"); upgrade {
		telemetry.Track("init_upgrade", nil)
		c, err := config.LoadFromOS()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		return migrateConfigFiles(c, false)
	}

	// Engine resolution uses the same priority as build/run:
	// CLI flag > TOML [cell].engine > "docker" default.
	// init may run before any TOML exists, so LoadFromOS silently
	// returns zero-value config when there's no file yet.
	c, err := config.LoadFromOS()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
	engineName, err := resolveEngine(os.Stderr, cellCfg)
	if err != nil {
		return err
	}
	telemetry.Track("init", map[string]any{"engine": string(engineName), "stack": cmd.Flags().Lookup("stack").Value.String()})

	if bi, _ := cmd.Flags().GetString("base-image"); bi != "" {
		os.Setenv("DEVCELL_BASE_IMAGE", bi)
		ux.Debugf("DEVCELL_BASE_IMAGE: %s (--base-image flag)", bi)
	} else if bi := os.Getenv("DEVCELL_BASE_IMAGE"); bi != "" {
		ux.Debugf("DEVCELL_BASE_IMAGE: %s (env)", bi)
	}

	e, err := engine.For(engineName)
	if err != nil {
		return err
	}
	return e.Init(context.Background(), initOpts(cmd, engineCell(c, cellCfg)))
}

// initOpts is the engine.InitOpts for `cell init` on cell c: every init
// flag, whichever engines read it.
func initOpts(cmd *cobra.Command, c engine.Cell) engine.InitOpts {
	flags := cmd.Flags()
	stack, _ := flags.GetString("stack")
	modules, _ := flags.GetStringSlice("modules")
	yes, _ := flags.GetBool("yes")
	force, _ := flags.GetBool("force")
	update, _ := flags.GetBool("update")
	return engine.InitOpts{
		Cell:    c,
		Stack:   stack,
		Force:   force,
		Modules: modules,
		Yes:     yes,
		Update:  update,
	}
}

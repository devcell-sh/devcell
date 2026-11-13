package main

import (
	"encoding/json"
	"os"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/experiments"
	"github.com/devcell-sh/devcell/internal/scan"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/tui"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Interactive TUI dashboard for disk, cells, and health",
	Long: `Full-screen interactive dashboard showing disk usage breakdown,
nix store closure analysis, running cell status, and system health.

Requires the ui-doctor experimental feature.`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().String("format", "text", "output format: text (TUI) or json")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()

	cwd := scan.MustCwd()
	cellCfg := cfg.LoadFromOS("", cwd)
	exp, err := experimental(cellCfg.Cell)
	if err != nil {
		return err
	}
	if err := experiments.Require(exp, experiments.Doctor, "cell doctor"); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString("format")
	telemetry.Track("doctor", map[string]any{"format": format})

	if format == "json" {
		return runDoctorJSON()
	}

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	stdinTTY := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	if !isTTY || !stdinTTY {
		ux.Warn("Not a terminal. Use --format=json for non-interactive output.")
		return runDoctorJSON()
	}

	return tui.RunDoctor()
}

func runDoctorJSON() error {
	out := map[string]any{
		"status": "ok",
		"note":   "full scan data will be populated in Phase 2+",
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

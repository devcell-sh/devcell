package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devcell-sh/devcell/internal/storage"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var cleanSolitaryCmd = &cobra.Command{
	Use:   "solitary",
	Short: "Remove solitary project clones (.devcell/solitary/)",
	Long: `Remove isolated directories created by solitary mode.

Each --solitary run creates timestamped project clones, empty homes, and
mount directories under .devcell/solitary/. This command removes all of
them for the current project directory.`,
	RunE: runCleanSolitary,
}

func init() {
	cleanSolitaryCmd.Flags().Bool("dry-run", false, "show clones without removing them")
	cleanSolitaryCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cleanCmd.AddCommand(cleanSolitaryCmd)
}

func runCleanSolitary(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")

	telemetry.Track("clean_solitary", map[string]any{"dry_run": dryRun})

	cwd := mustCwd()
	solitaryDir := filepath.Join(cwd, ".devcell", "solitary")

	entries, err := os.ReadDir(solitaryDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No solitary clones found.")
			return nil
		}
		return fmt.Errorf("read %s: %w", solitaryDir, err)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(solitaryDir, e.Name()))
		}
	}

	if len(dirs) == 0 {
		fmt.Println("No solitary clones found.")
		return nil
	}

	fmt.Printf("\n Solitary clones in %s:\n", solitaryDir)
	var totalBytes int64
	for _, d := range dirs {
		sz := dirSize(d)
		totalBytes += sz
		fmt.Printf("   %-40s %s\n", filepath.Base(d), ux.HumanBytes(sz))
	}
	fmt.Printf("\n Total: %d clone(s), %s\n", len(dirs), ux.HumanBytes(totalBytes))

	if dryRun {
		fmt.Println("\nDry run, not removing.")
		return nil
	}

	isTTY := isatty.IsTerminal(os.Stdin.Fd())
	if !yes && isTTY {
		fmt.Printf("\nRemove %d solitary clone(s), %s? [y/N] ", len(dirs), ux.HumanBytes(totalBytes))
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	} else if !yes && !isTTY {
		return fmt.Errorf("refusing to clean without --yes in non-interactive mode")
	}

	cleaned := 0
	for _, d := range dirs {
		if err := storage.ForceRemoveAll(d); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing %s: %v\n", filepath.Base(d), err)
		} else {
			cleaned++
		}
	}

	fmt.Printf("Removed %d of %d solitary clone(s).\n", cleaned, len(dirs))
	return nil
}

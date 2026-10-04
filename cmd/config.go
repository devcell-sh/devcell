package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/telemetry"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and migrate devcell config files",
}

var configMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Rewrite deprecated keys in devcell.toml and .devcell.toml to the current syntax",
	Long: `Rewrites every deprecated key in the global config (~/.config/devcell/devcell.toml)
and the project config (.devcell.toml) to its current spelling, in place.
Comments, ordering and unrelated lines are kept. A changed file is first
copied to <file>.bak-<timestamp>.

The rewritten config is loaded and checked before it is written; a file that
would not load, or that would still warn, is left untouched and reported.

Use --dry-run to see the changes without writing anything.`,
	RunE: runConfigMigrate,
}

func init() {
	configMigrateCmd.Flags().BoolP("dry-run", "n", false, "show what would change, write nothing")
	configCmd.AddCommand(configMigrateCmd)
}

func runConfigMigrate(cmd *cobra.Command, args []string) error {
	applyOutputFlags()
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	telemetry.Track("config_migrate", map[string]any{"dry_run": dryRun})

	c, err := config.LoadFromOS()
	if err != nil {
		return err
	}
	return migrateConfigFiles(c, dryRun)
}

// migrateConfigFiles rewrites the project and global config files to the
// current syntax and prints one row per file. Shared by `cell config
// migrate` and `cell init --upgrade`. The project file goes first: it is
// the one being worked on, and inside a cell the global file is often a
// read-only bind mount, which is reported as a skip with a hint rather
// than a failure. A file that fails for another reason is reported and
// left untouched; the joined errors are returned at the end so the other
// file still gets its turn.
func migrateConfigFiles(c config.Config, dryRun bool) error {
	paths := []string{
		filepath.Join(c.BaseDir, ".devcell.toml"),
		filepath.Join(c.ConfigDir, "devcell.toml"),
	}
	var failed []error
	for _, path := range paths {
		if _, statErr := os.Stat(path); statErr != nil {
			continue
		}
		res, backup, err := cfg.MigrateFile(path, !dryRun)
		// Full path on purpose: these rows report a write to one of two
		// files that share a base name.
		name := path
		var link *cfg.ReadOnlySymlinkError
		switch {
		case errors.As(err, &link):
			// A symlink into /nix/store is read-only on the host too: the
			// file is generated, so the fix goes in whatever generates it.
			// The full result goes next to the link so there is a complete
			// file to copy, not just a list of changes.
			printChanges(res)
			source := "its source"
			if link.HomeManaged() {
				source = "the home-manager module that writes it, then `home-manager switch`"
			}
			ux.Warn(fmt.Sprintf("%s — symlink to read-only %s; %s, apply it in %s", name, link.Target, migratedCopy(path, res, dryRun), source))
			continue
		case errors.Is(err, cfg.ErrReadOnly):
			printChanges(res)
			ux.Warn(fmt.Sprintf("%s — read-only here, %s, or run `cell init --upgrade` on the host to migrate it", name, migratedCopy(path, res, dryRun)))
			continue
		}
		if err != nil {
			ux.FailMsg(fmt.Sprintf("%s — %v", name, err))
			failed = append(failed, err)
			continue
		}
		if len(res.Changes) == 0 {
			// A copy left by an earlier run would be mistaken for current.
			_ = os.Remove(path + ".migrated")
			ux.SuccessMsg(name + " — up to date")
			continue
		}
		printChanges(res)
		switch {
		case dryRun:
			ux.Info(fmt.Sprintf("%s — %s (dry run, nothing written)", name, plural(len(res.Changes), "change")))
		default:
			ux.SuccessMsg(fmt.Sprintf("%s — %s, backup %s", name, plural(len(res.Changes), "change"), filepath.Base(backup)))
		}
	}
	return errors.Join(failed...)
}

// migratedCopy writes the migrated text to "<path>.migrated" for a file
// that cannot be rewritten in place and returns the phrase for the row.
// When the directory is not writable either, the text is printed instead.
func migratedCopy(path string, res cfg.MigrationResult, dryRun bool) string {
	if dryRun {
		return "dry run"
	}
	out := path + ".migrated"
	if err := os.WriteFile(out, []byte(res.After), 0o644); err != nil {
		fmt.Println("        migrated config:")
		fmt.Print(res.After)
		return "migrated config printed above"
	}
	return "full result written to " + out
}

func printChanges(res cfg.MigrationResult) {
	for _, ch := range res.Changes {
		fmt.Printf("        %s: %s\n", ch.Key, ch.Action)
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

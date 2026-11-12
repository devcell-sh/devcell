package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/storage"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var cleanCacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Remove old and new cache directories",
	Long: `Find and remove cache directories managed by devcell.

Shows a list of everything found and asks for confirmation before removing.

Old per-workspace caches (from before unified storage):
  go/pkg/mod          old GOMODCACHE
  .cache/go-build     old GOCACHE
  .npm                old npm cache
  .cache/uv           old uv cache
  winkit/mackit       legacy per-cell locations (migrated, not deleted)

New unified cache:
  ~/.devcell/cache/   shared and private cache trees

Use --dry-run to preview. Use --all to scan every workspace.`,
	RunE: runCleanCache,
}

func init() {
	cleanCacheCmd.Flags().Bool("dry-run", false, "show caches without removing them")
	cleanCacheCmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cleanCacheCmd.Flags().Bool("all", false, "scan every workspace, not just the current one")
	cleanCacheCmd.Flags().String("workspace", "", "target a specific workspace (default: current)")
	cleanCmd.AddCommand(cleanCacheCmd)
}

func runCleanCache(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	all, _ := cmd.Flags().GetBool("all")
	wsFlag, _ := cmd.Flags().GetString("workspace")

	telemetry.Track("clean_cache", map[string]any{"dry_run": dryRun, "all": all})

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("user home: %w", err)
	}
	devcellHome := filepath.Join(home, ".devcell")

	workspaces, err := resolveWorkspaces(devcellHome, wsFlag, all)
	if err != nil {
		return err
	}

	// Collect old per-workspace caches
	var oldCaches []string
	for _, ws := range workspaces {
		wsHome := filepath.Join(devcellHome, ws)
		oldCaches = append(oldCaches, storage.FindOldWorkspaceCaches(wsHome)...)
	}

	// Old winkit/mackit
	oldWinkit, _ := storage.FindOldWinkitCaches(devcellHome)
	oldMackit, hasMackit := storage.FindOldMackitCache(home)

	// Old unified cache at ~/.devcell/cache (before XDG move)
	oldUnified := filepath.Join(devcellHome, "cache")
	hasOldUnified := dirNonEmpty(oldUnified)

	// New unified cache at ~/.cache/devcell
	unifiedCache := filepath.Join(home, ".cache", "devcell")
	hasUnified := dirNonEmpty(unifiedCache)

	total := len(oldCaches) + len(oldWinkit)
	if hasMackit {
		total++
	}
	if hasOldUnified {
		total++
	}
	if hasUnified {
		total++
	}

	if total == 0 {
		fmt.Println("No caches found.")
		return nil
	}

	var allDirs []string
	for _, d := range oldCaches {
		allDirs = append(allDirs, d)
	}
	allDirs = append(allDirs, oldWinkit...)
	if hasMackit {
		allDirs = append(allDirs, oldMackit)
	}
	if hasOldUnified {
		allDirs = append(allDirs, oldUnified)
	}
	if hasUnified {
		allDirs = append(allDirs, unifiedCache)
	}

	fmt.Println()
	sp := ux.NewProgressSpinner(fmt.Sprintf("Calculating %d cache sizes…", len(allDirs)))
	sizes := make(map[string]int64, len(allDirs))
	for i, d := range allDirs {
		if ux.LogPlainText {
			fmt.Printf("  [%d/%d] %s\n", i+1, len(allDirs), d)
		}
		sizes[d] = dirSize(d)
	}
	sp.Stop()

	var totalBytes int64
	if len(oldCaches) > 0 {
		fmt.Println("\n Old workspace caches:")
		for _, d := range oldCaches {
			rel, _ := filepath.Rel(devcellHome, d)
			sz := sizes[d]
			totalBytes += sz
			fmt.Printf("   %-30s %s\n", rel, ux.HumanBytes(sz))
		}
	}
	if len(oldWinkit) > 0 {
		fmt.Println("\n Old winkit caches (will be migrated):")
		for _, d := range oldWinkit {
			rel, _ := filepath.Rel(devcellHome, d)
			sz := sizes[d]
			totalBytes += sz
			fmt.Printf("   %-30s %s\n", rel, ux.HumanBytes(sz))
		}
	}
	if hasMackit {
		sz := sizes[oldMackit]
		totalBytes += sz
		fmt.Printf("\n Old mackit cache (will be migrated):\n   %-30s %s\n", oldMackit, ux.HumanBytes(sz))
	}
	if hasOldUnified {
		sz := sizes[oldUnified]
		totalBytes += sz
		fmt.Printf("\n Old unified cache (pre-XDG):\n   %-30s %s\n", oldUnified, ux.HumanBytes(sz))
	}
	if hasUnified {
		sz := sizes[unifiedCache]
		totalBytes += sz
		fmt.Printf("\n Unified cache:\n   %-30s %s\n", unifiedCache, ux.HumanBytes(sz))
	}

	fmt.Printf("\n Total: %s\n", ux.HumanBytes(totalBytes))

	if dryRun {
		fmt.Printf("\n%d cache location(s) found (dry run, not removing).\n", total)
		return nil
	}

	isTTY := isatty.IsTerminal(os.Stdin.Fd())
	if !yes && isTTY {
		fmt.Printf("\nClean %d cache location(s), %s? [y/N] ", total, ux.HumanBytes(totalBytes))
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

	// Remove old workspace caches
	if len(oldCaches) > 0 {
		if cleanErr := storage.CleanOldWorkspaceCaches(oldCaches); cleanErr != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", cleanErr)
		} else {
			cleaned += len(oldCaches)
		}
	}

	// Winkit/mackit: migrate to new unified location unless both caches
	// are being wiped anyway.
	wipingAll := hasOldUnified && hasUnified
	if !wipingAll {
		if len(oldWinkit) > 0 {
			winkitDst := filepath.Join(unifiedCache, "v1", "shared", "winkit")
			for _, old := range oldWinkit {
				if migrateErr := storage.MigrateWinkitCache(old, winkitDst); migrateErr != nil {
					ux.Debugf("winkit migration: %v", migrateErr)
				} else {
					cleaned++
				}
			}
		}
		if hasMackit {
			mackitDst := storage.MackitPath(unifiedCache)
			if migrateErr := storage.MigrateMackitCache(oldMackit, mackitDst); migrateErr != nil {
				ux.Debugf("mackit migration: %v", migrateErr)
			} else {
				cleaned++
			}
		}
	} else {
		for _, old := range oldWinkit {
			if err := storage.ForceRemoveAll(old); err == nil {
				cleaned++
			}
		}
		if hasMackit {
			if err := storage.ForceRemoveAll(oldMackit); err == nil {
				cleaned++
			}
		}
	}

	// Remove old unified cache (~/.devcell/cache)
	if hasOldUnified {
		if err := storage.ForceRemoveAll(oldUnified); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing old unified cache: %v\n", err)
		} else {
			cleaned++
		}
	}

	// Remove new unified cache (~/.cache/devcell)
	if hasUnified {
		if err := storage.ForceRemoveAll(unifiedCache); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing unified cache: %v\n", err)
		} else {
			cleaned++
		}
	}

	fmt.Printf("Cleaned %d of %d cache location(s).\n", cleaned, total)
	return nil
}

func resolveWorkspaces(devcellHome, wsFlag string, all bool) ([]string, error) {
	if wsFlag != "" {
		info, err := os.Stat(filepath.Join(devcellHome, wsFlag))
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("workspace %q not found in %s", wsFlag, devcellHome)
		}
		return []string{wsFlag}, nil
	}
	if all {
		entries, err := os.ReadDir(devcellHome)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", devcellHome, err)
		}
		var ws []string
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "cache" {
				continue
			}
			ws = append(ws, e.Name())
		}
		return ws, nil
	}
	c := config.Load(mustCwd(), os.Getenv)
	return []string{c.WorkspaceName}, nil
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func dirNonEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func dirSize(path string) int64 {
	var total int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

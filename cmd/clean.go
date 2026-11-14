package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/storage"
	"github.com/devcell-sh/devcell/internal/telemetry"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean old caches, Docker images, solitary clones, and nix store (safe defaults)",
	Long: `Scan all cleanable resources, show a combined breakdown, and ask
what to clean. Scans: old per-workspace caches, solitary project clones,
Docker daemon (containers, images, volumes, build cache), and nix store.

Use --quiet to skip the prompt and clean everything.
Use --dry-run to see what would be cleaned without removing anything.
Use subcommands (clean cache, clean build, etc.) for finer control.`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolP("yes", "y", false, "skip confirmation prompts")
	cleanCmd.Flags().BoolP("quiet", "q", false, "clean everything without interactive selection")
	cleanCmd.PersistentFlags().Bool("dry-run", false, "show what would be cleaned without removing anything")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()
	yes, _ := cmd.Flags().GetBool("yes")
	quiet, _ := cmd.Flags().GetBool("quiet")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose := ux.LogPlainText

	telemetry.Track("clean", map[string]any{"dry_run": dryRun, "quiet": quiet})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Phase 1: detect and calculate ─────────────────────────────────
	sp := newScanSpinner(verbose, "Scanning caches…")
	cacheResult := scanCaches(true)
	finishScanStep(sp, verbose, "Scanned caches", cacheResult.Count(), cacheResult.TotalBytes)

	sp = newScanSpinner(verbose, "Scanning solitary clones…")
	solResult := scanSolitary()
	finishScanStep(sp, verbose, "Scanned solitary clones", len(solResult.Dirs), solResult.TotalBytes)

	sp = newScanSpinner(verbose, "Querying Docker…")
	dockerResult := scanDocker(ctx)
	if sp != nil {
		sp.Success("Queried Docker")
	}

	sp = newScanSpinner(verbose, "Querying nix store…")
	nixResult := scanNix(ctx)
	if sp != nil {
		sp.Success("Queried nix store")
	}

	buildResult := mergeBuildScan(dockerResult, nixResult)

	// ── Phase 2: present breakdown ────────────────────────────────────
	hasCaches := cacheResult.Count() > 0
	hasSolitary := len(solResult.Dirs) > 0
	hasDocker := buildResult.DiskUsage.Images.Size != ""
	hasNixInfo := buildResult.NixOK && buildResult.NixReport.Health.ProfileHashes > 0
	hasStale := len(buildResult.Stale) > 0

	if !hasCaches && !hasSolitary && !hasDocker && !hasNixInfo {
		fmt.Println("Nothing to clean.")
		return nil
	}

	if hasCaches {
		presentCaches(cacheResult)
	}

	if hasSolitary {
		fmt.Printf("\n Solitary clones:\n")
		for _, d := range solResult.Dirs {
			fmt.Printf("   %-40s %s\n", filepath.Base(d), ux.HumanBytes(solResult.Sizes[d]))
		}
	}

	if hasStale {
		homes := hostHomes()
		fmt.Printf("\n Stale containers (%d with no active session):\n", len(buildResult.Stale))
		headers := []string{"CELL", "WORKSPACE", "UPTIME", "PROJECT"}
		rows := make([][]string, 0, len(buildResult.Stale))
		for _, s := range buildResult.Stale {
			rows = append(rows, []string{
				s.Name, s.Workspace, s.Uptime,
				docker.ShortenPath(s.Project, homes...),
			})
		}
		ux.PrintTable(headers, rows)
	}

	if hasDocker {
		du := buildResult.DiskUsage
		fmt.Println("\n Docker:")
		if du.Images.Size != "" {
			fmt.Printf("   Images:      %-10s reclaimable: %s\n", du.Images.Size, du.Images.Reclaimable)
		}
		if du.Containers.Size != "" {
			fmt.Printf("   Containers:  %-10s reclaimable: %s\n", du.Containers.Size, du.Containers.Reclaimable)
		}
		if du.Volumes.Size != "" {
			fmt.Printf("   Volumes:     %-10s reclaimable: %s\n", du.Volumes.Size, du.Volumes.Reclaimable)
		}
		if du.BuildCache.Size != "" {
			fmt.Printf("   Build cache: %-10s reclaimable: %s\n", du.BuildCache.Size, du.BuildCache.Reclaimable)
		}
	}

	if hasNixInfo {
		presentNixStore(buildResult)
	}

	totalBytes := cacheResult.TotalBytes + solResult.TotalBytes
	if hasDocker {
		du := buildResult.DiskUsage
		for _, r := range []string{du.Images.Reclaimable, du.Containers.Reclaimable, du.Volumes.Reclaimable, du.BuildCache.Reclaimable} {
			s := strings.SplitN(r, " ", 2)[0]
			if n, err := docker.ParseSize(s); err == nil && n > 0 {
				totalBytes += n
			}
		}
	}
	totalBytes += nixReclaimableBytes(buildResult)
	fmt.Printf("\n Total reclaimable: ~%s\n", ux.HumanBytes(totalBytes))

	if dryRun {
		fmt.Println("\nDry run, not removing.")
		return nil
	}

	// ── Phase 3: confirm and execute ──────────────────────────────────
	isTTY := isatty.IsTerminal(os.Stdin.Fd())
	if !yes && !quiet && isTTY {
		fmt.Printf("\nClean all of the above? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	} else if !yes && !quiet && !isTTY {
		return fmt.Errorf("refusing to clean without --yes in non-interactive mode")
	}

	if hasCaches {
		execCacheClean(cacheResult)
	}
	if hasSolitary {
		execSolitaryClean(solResult)
	}
	if hasStale {
		errs := docker.ReapStaleCells(ctx, buildResult.Stale)
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  error: %v\n", e)
			}
		} else {
			fmt.Printf("Stopped %d stale container(s).\n", len(buildResult.Stale))
		}
	}

	if hasDocker || hasNixInfo {
		if err := execBuildClean(ctx, buildResult, verbose, isTTY); err != nil {
			fmt.Fprintf(os.Stderr, "  build: %v\n", err)
		}
	}

	return nil
}

// ── Scan phase helpers ────────────────────────────────────────────────

func newScanSpinner(verbose bool, text string) *ux.ProgressSpinner {
	if verbose {
		return nil
	}
	return ux.NewProgressSpinner(text)
}

func finishScanStep(sp *ux.ProgressSpinner, verbose bool, label string, count int, bytes int64) {
	if count == 0 {
		if sp != nil {
			sp.Success(label + ": none found")
		}
		return
	}
	msg := fmt.Sprintf("%s: %d item(s), %s", label, count, ux.HumanBytes(bytes))
	if sp != nil {
		sp.Success(msg)
	}
}

// ── Present helpers ──────────────────────────────────────────────────

func nixReclaimableCount(scan buildScanResult) int {
	if !scan.NixOK {
		return 0
	}
	liveSet := make(map[string]bool)
	for _, lh := range docker.LiveHashes(scan.LiveClosures) {
		liveSet[lh] = true
	}
	count := 0
	for _, m := range scan.NixReport.Metas {
		if !liveSet[m.Hash] {
			count++
		}
	}
	count += scan.NixReport.Health.StaleRoots
	count += scan.NixReport.Health.OrphanedGenerations
	return count
}

func presentNixStore(scan buildScanResult) {
	h := scan.NixReport.Health
	metas := scan.NixReport.Metas

	liveSet := make(map[string]bool)
	for _, lh := range docker.LiveHashes(scan.LiveClosures) {
		liveSet[lh] = true
	}

	var live, dead []docker.NixRootMeta
	for _, m := range metas {
		if liveSet[m.Hash] {
			live = append(live, m)
		} else {
			dead = append(dead, m)
		}
	}

	volSize := ""
	if scan.NixReport.VolumeBytes > 0 {
		volSize = " (" + ux.HumanBytes(scan.NixReport.VolumeBytes) + " on disk)"
	}
	fmt.Printf("\n Nix store%s: %d profile hashes, %d generations on volume\n",
		volSize, h.ProfileHashes, h.Generations)

	if len(live) > 0 {
		fmt.Printf("   %d protected (used by running containers):\n", len(live))
		writeNixMetaTable(live)
	}
	if len(dead) > 0 {
		fmt.Printf("   %d reclaimable (roots will be reaped, then garbage collected):\n", len(dead))
		writeNixMetaTable(dead)
	}
	if h.OrphanedGenerations > 0 {
		fmt.Printf("   %d orphaned generation(s) (will be swept)\n", h.OrphanedGenerations)
	}
	if h.StaleRoots > 0 {
		fmt.Printf("   %d stale root(s) (dangling symlinks)\n", h.StaleRoots)
	}
	if len(dead) == 0 && h.OrphanedGenerations == 0 && h.StaleRoots == 0 {
		fmt.Println("   all clean (GC will reclaim unreferenced store paths)")
	}
	if len(scan.NixReport.TopPaths) > 0 {
		fmt.Println("   Top store paths:")
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, p := range scan.NixReport.TopPaths {
			fmt.Fprintf(tw, "     %s\t%s\n", nixStorePathName(p.Name), ux.HumanBytes(p.Bytes))
		}
		tw.Flush()
	}
}

func writeNixMetaTable(metas []docker.NixRootMeta) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, m := range metas {
		cell := m.Cell
		if cell == "" {
			cell = "-"
		}
		stack := m.Stack
		if stack == "" {
			stack = "-"
		}
		if m.Modules != "" {
			stack += "+" + m.Modules
		}
		size := "-"
		if m.ClosureBytes > 0 {
			size = ux.HumanBytes(m.ClosureBytes)
		}
		fmt.Fprintf(w, "     %s\t%s\t%s\t%s\n", shortHash(m.Hash), cell, stack, size)
	}
	w.Flush()
}

func nixReclaimableBytes(scan buildScanResult) int64 {
	if !scan.NixOK {
		return 0
	}
	liveSet := make(map[string]bool)
	for _, lh := range docker.LiveHashes(scan.LiveClosures) {
		liveSet[lh] = true
	}
	var total int64
	for _, m := range scan.NixReport.Metas {
		if !liveSet[m.Hash] {
			total += m.ClosureBytes
		}
	}
	return total
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// nixStorePathName strips the 32-char nix hash prefix from a store path basename.
// "abc123...-gcc-14.1.0" → "gcc-14.1.0"
func nixStorePathName(name string) string {
	if len(name) > 33 && name[32] == '-' {
		return name[33:]
	}
	return name
}

func presentCaches(r cacheScanResult) {
	if len(r.OldCaches) > 0 {
		fmt.Println("\n Old workspace caches:")
		for _, d := range r.OldCaches {
			rel, _ := filepath.Rel(r.DevcellHome, d)
			fmt.Printf("   %-30s %s\n", rel, ux.HumanBytes(r.Sizes[d]))
		}
	}
	if len(r.OldWinkit) > 0 {
		fmt.Println("\n Old winkit caches (will be migrated):")
		for _, d := range r.OldWinkit {
			rel, _ := filepath.Rel(r.DevcellHome, d)
			fmt.Printf("   %-30s %s\n", rel, ux.HumanBytes(r.Sizes[d]))
		}
	}
	if r.HasMackit {
		fmt.Printf("\n Old mackit cache (will be migrated):\n   %-30s %s\n",
			r.OldMackit, ux.HumanBytes(r.Sizes[r.OldMackit]))
	}
	if r.HasOldUnif {
		fmt.Printf("\n Old unified cache (pre-XDG):\n   %-30s %s\n",
			r.OldUnified, ux.HumanBytes(r.Sizes[r.OldUnified]))
	}
	if r.HasUnified {
		fmt.Printf("\n Unified cache:\n   %-30s %s\n",
			r.UnifiedCache, ux.HumanBytes(r.Sizes[r.UnifiedCache]))
	}
}

// ── Execute helpers ──────────────────────────────────────────────────

func execCacheClean(r cacheScanResult) {
	cleaned := 0
	if len(r.OldCaches) > 0 {
		if err := storage.CleanOldWorkspaceCaches(r.OldCaches); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		} else {
			cleaned += len(r.OldCaches)
		}
	}

	wipingAll := r.HasOldUnif && r.HasUnified
	if !wipingAll {
		if len(r.OldWinkit) > 0 {
			winkitDst := filepath.Join(r.UnifiedCache, "v1", "shared", "winkit")
			for _, old := range r.OldWinkit {
				if err := storage.MigrateWinkitCache(old, winkitDst); err != nil {
					ux.Debugf("winkit migration: %v", err)
				} else {
					cleaned++
				}
			}
		}
		if r.HasMackit {
			mackitDst := storage.MackitPath(r.UnifiedCache)
			if err := storage.MigrateMackitCache(r.OldMackit, mackitDst); err != nil {
				ux.Debugf("mackit migration: %v", err)
			} else {
				cleaned++
			}
		}
	} else {
		for _, old := range r.OldWinkit {
			if err := storage.ForceRemoveAll(old); err == nil {
				cleaned++
			}
		}
		if r.HasMackit {
			if err := storage.ForceRemoveAll(r.OldMackit); err == nil {
				cleaned++
			}
		}
	}

	if r.HasOldUnif {
		if err := storage.ForceRemoveAll(r.OldUnified); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing old unified cache: %v\n", err)
		} else {
			cleaned++
		}
	}
	if r.HasUnified {
		if err := storage.ForceRemoveAll(r.UnifiedCache); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing unified cache: %v\n", err)
		} else {
			cleaned++
		}
	}
	fmt.Printf("Cleaned %d of %d cache location(s).\n", cleaned, r.Count())
}

func execSolitaryClean(r solitaryScanResult) {
	cleaned := 0
	for _, d := range r.Dirs {
		if err := storage.ForceRemoveAll(d); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing %s: %v\n", filepath.Base(d), err)
		} else {
			cleaned++
		}
	}
	fmt.Printf("Removed %d of %d solitary clone(s).\n", cleaned, len(r.Dirs))
}

func execBuildClean(ctx context.Context, scan buildScanResult, verbose, isTTY bool) error {
	homeDir, _ := os.UserHomeDir()
	closures := scan.LiveClosures

	if scan.ClosureErr != nil {
		return scan.ClosureErr
	}
	if len(scan.ClosureFailures) > 0 {
		if !docker.ConfirmDestructive(os.Stdout, os.Stdin, true, isTTY, docker.BuildStopBrokenPrompt(scan.ClosureFailures)) {
			return fmt.Errorf("skipping prune: %d container(s) with unresolvable closures", len(scan.ClosureFailures))
		}
		for _, f := range scan.ClosureFailures {
			docker.DockerStopContainer(ctx, f.Container)
		}
	}

	// Reap dead GC roots first so the subsequent GC can reclaim their closures.
	// Skip when the scan found nothing reclaimable (all hashes are live).
	if nixReclaimableCount(scan) > 0 {
		for _, step := range docker.BuildCleanupSteps(closures) {
			if err := execStep(ctx, step, verbose); err != nil {
				ux.Debugf("reap: %v", err)
			}
		}
	}

	opts := docker.PruneOpts{
		GOOS:          runtime.GOOS,
		HomeDir:       homeDir,
		Rootless:      detectRootlessDocker(ctx),
		NixOS:         fileExists("/etc/NIXOS"),
		DiskUsage:     scan.DiskUsage,
		LiveClosures:  closures,
		SkipReapSweep: nixReclaimableCount(scan) == 0,
	}

	var pruneSp *ux.ProgressSpinner
	return docker.RunPrune(docker.RunPruneArgs{
		Opts:    opts,
		Exec:    func(step docker.PruneStep) error { return execStep(ctx, step, verbose) },
		Out:     os.Stdout,
		In:      os.Stdin,
		SkipYes: true,
		IsTTY:   isTTY,
		Verbose: verbose,
		OnExecStart: func() {
			if !verbose {
				pruneSp = ux.NewProgressSpinner("Pruning Docker and nix…")
			}
		},
		OnStepStart: func(label string) {
			if pruneSp != nil {
				pruneSp.UpdateText(label)
			}
		},
		OnExecDone: func(err error) {
			if pruneSp == nil {
				return
			}
			if err != nil {
				pruneSp.Fail("Prune failed")
			} else {
				pruneSp.Success("Pruned Docker and nix")
			}
		},
	})
}

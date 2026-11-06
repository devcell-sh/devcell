package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/mattn/go-isatty"
)

// CELL-390: "Nix store" preflight phase — read-only health report on the
// shared nix volume at every cell start. Non-fatal by design: any probe
// failure degrades to a "skipped" detail, never blocks boot.
//
// Hygiene (default-on, --no-hygiene to disable): removes dangling GC root
// symlinks and orphaned metadata, wipes the ephemeral registry blob cache.
// Runs only when the probe detects stale roots, so clean volumes skip the
// extra container. autoCleanup (--auto-cleanup) is the more aggressive
// CELL-334 reaper (running-only retention).
//
// Every datapoint and decision is mirrored to ux.Debugf so `--debug` shows
// the full check: volume, argv, raw probe output, parsed counts, cleanup
// closures, and reap results.
func nixStorePhase(ctx context.Context, pr ux.Phaser, thin bool, baseDir, hostHome string, staleWarn, autoCleanup, noHygiene bool) error {
	if !thin {
		ux.Debugf("nix health: skipped (not thin mode — no store volume)")
		return nil
	}
	if v := os.Getenv("DEVCELL_NIX_HEALTH_CHECK"); v == "false" || v == "0" {
		ux.Debugf("nix health: skipped (DEVCELL_NIX_HEALTH_CHECK=%s)", v)
		return nil
	}

	var health NixStoreHealth
	probed := false
	_ = pr.PhaseDetailedWarn("Nix store", func() (string, bool, error) {
		volume := ThinStoreVolume()
		argv := NixHealthProbeArgv(volume)
		ux.Debugf("nix health: probing volume=%s", volume)
		ux.Debugf("nix health: argv=%s", DebugArgv(argv))

		out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
		ux.Debugf("nix health: raw output=%q err=%v", strings.TrimSpace(string(out)), err)
		if err != nil {
			return "skipped (probe unavailable)", false, nil
		}
		h, perr := ParseNixStoreHealth(string(out))
		if perr != nil {
			ux.Debugf("nix health: parse failed: %v", perr)
			return "skipped (unreadable probe output)", false, nil
		}
		ux.Debugf("nix health: parsed total=%d stale=%d hashes=%d generations=%d orphaned=%d revs=%d newest_rev=%s newest_projects=%d",
			h.TotalRoots, h.StaleRoots, h.ProfileHashes, h.Generations, h.OrphanedGenerations,
			h.DistinctRevs, h.NewestRev, h.NewestProjects)
		health, probed = h, true

		tag, detail, severity := h.Summary()
		ux.Debugf("nix health: tag=%s detail=%s severity=%d", tag, detail, severity)
		rowDetail := tag + " — " + detail
		if severity != NixStoreOK && h.Generations > 0 {
			extra := ", " + plural(h.Generations, "generation")
			budget := ux.PhaseRowBudget(len("Nix store"))
			if len(rowDetail)+len(extra) <= budget {
				rowDetail += extra
			}
		}
		if !noHygiene {
			rowDetail += "; " + hygieneDetail(ctx, health, hostHome)
		}
		if autoCleanup {
			rowDetail += "; " + autoCleanupDetail(ctx)
		}
		if severity == NixStoreError {
			return "", false, fmt.Errorf("%s", rowDetail)
		}
		return rowDetail, severity == NixStoreWarn, nil
	})

	return staleCellNudge(baseDir, health, probed, staleWarn)
}

// staleCellNudge implements CELL-391: if this project's nixpkgs rev is
// behind the newest rev live on the volume, warn that starting keeps a
// parallel closure alive and offer to abort (default: proceed). Inform,
// not enforce — every failure path is silence, and non-TTY never blocks.
func staleCellNudge(baseDir string, h NixStoreHealth, probed, staleWarn bool) error {
	if !probed {
		return nil
	}
	if !staleWarn {
		ux.Debugf("stale check: skipped ([cell] stale_warning = false)")
		return nil
	}
	if v := os.Getenv("DEVCELL_STALE_WARN"); v == "false" || v == "0" {
		ux.Debugf("stale check: skipped (DEVCELL_STALE_WARN=%s)", v)
		return nil
	}
	projectRev, _ := ProjectNixpkgsRev(baseDir)
	ux.Debugf("stale check: project rev=%q volume newest=%q distinct revs=%d",
		projectRev, h.NewestRev, h.DistinctRevs)
	warning, stale := StaleCellWarning(projectRev, h)
	if !stale {
		ux.Debugf("stale check: cell is current (or revs unknown) — no nudge")
		return nil
	}
	isTTY := isatty.IsTerminal(os.Stdin.Fd())
	ux.Debugf("stale check: nudging user (tty=%v, default=proceed)", isTTY)
	if ConfirmProceed(os.Stdout, os.Stdin, isTTY, warning) {
		ux.Debugf("stale check: user chose to proceed with the stale closure")
		return nil
	}
	ux.Debugf("stale check: user aborted launch")
	return fmt.Errorf("launch aborted — update this cell with: cell build --update")
}

// CELL-418: detect whether the thin image's baked-in nix closure still
// exists on the shared nix-store volume. A dead closure means the image
// will boot silently broken — no nix-daemon, no shell setup, no tools.
// Prompts for rebuild on TTY; auto-rebuilds on non-TTY.
//
// Skipped in dry-run mode: image acquisition already skips there, so the
// image may not exist at all, and probing a missing image looks exactly
// like a dead closure. Acting on it would turn a dry run into an image
// build.
func closureCheckPhase(ctx context.Context, pr ux.Phaser, thin bool, imageTag string, dryRun bool, rebuild func() error) error {
	if !thin {
		ux.Debugf("closure check: skipped (not thin mode)")
		return nil
	}
	if dryRun {
		ux.Debugf("closure check: skipped (dry-run)")
		return nil
	}
	volume := ThinStoreVolume()
	ux.Debugf("closure check: volume=%s image=%s", volume, imageTag)

	argv := ClosureAliveArgv(volume, imageTag)
	ux.Debugf("closure check: argv=%s", DebugArgv(argv))

	out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
	resolved, alive := ParseClosureAliveResult(string(out), err)
	ux.Debugf("closure check: alive=%v resolved=%q err=%v", alive, resolved, err)

	warning, dead := ClosureDeadWarning(alive)
	if !dead {
		ux.Debugf("closure check: closure alive at %s", resolved)
		return nil
	}

	isTTY := isatty.IsTerminal(os.Stdin.Fd())
	ux.Debugf("closure check: dead closure detected (tty=%v)", isTTY)

	if !ConfirmProceed(os.Stdout, os.Stdin, isTTY, warning) {
		return fmt.Errorf("launch aborted — rebuild with: cell build")
	}

	return rebuild()
}

// hygieneDetail runs the zero-risk startup hygiene: stale GC root removal,
// orphaned metadata removal, and registry blob cache wipe. Failures degrade
// to debug output; a cell start must never break because hygiene did.
func hygieneDetail(ctx context.Context, health NixStoreHealth, hostHome string) string {
	if v := os.Getenv("DEVCELL_HYGIENE"); v == "false" || v == "0" {
		ux.Debugf("hygiene: skipped (DEVCELL_HYGIENE=%s)", v)
		return "hygiene skipped"
	}

	var parts []string

	if health.StaleRoots > 0 {
		volume := ThinStoreVolume()
		argv := HygieneNixArgv(volume)
		ux.Debugf("hygiene: running nix cleanup (stale=%d)", health.StaleRoots)
		ux.Debugf("hygiene: argv=%s", DebugArgv(argv))

		out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
		ux.Debugf("hygiene: output=%q err=%v", strings.TrimSpace(string(out)), err)
		if err != nil {
			ux.Debugf("hygiene: nix cleanup failed: %v", err)
		} else {
			stale, orphaned := ParseHygieneResult(string(out))
			if stale > 0 {
				parts = append(parts, plural(stale, "stale root")+" removed")
			}
			if orphaned > 0 {
				parts = append(parts, plural(orphaned, "orphaned meta")+" removed")
			}
		}
	}

	registryDir := filepath.Join(hostHome, ".devcell", "registry")
	if info, err := os.Stat(registryDir); err == nil && info.IsDir() {
		ux.Debugf("hygiene: wiping registry cache %s", registryDir)
		if err := os.RemoveAll(registryDir); err != nil {
			ux.Debugf("hygiene: registry wipe failed: %v", err)
		} else {
			parts = append(parts, "registry cleared")
		}
	}

	if len(parts) == 0 {
		return "hygiene clean"
	}
	return strings.Join(parts, ", ")
}

// autoCleanupDetail runs the CELL-334 reaper without prompting — the
// --auto-cleanup flag IS the consent. Failures degrade to a detail string;
// a cell start must never break because hygiene did.
func autoCleanupDetail(ctx context.Context) string {
	closures, failures, err := CollectLiveClosures(
		func() ([]string, error) { return DockerRunningDevcellContainers(ctx) },
		func(container, link string) (string, error) {
			return DockerResolveContainerLink(ctx, container, link)
		},
		ux.Debugf,
	)
	if err != nil {
		ux.Debugf("auto-cleanup: refused: %v", err)
		return "auto-cleanup skipped (docker ps failed)"
	}
	if len(failures) > 0 {
		for _, f := range failures {
			ux.Debugf("auto-cleanup: skipping unresolvable container %s: %v", f.Container, f.Err)
		}
	}
	for _, step := range BuildCleanupSteps(closures) {
		ux.Debugf("auto-cleanup: exec %s", strings.Join(step.Argv[:6], " ")+" …")
		out, runErr := exec.CommandContext(ctx, step.Argv[0], step.Argv[1:]...).CombinedOutput()
		ux.Debugf("auto-cleanup: output=%q err=%v", strings.TrimSpace(string(out)), runErr)
		if runErr != nil {
			return "auto-cleanup failed (see --debug)"
		}
		// Surface the reaper's own one-line summary as the non-debug UX.
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Cleanup:") {
				return strings.ToLower(strings.TrimPrefix(line, "Cleanup: "))
			}
		}
	}
	return "auto-cleanup ran"
}

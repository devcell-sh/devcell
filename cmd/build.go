package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/runner"
	"github.com/DimmKirr/devcell/internal/scaffold"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/version"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build (or rebuild) the local devcell image",
	RunE:  runBuild,
}

func init() {
	buildCmd.Flags().Bool("update", false, "update nix flake inputs and rebuild without cache")
	buildCmd.Flags().Bool("no-generate", false, "skip regenerating build context (flake.nix, Dockerfile, etc.)")
	// Post-2026-05-15 flip (DIMM-204): pure is the default. --impure (DIMM-213
	// canonical name) opts into the legacy Dockerfile path. --debian is the
	// deprecated alias retained for one release. --pure is a silent no-op
	// (same as default).
	buildCmd.Flags().Bool("impure", false, "build via legacy Dockerfile path (default is nix2container/pure)")
	buildCmd.Flags().Bool("debian", false, "deprecated alias for --impure (will be removed)")
	buildCmd.Flags().Bool("pure", false, "silent no-op (kept for back-compat; pure is the default after DIMM-204)")
	// DIMM-246: one-shot stack override. Precedence: --stack > $DEVCELL_STACK >
	// [cell].stack in TOML > default (ResolvedStack → "base").
	buildCmd.Flags().String("stack", "", "override [cell].stack for this build (base, go, node, python, fullstack, electronics, ultimate)")
}

func runBuild(cmd *cobra.Command, _ []string) error {
	applyOutputFlags()
	update, _ := cmd.Flags().GetBool("update")
	noGenerate, _ := cmd.Flags().GetBool("no-generate")
	impure, _ := cmd.Flags().GetBool("impure")
	// Back-compat: accept --debian as an alias for --impure.
	if !impure {
		debian, _ := cmd.Flags().GetBool("debian")
		impure = debian
	}
	// Allow --impure / --debian via the positional scanner too so
	// `cell claude --build --impure` (or --debian) works.
	if !impure {
		impure = scanFlag("--impure") || scanFlag("--debian")
	}

	c, err := config.LoadFromOS()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ── Default: pure (nix2container) engine — strict, no docker-build fallback ──
	if !impure {
		stackOverride, err := resolveStackOverride(cmd.Flags().Lookup("stack").Value.String(), os.Getenv)
		if err != nil {
			return err
		}
		return runBuildPure(c, stackOverride)
	}

	// ── Vagrant engine ────────────────────────────────────────────────────────
	// cell build --engine=vagrant   → vagrant provision (re-applies nixhome flake)
	// cell build --update --engine=vagrant → nix flake update inside VM, then provision
	engine := scanStringFlag("--engine")
	if scanFlag("--macos") {
		engine = "vagrant"
	}
	if engine == "vagrant" {
		cellCfgVagrant := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
		vagrantBox := scanStringFlag("--vagrant-box")
		if vagrantBox == "" {
			vagrantBox = "utm/bookworm"
		}
		vagrantProvider := scanStringFlag("--vagrant-provider")
		if vagrantProvider == "" {
			vagrantProvider = "utm"
		}
		// Scaffold Vagrantfile idempotently (same as runVagrantAgent step 1).
		nixhomeDir := resolveVagrantNixhome(c.BaseDir)
		if nixhomeDir == "" {
			nixhomeDir = c.BaseDir + "/nixhome"
		}
		vmConfigDir := os.Getenv("DEVCELL_CONFIG_DIR")
		if vmConfigDir == "" {
			vmConfigDir = c.HostHome + "/.config/devcell"
		}
		// Always regenerate Vagrantfile on build (ports, stack may have changed).
		os.Remove(c.BuildDir + "/Vagrantfile")
		if err := scaffold.ScaffoldLinuxVagrantfile(
			c.BuildDir, vagrantBox, vagrantProvider,
			cellCfgVagrant.Cell.ResolvedStack(),
			c.BaseDir, nixhomeDir,
			c.VNCPort, c.RDPPort,
			c.HostHome, vmConfigDir,
		); err != nil {
			fmt.Fprintf(os.Stderr, "warning: vagrantfile scaffold failed: %v\n", err)
		}
		return runVagrantBuild(c.BuildDir, c.BaseDir, cellCfgVagrant, update, scanFlag("--dry-run"))
	}

	// ── Docker engine (default) ───────────────────────────────────────────────
	if err := config.EnsureBuildDir(c.BuildDir); err != nil {
		return fmt.Errorf("ensure build dir: %w", err)
	}

	cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
	ux.Debugf("BuildDir: %s", c.BuildDir)
	if cellCfg.Cell.NixhomePath != "" {
		ux.Debugf("NixhomePath: %s (from config/env)", cellCfg.Cell.NixhomePath)
	}

	// Sync local nixhome into build context when nixhome path is set.
	if nixhomePath := cellCfg.Cell.NixhomePath; nixhomePath != "" {
		ux.Debugf("Syncing nixhome: %s → %s/nixhome/", nixhomePath, c.BuildDir)
		if err := scaffold.SyncNixhome(nixhomePath, c.BuildDir); err != nil {
			return fmt.Errorf("sync nixhome: %w", err)
		}
	}

	if !noGenerate {
		// Regenerate all build artifacts from merged config (flake.nix,
		// Dockerfile, package.json, pyproject.toml) so that stack/modules
		// changes in devcell.toml take effect without re-running cell init.
		if err := scaffold.RegenerateBuildContext(c.BuildDir, cellCfg); err != nil {
			return fmt.Errorf("regenerate build context: %w", err)
		}
	}

	if update {
		if err := updateFlakeLockWithSpinner(c.BuildDir, false, "Updating nix flake inputs"); err != nil {
			return err
		}
	}

	if err := buildImageWithSpinner(c.BuildDir, update, "Building devcell image", false); err != nil {
		return err
	}
	return nil
}

// runBuildPure runs the strict nix2container path. No docker-build fallback.
//
// Nixhome resolution mirrors the docker path (scaffold.go:130-140):
//  1. [cell].nixhome (TOML / DEVCELL_NIXHOME_PATH) — synced into BuildDir.
//  2. <BaseDir>/nixhome on disk             — synced into BuildDir.
//  3. github:DimmKirr/devcell/<ver>?dir=nixhome — passed to nix directly,
//     no local sync. Nix fetches and caches under /nix/store.
//
// The flake ref is then composed by PureBuildArgv as
// "<ref>#packages.<arch>.devcell-<stack>-pure-image" and loaded into the
// local Docker daemon as runner.UserImageTagPure().
// resolveStackOverride collapses the --stack flag value and the DEVCELL_STACK
// env var into a single override string for runBuildPure. Precedence:
// flag > env > "" (empty → caller uses TOML / default).
//
// Empty flagValue means the user didn't pass --stack; in that case the env
// var is consulted. Returns an error if either value names an unknown stack.
// getenv is injected so tests can drive the env layer deterministically
// (avoids polluting the real process env).
func resolveStackOverride(flagValue string, getenv func(string) string) (string, error) {
	if flagValue != "" {
		if err := cfg.ValidateStack(flagValue); err != nil {
			return "", err
		}
		return flagValue, nil
	}
	if v := getenv("DEVCELL_STACK"); v != "" {
		if err := cfg.ValidateStack(v); err != nil {
			return "", err
		}
		return v, nil
	}
	return "", nil
}

// runBuildPure runs the nix2container build. stackOverride wins over the
// TOML-resolved stack when non-empty (DIMM-246). Validation of the override
// happens at the caller (runBuild) so this function can stay focused on the
// build itself.
func runBuildPure(c config.Config, stackOverride string) error {
	cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
	stack := cellCfg.Cell.ResolvedStack()
	if stackOverride != "" {
		stack = stackOverride
	}

	if err := config.EnsureBuildDir(c.BuildDir); err != nil {
		return fmt.Errorf("ensure build dir: %w", err)
	}

	resolved := runner.ResolvePureNixhomeRef(runner.PureNixhomeInputs{
		TomlNixhome: cellCfg.Cell.NixhomePath,
		BaseDir:     c.BaseDir,
		Version:     version.Version,
	})

	// Local source: sync into BuildDir so the flake path is stable across
	// runs and the user can inspect/edit .devcell/nixhome/ for debugging.
	// Remote source: skip the sync — nix handles the fetch and cache.
	flakeRef := resolved.FlakeRef
	if !resolved.Remote {
		if err := scaffold.SyncNixhome(resolved.LocalPath, c.BuildDir); err != nil {
			return fmt.Errorf("sync nixhome: %w", err)
		}
		flakeRef = "path:" + c.BuildDir + "/nixhome"
		ux.Debugf("Pure build using local nixhome: %s (synced from %s)", flakeRef, resolved.LocalPath)
	} else {
		ux.Debugf("Pure build using remote nixhome: %s", flakeRef)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var buf bytes.Buffer
	var out io.Writer = &buf
	if ux.Verbose {
		out = os.Stdout
	}
	// Wrap with a layer counter so the --debug summary below can report
	// new (transferred) vs cached (already-at-destination) blob counts
	// captured straight from skopeo's per-blob log lines.
	lc := runner.NewLayerCounter(out)
	out = lc

	label := fmt.Sprintf("Building devcell image (nix2container, stack=%s)", stack)
	sp := ux.NewProgressSpinner(label)
	err := runner.BuildImagePure(ctx, runner.PureBuildSpec{
		FlakeRef:  flakeRef,
		StackName: stack,
		// Anchor the out-link inside BuildDir so we don't drop result-*
		// symlinks in the user's cwd (especially relevant for the github:
		// fallback where there's no local nixhome dir to anchor next to).
		OutLink: filepath.Join(c.BuildDir, fmt.Sprintf("result-%s-pure", stack)),
	}, runner.UserImageTagPure(), ux.Verbose, out)
	if err != nil {
		sp.Fail(label + " failed")
		if !ux.Verbose && buf.Len() > 0 {
			fmt.Fprint(os.Stderr, buf.String())
		}
		return err
	}
	// Append the loaded image size to the spinner's success line.
	// skopeo's per-blob progress is empty when stdout isn't a TTY, so this
	// synthesizes a single summary number from `docker image inspect` —
	// always available regardless of skopeo's terminal heuristics.
	tag := runner.UserImageTagPure()
	successLabel := label
	if size := runner.LocalImageSize(ctx, tag); size > 0 {
		successLabel = fmt.Sprintf("%s — %s loaded", label, runner.HumanBytes(size))
	}
	sp.Success(successLabel)

	// --debug summary — answers "is this a fresh build, and how much was cached?"
	// Only fires under --debug (ux.Debugf is a no-op otherwise).
	if ux.Verbose {
		printBuildDebugSummary(ctx, tag, lc.Stats())
	}
	return nil
}

// printBuildDebugSummary prints the post-build debug block: image ID +
// created timestamp + size + total layers + new/cached split. Surfaces the
// "did my rebuild actually produce a new image, or is the cache stale?"
// question that motivated the feature (DIMM-216 debugging session).
func printBuildDebugSummary(ctx context.Context, tag string, layers runner.LayerStats) {
	info, err := runner.InspectImageDebug(ctx, tag)
	if err != nil {
		ux.Debugf("post-build inspect failed: %v", err)
		return
	}
	ux.Debugf("image:        %s", info.Tag)
	ux.Debugf("image ID:     %s", shortID(info.ID))
	ux.Debugf("created:      %s", info.Created)
	ux.Debugf("size:         %s", runner.HumanBytes(info.SizeBytes))
	ux.Debugf("layers total: %d", info.LayerCount)
	// New + Cached may not sum to LayerCount: skopeo's log is only emitted
	// for the registry-push leg, and skipped-line wording varies across
	// versions. We surface raw counts and the leftover as "unaccounted"
	// rather than fabricate a guarantee that doesn't hold.
	ux.Debugf("layers new:   %d", layers.New)
	ux.Debugf("layers cached:%d", layers.Cached)
	if rest := info.LayerCount - layers.New - layers.Cached; rest > 0 {
		ux.Debugf("layers other: %d  (not classified by skopeo log)", rest)
	}
}

// shortID renders sha256:abcdef… as abcdef12 to match `docker images`.
func shortID(id string) string {
	const prefix = "sha256:"
	s := id
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		s = s[len(prefix):]
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

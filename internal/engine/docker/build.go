package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/nixhome"
	"github.com/DimmKirr/devcell/internal/scaffold"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/version"
)

// buildParams is a thin-image build resolved from engine.BuildOpts.
type buildParams struct {
	stack          string // --stack, else DEVCELL_STACK, else Cell.Stack
	explicitStack  bool   // the stack was overridden or set in [cell] stack; the progress label names it
	image          string // tag to build: --image, else DEVCELL_BUILD_IMAGE, else "" for UserImageTagThin
	recreateVolume bool   // --update: remove the nix-store volume first so the build repopulates it
	threads        int    // --build-threads, exported as DEVCELL_BUILD_THREADS; 0 leaves it alone
}

// newBuildParams resolves opts for build. getenv is os.Getenv outside tests.
func newBuildParams(opts engine.BuildOpts, getenv func(string) string) (buildParams, error) {
	override, err := resolveStackOverride(opts.Stack, getenv)
	if err != nil {
		return buildParams{}, err
	}
	image := opts.Image
	if image == "" {
		image = getenv("DEVCELL_BUILD_IMAGE")
	}
	return buildParams{
		stack:          opts.Cell.WithStack(override).Stack,
		explicitStack:  override != "" || opts.Cell.Config.Cell.StackExplicit(),
		image:          image,
		recreateVolume: opts.Update,
		threads:        opts.BuildThreads,
	}, nil
}

// autoBuildParams is the build Run starts itself when the image is missing
// or its nix closure is gone: the cell's stack, the default tag, and the
// nix-store volume kept.
func autoBuildParams(c engine.Cell) buildParams {
	return buildParams{stack: c.Stack, explicitStack: c.Config.Cell.StackExplicit()}
}

// resolveStackOverride collapses the --stack flag value and the DEVCELL_STACK
// env var into a single override string. Precedence:
// flag > env > "" (empty = caller uses TOML / default).
//
// getenv is injected so tests can drive the env layer deterministically.
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

// build builds a thin image (CELL-156):
//  1. Ensure core image exists (pull or use cached)
//  2. docker run core with nix volume + docker socket:
//     - home-manager switch (reuses volume-cached /nix/store)
//     - docker build (inside container, via socket) → thin image
func build(ctx context.Context, c engine.Cell, p buildParams) error {
	// Daemon preflight — surface the actionable error when docker is down
	// before any pull/build attempt (CELL-44). Run probes before its
	// auto-build too; this guards direct `cell build` callers.
	if err := DockerDaemonReachable(ctx); err != nil {
		return err
	}
	logDiagnostics(ctx, hostConfig(c))

	cellCfg := c.Config
	stack := p.stack

	if err := config.EnsureBuildDir(c.BuildDir); err != nil {
		return fmt.Errorf("ensure build dir: %w", err)
	}
	nixhomeSrc := nixhome.ResolveNixhomeRef(version.Version)
	if err := scaffold.SyncNixhome(nixhomeSrc, c.BuildDir); err != nil {
		return fmt.Errorf("sync nixhome: %w", err)
	}

	// Validate nix packages before generating the flake.
	if err := cfg.ValidateNixPackages(cellCfg.Packages.Nix); err != nil {
		return err
	}
	_, overrides := cfg.ResolveNixChannels(cellCfg.Packages.Nix)
	for _, o := range overrides {
		ux.Debugf("nix: %s", o)
	}

	// Write the overlay flake at .devcell/flake.nix — same generator as pure
	// path. Imports path:./nixhome (the just-synced upstream) + enables the
	// merged TOML modules. home-manager will switch against this overlay's
	// `devcell-local<arch>` output, not the upstream stack outputs directly,
	// so [cell].modules takes effect in thin builds (CELL-38 + CELL-61).
	overlayFlake := scaffold.GenerateFlakeNixWithMcp(stack, cellCfg.Cell.Modules, version.Version, true, cellCfg.Mcp.Enabled, cellCfg.Packages.Nix)
	overlayPath := filepath.Join(c.BuildDir, "flake.nix")
	if err := os.WriteFile(overlayPath, []byte(overlayFlake), 0o644); err != nil {
		return fmt.Errorf("write overlay flake: %w", err)
	}
	// Symlink entrypoint.sh up from synced nixhome so ThinBuildArgv's
	// `cp /opt/nixhome/entrypoint.sh` keeps working when mounting the overlay
	// dir at /opt/nixhome instead of the raw nixhome.
	entrypointLink := filepath.Join(c.BuildDir, "entrypoint.sh")
	_ = os.Remove(entrypointLink)
	if err := os.Symlink(filepath.Join("nixhome", "entrypoint.sh"), entrypointLink); err != nil {
		return fmt.Errorf("symlink entrypoint.sh: %w", err)
	}

	// ── Platform compatibility preflight ──────────────────────────────────
	{
		nixhomeFlake := "path:" + filepath.Join(c.BuildDir, "nixhome")
		targetSystem := DetectArch() + "-linux"
		preLabel := fmt.Sprintf("Platform compatibility check (%s)", targetSystem)
		sp := ux.NewProgressSpinner(preLabel)
		if err := nixhome.PreflightPlatformCheck(ctx, nixhomeFlake, targetSystem); err != nil {
			sp.Fail(preLabel)
			return err
		}
		sp.Success(preLabel)
	}

	// ── Package summary ───────────────────────────────────────────────────
	// Evaluated from the overlay flake so [cell] packages and modules are
	// included. Informational: an eval failure warns and the build goes on.
	// The counts also ride on the image as the devcell.packages label.
	var packageCounts string
	{
		overlayFlake := "path:" + c.BuildDir
		homeConfig := "devcell-local"
		if DetectArch() == "aarch64" {
			homeConfig += "-aarch64"
		}
		pkgLabel := "Resolving packages"
		sp := ux.NewProgressSpinner(pkgLabel)
		defs, hmDecl, err := nixhome.EvalPackageDefinitions(ctx, overlayFlake, homeConfig)
		switch {
		case err != nil:
			sp.Warn(pkgLabel + " skipped")
			ux.Debugf("package summary: %v", err)
		case defs == nil:
			sp.Stop()
		default:
			counts := nixhome.GroupPackageDefinitions(defs, hmDecl)
			packageCounts = nixhome.FormatModuleCounts(counts)
			sp.Success(nixhome.FormatPackageSummary(counts))
		}
	}

	// What we hand ThinBuildArgv is the OVERLAY dir (.devcell), mounted at
	// /opt/nixhome inside the builder. home-manager target becomes
	// `devcell-local` (matches GenerateFlakeNix's homeConfigurations output).
	nixhomeRef := DockerHostPath(c.BuildDir)
	homeManagerTarget := "local"

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	coreImage := cellCfg.Nix.ResolvedImage()
	tag := ResolveBuildTag(p.image, UserImageTagThin())
	volumeName := ThinStoreVolume()
	containerName := ThinBuilderContainerName(c.AppName)

	// ── Ensure core image exists for target platform ───────────────────────
	targetPlatform := DockerPlatform(DetectArch())
	if !ImageExistsForPlatform(ctx, coreImage, targetPlatform) {
		pullLabel := fmt.Sprintf("Pulling core image %s (%s)", coreImage, targetPlatform)
		sp := ux.NewProgressSpinner(pullLabel)
		if err := PullImageForPlatform(ctx, coreImage, targetPlatform, ux.Verbose); err != nil {
			sp.Fail(pullLabel + " failed")
			return fmt.Errorf("pull core image: %w", err)
		}
		sp.Success(pullLabel)
	}

	// ── Volume management ──────────────────────────────────────────────────
	// Docker auto-populates named volumes from the image on first mount when
	// the volume is empty. Don't pre-create — let docker run create it
	// implicitly so auto-populate fires correctly.
	if p.recreateVolume {
		_ = exec.CommandContext(ctx, "docker", "volume", "rm", "-f", volumeName).Run()
	}

	// ── Build thin image ────────────────────────────────────────────────────
	buildLabel := BuildLabel("Building thin image", stack, p.explicitStack)
	sp := ux.NewProgressSpinner(buildLabel)

	// Reclaim this app's builder slot. A builder left behind by a crashed or
	// interrupted run is removed; a *running* one is never killed — builds
	// for other apps have their own name and are untouched either way.
	exists, running := BuilderContainerState(ctx, containerName)
	remove, err := ReclaimBuilderSlot(containerName, exists, running)
	if err != nil {
		sp.Fail(buildLabel + " failed")
		return err
	}
	if remove {
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()
	}

	// CELL-41: pass the real user-facing stack name + modules CSV so the
	// container's metadata.json reports them truthfully. The HM target stays
	// "local" — that's a flake-output naming detail, not user content.
	modulesCSV := strings.Join(cellCfg.Cell.Modules, ",")
	projectName := filepath.Base(c.BaseDir)

	// [build] TOML → env, before argv construction reads them. An explicit
	// env var wins over TOML (env > toml > derived default).
	applyBuildEnv := func(envVar, val string) {
		if val != "" && os.Getenv(envVar) == "" {
			os.Setenv(envVar, val)
		}
	}
	applyBuildEnv("DEVCELL_BUILD_MEMORY", cellCfg.Build.Memory)
	applyBuildEnv("DEVCELL_BUILD_CPUS", cellCfg.Build.CPUs)
	if cellCfg.Build.MaxJobs > 0 {
		applyBuildEnv("DEVCELL_NIX_MAX_JOBS", strconv.Itoa(cellCfg.Build.MaxJobs))
	}
	if cellCfg.Build.Cores > 0 {
		applyBuildEnv("DEVCELL_NIX_CORES", strconv.Itoa(cellCfg.Build.Cores))
	}
	if cellCfg.Build.Threads > 0 {
		applyBuildEnv("DEVCELL_BUILD_THREADS", strconv.Itoa(cellCfg.Build.Threads))
	}

	argv := ThinBuildArgvWithPackages(coreImage, containerName, volumeName, nixhomeRef, tag, homeManagerTarget, DetectArch(), stack, modulesCSV, projectName, packageCounts)

	// Log the resolved build resource config under --debug.
	if lim := ResolveBuildLimits(); lim.Memory != "" || lim.CPUs != "" {
		maxJobs := "auto"
		if lim.MaxJobs > 0 {
			maxJobs = fmt.Sprintf("%d", lim.MaxJobs)
		}
		cores := "default"
		if lim.Cores > 0 {
			cores = fmt.Sprintf("%d", lim.Cores)
		}
		threads := "128"
		if v := os.Getenv("DEVCELL_BUILD_THREADS"); v != "" {
			threads = v
		}
		ux.Debugf("build limits: --memory=%s --cpus=%s nix max-jobs=%s cores=%s threads=%s", lim.Memory, lim.CPUs, maxJobs, cores, threads)
	} else {
		ux.Debugf("build limits: uncapped (daemon too small for a ceiling)")
	}

	// Stream the overlay through Docker stdin. Unlike a bind mount, this is
	// resolved by the local cell process and works when the selected daemon is
	// inside Docker Desktop, Colima, or on a remote host.
	archive, err := os.CreateTemp("", "devcell-thin-nixhome-*.tar")
	if err != nil {
		sp.Fail(buildLabel + " failed")
		return fmt.Errorf("create thin nixhome archive: %w", err)
	}
	archivePath := archive.Name()
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archivePath)
	}()
	if err := WriteThinBuildContext(archive, c.BuildDir); err != nil {
		sp.Fail(buildLabel + " failed")
		return fmt.Errorf("archive thin nixhome: %w", err)
	}
	size, _ := archive.Seek(0, io.SeekCurrent)
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		sp.Fail(buildLabel + " failed")
		return fmt.Errorf("rewind thin nixhome archive: %w", err)
	}
	ux.Debugf("thin nixhome transport: tar-stdin source=%q bytes=%d", c.BuildDir, size)

	var buf bytes.Buffer
	var out io.Writer = &buf
	if ux.Verbose {
		out = os.Stdout
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = archive
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		sp.Fail(buildLabel + " failed")
		if BuilderOrphanedByCancel(err, ctx.Err()) {
			// ctx is already cancelled; use a fresh one so the cleanup runs.
			rmCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = exec.CommandContext(rmCtx, "docker", "rm", "-f", containerName).Run()
			cancel()
		}
		if !ux.Verbose && buf.Len() > 0 {
			fmt.Fprint(os.Stderr, buf.String())
		}
		return fmt.Errorf("thin build: %w", err)
	}

	successLabel := buildLabel
	if size := LocalImageSize(ctx, tag); size > 0 {
		successLabel = fmt.Sprintf("%s — %s", buildLabel, ux.HumanBytes(size))
	}
	sp.Success(successLabel)
	return nil
}

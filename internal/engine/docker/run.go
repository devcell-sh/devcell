package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/DimmKirr/devcell/internal/backup"
	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cellrun"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/op"
	"github.com/DimmKirr/devcell/internal/telemetry"
	"github.com/DimmKirr/devcell/internal/ux"
)

// Run acquires the thin image (building it when it is missing, the nix-store
// volume is unpopulated, or opts.Rebuild is set), runs the boot checklist,
// and runs the agent in a new container with the terminal attached. With
// opts.Detach the container starts in the background and Run returns once
// docker started it; with opts.DryRun it prints the docker run argv instead.
// A non-zero agent exit is *engine.ExitError.
//
// Part of the checklist is engine-neutral but so far runs only here: the
// system prompt, git identity, 1Password secrets, the OpenRouter key, the
// Claude session ID, the cell home backup, the cellrun record and the
// command_finish telemetry event. opts.Force, opts.Update and
// opts.Background are not honored; --force and --stack=<name> in opts.Args
// reach the agent.
func (Engine) Run(ctx context.Context, opts engine.RunOpts) error {
	c := hostConfig(opts.Cell)
	cellCfg := opts.Cell.Config
	binary, defaultFlags, userArgs, extraEnv := opts.Binary, opts.DefaultFlags, opts.Args, opts.Env

	// Set stack/modules so UserImageTag() produces stack-based tags.
	useCell(opts.Cell)

	thin := true
	imageTag := func() string {
		return PickImageTagThin()
	}
	dryRun := opts.DryRun
	explicitBuild := opts.Rebuild

	// Resolve available GUI ports — probe and bump if already bound
	if cellCfg.GUI.ResolvedEnabled() {
		c.ResolveAvailablePorts()
	}

	// ── Image acquisition ────────────────────────────────────────────────────
	// Daemon preflight: surface a single actionable error if docker is down
	// before any pull/build attempt (CELL-44). Skip in dry-run.
	if !dryRun {
		if err := DockerDaemonReachable(ctx); err != nil {
			return err
		}
		logDiagnostics(ctx, c)
	}
	// ── Thin image path (CELL-156) ──────────────────────────────────────────
	if thin {
		needsBuild := false
		reason := ""
		switch {
		case explicitBuild:
			needsBuild = true
		case dryRun:
			// no-op
		case !ImageExists(ctx, imageTag()):
			needsBuild, reason = true, fmt.Sprintf(" No %s image found — building automatically (thin mode)", imageTag())
		case !VolumeHydrated(ThinStoreVolume(), ThinEntrypointSentinel,
			func(v string) bool { return VolumeExists(ctx, v) },
			func(v, p string) bool { return VolumeContains(ctx, v, p) }):
			needsBuild, reason = true, " /nix volume is missing or unpopulated — rebuilding (thin mode, CELL-38)"
		}
		if needsBuild {
			if reason != "" {
				fmt.Println(reason)
			}
			if err := build(ctx, opts.Cell, autoBuildParams(opts.Cell)); err != nil {
				return err
			}
		}
	}

	// Cell-open banner — CELL-48. Always print the compact header so users
	// see "which cell · which project · which pane" at every launch. The cell
	// name is always shown (including the `main` default) — it's a real
	// persistent identity with its own `~/.devcell/<name>/` home, not a
	// placeholder, and surfacing it teaches the cell model.
	project := filepath.Base(c.BaseDir)
	fmt.Println(" " + ux.Banner(c.CellName, project, c.Bunk))

	if ux.Verbose {
		fmt.Println()
		const keyW = 8 // longest key is "Timezone" / "Modules" / "Network"
		// Project / Cell
		fmt.Println("   " + ux.KV(keyW, "Project", project+ux.StyleMuted.Render("  "+c.BaseDir)))
		if c.CellName != "" {
			fmt.Println("   " + ux.KV(keyW, "Cell", c.CellName+ux.StyleMuted.Render("  "+c.CellHome)))
		}
		// Image — current tag + size, more useful than the in-container
		// /etc/devcell/*-image-version strings (which can be missing).
		tag := imageTag()
		imgLine := tag
		if size := LocalImageSize(ctx, tag); size > 0 {
			imgLine += ux.StyleMuted.Render("  " + ux.HumanBytes(size))
		}
		fmt.Println("   " + ux.KV(keyW, "Image", imgLine))
		// Modules source — CELL-48 core ask.
		fmt.Println("   " + ux.KV(keyW, "Modules", cellCfg.Cell.DescribeModulesSource()))
		// Identity / network — surfaces the values bot-detection-relevant
		// settings will resolve to inside the container, so the user can
		// confirm at boot whether MAC / hostname / TZ / locale match the
		// expected persistent identity.
		mac := cellCfg.Cell.MacAddress
		if mac == "" {
			mac = "auto"
		}
		hostname := cellCfg.Cell.ResolvedHostname(c.AppName)
		if envHost := os.Getenv("DEVCELL_HOSTNAME"); envHost != "" {
			hostname = envHost
		}
		fmt.Println("   " + ux.KV(keyW, "Network", "devcell-network"+ux.StyleMuted.Render(" · hostname "+hostname+" · MAC "+mac)))
		// Locale + timezone — combine on one row.
		tz := cellCfg.Cell.Timezone
		if tz == "" {
			if envTZ := os.Getenv("TZ"); envTZ != "" {
				tz = envTZ + " (from host $TZ)"
			} else {
				tz = "(container default)"
			}
		}
		locale := cellCfg.Cell.Locale
		if locale == "" {
			if envLang := os.Getenv("LANG"); envLang != "" && envLang != "POSIX" && envLang != "C" {
				locale = envLang + " (from host $LANG)"
			} else {
				locale = "en_US.UTF-8 (default)"
			}
		}
		fmt.Println("   " + ux.KV(keyW, "Locale", locale))
		fmt.Println("   " + ux.KV(keyW, "Timezone", tz))
		fmt.Println("   " + ux.KV(keyW, "Ports", "VNC localhost:"+c.VNCPort+ux.StyleMuted.Render(" · ")+"RDP localhost:"+c.RDPPort))
		// Boot dir — where the BootDirWatcher polls for in-container sentinels.
		// Useful in --debug: `ls $bootdir/` after a boot shows the chain of
		// fragments that fired (post-mortem). CELL-264.
		fmt.Println("   " + ux.KV(keyW, "Boot", filepath.Join(c.CellHome, "boot")))
		fmt.Println()
	}

	// CELL-262: cell-open phases as a permanent checklist via PhaseRunner.
	// Each row lands as `✓ <name> [— <detail>] <elapsed>` and persists across
	// the docker exec handoff, so the user sees the full boot story above
	// claude's first prompt. Replaces the prior "Opening Cell" spinner +
	// inline stderr warnings + silent successes mix.
	//
	// 7-phase set (Docker daemon and Volume hydrated stay as silent
	// upstream gates — surfacing them as ✓ rows for work that already ran
	// reads as noise). Non-fatal phases discard the returned error with `_ =`;
	// fatal phases propagate via `if err := ...; err != nil { return err }`.
	pr := &ux.PhaseRunner{}

	_ = pr.Phase("Network", func() error { return EnsureNetwork(ctx) })

	if err := pr.Phase("Orphan check", func() error {
		return RemoveOrphanedContainer(ctx, c.ContainerName)
	}); err != nil {
		return err
	}

	// CELL-390: read-only nix-store health report (thin mode only).
	// Non-fatal; mutation only behind the explicit --auto-cleanup opt-in.
	// CELL-391: may nudge when this cell's lock is behind the volume's
	// newest — the only error path is the user explicitly answering "n".
	if err := nixStorePhase(ctx, pr, thin, c.BaseDir, cellCfg.Cell.StaleWarningEnabled(), opts.AutoCleanup); err != nil {
		return err
	}

	// CELL-418: check that the thin image's baked-in nix closure is still
	// alive on the shared volume. A dead closure means GC reaped the store
	// paths — prompt for rebuild (auto-rebuild in non-TTY).
	if err := closureCheckPhase(ctx, pr, thin, imageTag(), func() error {
		return build(ctx, opts.Cell, autoBuildParams(opts.Cell))
	}); err != nil {
		return err
	}

	_ = pr.Phase("Backup", func() error { return backup.Backup(c.CellHome, time.Now()) })

	// Pin the container to the exact image ID so a concurrent `cell build`
	// can't swap the tag under us mid-launch. Falls back to the mutable tag
	// on failure (current behaviour) — kept silent inside the closure so the
	// row stays a ✓ either way.
	var imageID string
	_ = pr.PhaseDetailed("Image pin", func() (string, error) {
		id, idErr := LocalImageIDFor(ctx, imageTag())
		if idErr != nil {
			imageID = imageTag()
			return imageID, nil
		}
		imageID = id
		short := id
		if len(short) > 19 { // "sha256:abcdef012345" = 19 chars
			short = short[:19]
		}
		return short, nil
	})
	// Per-module package counts stamped on the image at build time
	// (devcell.packages label). Older images have no label: no row.
	if pkgs := ImageLabel(ctx, imageID, "devcell.packages"); pkgs != "" {
		_ = pr.PhaseDetailed("Packages", func() (string, error) { return pkgs, nil })
	}
	if ux.Verbose && !dryRun {
		source := DockerHostPath(c.BaseDir)
		probeVolume := ""
		if thin {
			probeVolume = ThinStoreVolume()
		}
		out, probeErr := ProbeDockerBind(
			ctx, imageID, probeVolume, source, ".devcell.toml")
		if probeErr != nil {
			ux.Debugf("docker bind probe: FAILED source=%q marker=.devcell.toml: %v output=%q",
				source, probeErr, out)
		} else {
			ux.Debugf("docker bind probe: OK source=%q %s", source, out)
		}
	}

	// Inject prompts for Claude Code as generated files. The overlay carries
	// container context (mounts, host paths, constraints) plus the append
	// prompt; the base, when configured, replaces Claude Code's built-in
	// prompt entirely. See cell.ResolveSystemPrompt / ResolveAppendPrompt
	// for the source-precedence chains. Fatal: a bad prompt produces a broken
	// claude session, fail loudly here.
	if binary == "claude" {
		rowName, _ := systemPromptRow(ux.Verbose, "")
		if err := pr.PhaseDetailed(rowName, func() (string, error) {
			flags, spErr := cell.ClaudePromptFlags(c, cellCfg, cell.ResolveOpts{
				EnvFile:         os.Getenv("DEVCELL_SYSTEM_PROMPT_FILE"),
				EnvInline:       os.Getenv("DEVCELL_SYSTEM_PROMPT"),
				AppendEnvFile:   os.Getenv("DEVCELL_APPEND_SYSTEM_PROMPT_FILE"),
				AppendEnvInline: os.Getenv("DEVCELL_APPEND_SYSTEM_PROMPT"),
				CellCfg:         cellCfg,
				CfgBaseDir:      c.BaseDir,
			})
			if spErr != nil {
				return "", spErr
			}
			defaultFlags = append(defaultFlags, flags...)
			_, detail := systemPromptRow(ux.Verbose, flags[len(flags)-1])
			return detail, nil
		}); err != nil {
			return fmt.Errorf("system prompt: %w", err)
		}
	}

	if binary == "codex" {
		if err := pr.PhaseDetailed("System prompt", func() (string, error) {
			flags, spErr := cell.CodexPromptFlags(c, cellCfg, cell.ResolveOpts{
				AppendEnvFile:   os.Getenv("DEVCELL_APPEND_SYSTEM_PROMPT_FILE"),
				AppendEnvInline: os.Getenv("DEVCELL_APPEND_SYSTEM_PROMPT"),
				CellCfg:         cellCfg,
				CfgBaseDir:      c.BaseDir,
			})
			if spErr != nil {
				return "", spErr
			}
			defaultFlags = append(defaultFlags, flags...)
			if cellCfg.LLM.SystemPrompt != "" || cellCfg.LLM.SystemPromptFile != "" {
				ux.Warn("[llm].system_prompt is set but Codex has no way to replace its built-in prompt. This setting is ignored for cell codex. Only [llm].append_system_prompt is wired.")
			}
			return "developer_instructions", nil
		}); err != nil {
			return fmt.Errorf("system prompt: %w", err)
		}
	}

	// Git identity falls to the host `git config` tier only when neither a
	// GIT_* env var nor [git] sets it (see cell.ResolveGitIdentity). Surface
	// that lookup as a row, then pin its result so BuildArgv does not run git
	// again. Non-fatal; row is "not configured" when both keys are absent.
	gitConfig := cell.HostGitConfig
	if cell.ResolveGitIdentity(os.Getenv, cellCfg.Git, nil).Source == cell.GitSourceDefault {
		_ = pr.PhaseDetailed("Git identity", func() (string, error) {
			name, email := cell.HostGitConfig("user.name"), cell.HostGitConfig("user.email")
			gitConfig = func(key string) string {
				return map[string]string{"user.name": name, "user.email": email}[key]
			}
			switch {
			case name != "" && email != "":
				return name + " <" + email + ">", nil
			case name != "":
				return name, nil
			case email != "":
				return email, nil
			default:
				return "not configured", nil
			}
		})
	}

	// Loading secrets — CELL-261 phase, now expressed through PhaseRunner.
	// Suppressed entirely when no [secrets.onepassword] documents are configured, or when the
	// user opted out via --no-secrets / --no-1password / DEVCELL_NO_SECRETS / DEVCELL_NO_1PASSWORD.
	var inheritEnv []string
	var resolvedSecrets map[string]string
	opDocs := cellCfg.Op.ResolvedDocuments()
	skipSecrets := opts.NoSecrets
	noSecretsEnv := os.Getenv("DEVCELL_NO_SECRETS")
	if noSecretsEnv == "" {
		noSecretsEnv = os.Getenv("DEVCELL_NO_1PASSWORD")
	}
	switch {
	case op.ShouldResolve(skipSecrets, noSecretsEnv, opDocs):
		ux.Debugf("1Password: resolving %d document(s): %v", len(opDocs), opDocs)
		_ = pr.PhaseDetailedRunning("Loading secrets (please authorize 1Password)", "Loaded secrets", func() (string, error) {
			if _, err := exec.LookPath("op"); err != nil {
				return "", fmt.Errorf("1Password CLI not installed")
			}
			resolved, errs := op.ResolveItems(opDocs)
			for _, e := range errs {
				ux.Debugf("1Password: %v", e)
			}
			resolvedSecrets = resolved
			keys := make([]string, 0, len(resolved))
			for k, v := range resolved {
				os.Setenv(k, v)
				inheritEnv = append(inheritEnv, k)
				keys = append(keys, k)
			}
			ux.Debugf("1Password: resolved %d secret(s) from %d document(s) (%d failed): %v",
				len(keys), len(opDocs)-len(errs), len(errs), keys)
			// Total failure (every item errored, nothing resolved) is a real
			// boot failure — surface it as ✗ instead of a green ✓ with a
			// misleading "0 resolved" detail. Partial success still renders
			// as ✓ because the cell can boot with whatever secrets landed.
			if len(resolved) == 0 && len(errs) > 0 {
				if len(opDocs) == 1 {
					return "", fmt.Errorf("could not read %q from 1Password", opDocs[0])
				}
				return "", fmt.Errorf("could not read any of %d 1Password documents", len(opDocs))
			}
			return ux.FormatSecretsPhase(len(resolved), len(errs)), nil
		})
	case len(opDocs) > 0 && (skipSecrets || noSecretsEnv != ""):
		ux.Debugf("1Password: skipped (--no-secrets / DEVCELL_NO_SECRETS)")
	}

	// Resolve deferred API keys that depend on 1Password secrets.
	if extraEnv != nil {
		// Agents in openrouter mode set an empty placeholder to request the key.
		if v, ok := extraEnv["OPENROUTER_API_KEY"]; ok && v == "" {
			if err := cell.FillOpenRouterKey(extraEnv); err != nil {
				return err
			}
			// Claude Code authenticates to OpenRouter with the same key.
			if t, ok := extraEnv["ANTHROPIC_AUTH_TOKEN"]; ok && t == "" {
				extraEnv["ANTHROPIC_AUTH_TOKEN"] = extraEnv["OPENROUTER_API_KEY"]
			}
		}
	}

	// Inject a deterministic session ID so agents resume the same
	// conversation when relaunched in the same tmux pane.
	// Claude Code: CLAUDE_CODE_SESSION_ID env var names a new/existing session.
	// OpenCode: --session requires an existing ID (no create-or-resume), so
	// we skip it. OpenCode's --continue resumes the last session in the
	// project directory, which the user can invoke manually.
	if binary == "claude" {
		sessID := cell.SessionID(c.AppName)
		if extraEnv == nil {
			extraEnv = make(map[string]string)
		}
		extraEnv["CLAUDE_CODE_SESSION_ID"] = sessID
	}

	// Expand [env] references after secret resolution:
	//   ${VAR} / $VAR  — resolved against the host environment
	//   ${secret:NAME} — resolved against 1Password secrets
	if err := cfg.ExpandEnv(cellCfg.Env, os.LookupEnv, resolvedSecrets); err != nil {
		return fmt.Errorf("%w", err)
	}

	// Validate and prepare WireGuard configs before docker run.
	if cfg.WireguardEnabled(cellCfg) {
		if err := cfg.ValidateWireguard(cellCfg); err != nil {
			return fmt.Errorf("wireguard config: %w", err)
		}
		if err := PrepareWireguard(c.CellHome, cellCfg); err != nil {
			return fmt.Errorf("wireguard prepare: %w", err)
		}
	}

	// Final ✓ row before docker exec takes the TTY. The phase checklist
	// stays on screen — the child TUI (claude, codex, …) draws on the row
	// immediately below `✓ Cell ready`, so users keep the full boot story
	// as scrollback above their session.
	pr.Seal("Cell ready")

	// CELL-264: in-container progress via fsnotify sentinel files. Start
	// a BootDirWatcher on a per-cell directory BEFORE docker run so the
	// container's entrypoint fragments can `touch $DEVCELL_BOOT_DIR/<name>`
	// as they boot. Each file CREATE becomes a row on the host between
	// Cell ready and the TTY handoff.
	//
	// Directory bind-mounts work universally on every Docker platform —
	// Linux native, macOS/Windows Docker Desktop, Lima, OrbStack — which
	// is why we ditched CELL-263 (sd_notify unix-socket bind-mounts had
	// transport issues through Docker Desktop's virtiofs).
	//
	// Stale-state hygiene: wipe the dir at the start of each launch so
	// leftover sentinels from a crashed prior run don't fire spurious
	// "ready" events before the new boot starts emitting them.
	bootDir := filepath.Join(c.CellHome, "boot")
	_ = os.RemoveAll(bootDir)
	bootWatcher := &BootDirWatcher{}
	var bootDirEnv string
	var bootEvents <-chan BootEvent
	if events, err := bootWatcher.Start(bootDir); err != nil {
		ux.Debugf("boot watcher: %v (continuing without in-container progress)", err)
	} else {
		bootDirEnv = bootDir
		bootEvents = events
		defer bootWatcher.Close()
	}

	// CELL-447: detect project flake.nix and prompt for trust host-side.
	// Flake is opt-in: enabled by --use-flake flag or flake=true in [cell] config.
	useFlake := opts.UseFlake || cellCfg.Cell.FlakeEnabled()
	trustFlake := false
	if useFlake {
		trustFlake = resolveTrustFlake(c.BaseDir, c.CellHome)
	}

	spec := newRunSpec(opts, c)
	spec.DefaultFlags = defaultFlags
	spec.ExtraEnv = extraEnv
	spec.TrustFlake = trustFlake
	spec.Image = imageID
	spec.GitConfig = gitConfig
	spec.InheritEnv = inheritEnv
	spec.BootDir = bootDirEnv
	spec.TTY = isatty.IsTerminal(os.Stdin.Fd())
	argv := BuildArgv(spec, OsFS, exec.LookPath)

	if dryRun {
		fmt.Println(cell.ShellJoin(argv))
		return nil
	}

	cmd := exec.Command(argv[0], argv[1:]...)

	if opts.Detach {
		// Detached: docker run -d prints container ID and exits.
		// Suppress stdout (container ID) and only show errors.
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("start container: %w", err)
		}
		fmt.Printf("Container %s started\n", c.ContainerName)
		return nil
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sess, sessErr := cellrun.Begin(c.BaseDir, binary, userArgs)
	if sessErr != nil {
		ux.Debugf("cellrun begin: %v", sessErr)
	}
	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		if sess != nil {
			_ = sess.Finish(c.BaseDir, err)
		}
		return fmt.Errorf("start %q: %w", argv[0], err)
	}

	// CELL-264: consume in-container boot events. Each sentinel file
	// CREATE opens or seals a row. Entrypoint is mostly quiet in non-debug
	// mode, so host rows and container stdout rarely interleave during
	// boot; once the entrypoint emits boot.ready and exec's into the
	// binary (claude/zsh), the consumer returns and stops rendering.
	if bootEvents != nil {
		go ConsumeBootEvents(bootEvents)
	}

	// Forward signals to the child process.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			_ = cmd.Process.Signal(sig)
		}
	}()

	waitErr := cmd.Wait()
	telemetry.TrackCommandFinish(filepath.Base(binary), time.Since(startTime).Milliseconds(), waitErr == nil)
	if sess != nil {
		if err := sess.Finish(c.BaseDir, waitErr); err != nil {
			ux.Debugf("cellrun finish: %v", err)
		}
	}
	return agentExit(waitErr)
}

// newRunSpec maps opts onto the RunSpec BuildArgv takes. Run fills in what
// its boot checklist resolves: the pinned image, prompt flags, agent env,
// secrets, git identity, boot dir, flake trust and TTY.
func newRunSpec(opts engine.RunOpts, c config.Config) RunSpec {
	return RunSpec{
		Config:       c,
		CellCfg:      opts.Cell.Config,
		Binary:       opts.Binary,
		DefaultFlags: opts.DefaultFlags,
		UserArgs:     opts.Args,
		Debug:        opts.Debug,
		NixDaemon:    opts.NixDaemon,
		NoPorts:      opts.NoPorts,
		ExtraEnv:     opts.Env,
		ThinImage:    true,
		Detach:       opts.Detach,
		NoSecrets:    opts.NoSecrets,
	}
}

// agentExit is what Run returns once the agent's container exits: nil on
// success, *engine.ExitError for a non-zero status, else err.
func agentExit(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &engine.ExitError{Code: exitErr.ExitCode()}
	}
	return err
}

// resolveTrustFlake checks if the project has a flake.nix and whether the
// user has trusted it. On first encounter, prompts interactively and caches
// the answer in cellHome. Returns true if DEVCELL_FLAKE_TRUST=1 should be
// passed to the container.
func resolveTrustFlake(baseDir, cellHome string) bool {
	flakePath := filepath.Join(baseDir, "flake.nix")
	if _, err := os.Stat(flakePath); err != nil {
		return false
	}

	trustFile := filepath.Join(cellHome, "flake-trust")
	if data, err := os.ReadFile(trustFile); err == nil {
		return strings.TrimSpace(string(data)) == "1"
	}

	if !isatty.IsTerminal(os.Stdin.Fd()) {
		ux.Debugf("project-flake: found flake.nix but stdin is not a terminal — skipping trust prompt")
		return false
	}

	fmt.Printf("\n Found flake.nix in %s\n", baseDir)
	fmt.Printf(" Install its packages into this cell? [Y/n] ")

	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(answer)

	trusted := answer == "" || strings.HasPrefix(strings.ToLower(answer), "y")

	_ = os.MkdirAll(cellHome, 0o755)
	if trusted {
		_ = os.WriteFile(trustFile, []byte("1\n"), 0o644)
	} else {
		_ = os.WriteFile(trustFile, []byte("0\n"), 0o644)
	}

	return trusted
}

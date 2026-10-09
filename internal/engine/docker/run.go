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

	streak "github.com/dimmkirr/go-streak-chart"
	"github.com/mattn/go-isatty"

	"github.com/DimmKirr/devcell/internal/backup"
	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cellrun"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/op"
	"github.com/DimmKirr/devcell/internal/s6"
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

	ux.SaveCursor()

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

	// Build the 4-group streak-chart boot panel.
	hasSecretsResolve := op.ShouldResolve(opts.NoSecrets, os.Getenv("DEVCELL_NO_SECRETS"), cellCfg.Op.ResolvedDocuments())
	_, hasORKeyPlaceholder := extraEnv["OPENROUTER_API_KEY"]
	gopts := groupOpts{
		hasPackages:    ImageLabel(ctx, imageTag(), "devcell.packages") != "",
		hasGitLookup:   cell.ResolveGitIdentity(os.Getenv, cellCfg.Git, nil).Source == cell.GitSourceDefault,
		hasSecrets:     hasSecretsResolve,
		hasAPIKeys:     hasORKeyPlaceholder && extraEnv["OPENROUTER_API_KEY"] == "",
		hasWireGuard:   cfg.WireguardEnabled(cellCfg),
		hasGUI:         cellCfg.GUI.ResolvedEnabled(),
		hasMCP:         binary == "claude" || binary == "codex" || binary == "opencode",
		hasNixPackages: opts.UseFlake || cellCfg.Cell.FlakeEnabled(),
	}
	groups := bootGroups(binary, gopts)
	panel := ux.NewBootPanel(groups)

	// ── Group 1: Prepare ─────────────────────────────────────────────────
	_ = panel.Phase("Docker", func() error { return nil }) // daemon already checked above

	_ = panel.Phase("Network", func() error { return EnsureNetwork(ctx) })

	// Orphan cleanup: remove stopped containers. Running containers are
	// left alone so we can attach to them via docker exec.
	if err := panel.Phase("Orphan cleanup", func() error {
		if ContainerRunning(ctx, c.ContainerName) {
			return nil
		}
		return RemoveOrphanedContainer(ctx, c.ContainerName)
	}); err != nil {
		return err
	}

	if err := nixStorePhase(ctx, panel, thin, c.BaseDir, cellCfg.Cell.StaleWarningEnabled(), opts.AutoCleanup); err != nil {
		return err
	}

	if err := closureCheckPhase(ctx, panel, thin, imageTag(), func() error {
		return build(ctx, opts.Cell, autoBuildParams(opts.Cell))
	}); err != nil {
		return err
	}

	_ = panel.Phase("Backup", func() error { return backup.Backup(c.CellHome, time.Now()) })

	var imageID string
	_ = panel.PhaseDetailed("Image pin", func() (string, error) {
		id, idErr := LocalImageIDFor(ctx, imageTag())
		if idErr != nil {
			imageID = imageTag()
			return imageID, nil
		}
		imageID = id
		short := id
		if len(short) > 19 {
			short = short[:19]
		}
		return short, nil
	})

	// ── Group 2: Configure ───────────────────────────────────────────────
	if pkgs := ImageLabel(ctx, imageID, "devcell.packages"); pkgs != "" {
		_ = panel.PhaseDetailed("Packages", func() (string, error) { return pkgs, nil })
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
		if err := panel.PhaseDetailed("Prompt", func() (string, error) {
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
			if ux.Verbose {
				return flags[len(flags)-1], nil
			}
			return "configured", nil
		}); err != nil {
			return fmt.Errorf("system prompt: %w", err)
		}
	}

	if binary == "codex" {
		if err := panel.PhaseDetailed("Prompt", func() (string, error) {
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
		_ = panel.PhaseDetailed("Git", func() (string, error) {
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
		_ = panel.PhaseDetailedRunningWarn("Loading secrets (please authorize 1Password)", "Secrets", func() (string, bool, error) {
			if _, err := exec.LookPath("op"); err != nil {
				return "", false, fmt.Errorf("1Password CLI not installed")
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
			if len(resolved) == 0 && len(errs) > 0 {
				if len(opDocs) == 1 {
					return "", false, fmt.Errorf("could not read %q from 1Password", opDocs[0])
				}
				return "", false, fmt.Errorf("could not read any of %d 1Password documents", len(opDocs))
			}
			detail, warn := ux.FormatSecretsPhase(len(resolved), len(errs))
			return detail, warn, nil
		})
	case len(opDocs) > 0 && (skipSecrets || noSecretsEnv != ""):
		ux.Debugf("1Password: skipped (--no-secrets / DEVCELL_NO_SECRETS)")
	}

	// Resolve deferred API keys that depend on 1Password secrets.
	if extraEnv != nil {
		if v, ok := extraEnv["OPENROUTER_API_KEY"]; ok && v == "" {
			if err := panel.Phase("API keys", func() error {
				if fillErr := cell.FillOpenRouterKey(extraEnv); fillErr != nil {
					return fillErr
				}
				if t, ok := extraEnv["ANTHROPIC_AUTH_TOKEN"]; ok && t == "" {
					extraEnv["ANTHROPIC_AUTH_TOKEN"] = extraEnv["OPENROUTER_API_KEY"]
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	if binary == "claude" {
		sessID := cell.SessionID(c.AppName)
		if extraEnv == nil {
			extraEnv = make(map[string]string)
		}
		extraEnv["CLAUDE_CODE_SESSION_ID"] = sessID
	}

	if err := cfg.ExpandEnv(cellCfg.Env, os.LookupEnv, resolvedSecrets); err != nil {
		return fmt.Errorf("%w", err)
	}

	if cfg.WireguardEnabled(cellCfg) {
		if err := panel.Phase("WireGuard", func() error {
			if valErr := cfg.ValidateWireguard(cellCfg); valErr != nil {
				return fmt.Errorf("wireguard config: %w", valErr)
			}
			return PrepareWireguard(c.CellHome, cellCfg)
		}); err != nil {
			return err
		}
	}

	// Boot dir: hosts the Go-generated entrypoint script and (for backward
	// compat) sentinel files from the entrypoint's _notify. Boot progress
	// is now consumed from docker logs (JSONL events from devcell-event and
	// _notify), not from polling sentinel files.
	bootDir := filepath.Join(c.CellHome, "boot")
	_ = os.RemoveAll(bootDir)
	if err := os.MkdirAll(bootDir, 0o755); err != nil {
		ux.Debugf("boot dir: %v", err)
	}

	// Write the Go-generated init script into bootDir.
	{
		env := s6.LinuxSessionEnv(c.HostUser)
		script := "#!/bin/bash\nset -e\n" + s6.EntrypointSnippet(env) + "\n"
		initPath := filepath.Join(bootDir, "entrypoint.sh")
		if err := os.WriteFile(initPath, []byte(script), 0o755); err != nil {
			return fmt.Errorf("write init script: %w", err)
		}
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
	spec.BootDir = bootDir
	spec.TTY = isatty.IsTerminal(os.Stdin.Fd())

	if dryRun {
		argv := BuildArgv(spec, OsFS, exec.LookPath)
		fmt.Println(cell.ShellJoin(argv))
		return nil
	}

	if opts.Detach {
		// cell start: detached with agent as CMD (unchanged).
		spec.Detach = true
		argv := BuildArgv(spec, OsFS, exec.LookPath)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("start container: %w", err)
		}
		fmt.Printf("Container %s started\n", c.ContainerName)
		return nil
	}

	// ── Interactive: detach + exec ────────────────────────────────────────
	// The container runs detached with `sleep infinity` as CMD.
	// Boot events (JSONL from devcell-event + _notify) flow through docker
	// logs. Once boot is done, we docker exec into the agent binary.
	// The container survives agent exit; `cell stop` tears it down.

	sess, sessErr := cellrun.Begin(c.BaseDir, binary, userArgs)
	if sessErr != nil {
		ux.Debugf("cellrun begin: %v", sessErr)
	}
	startTime := time.Now()

	alreadyRunning := ContainerRunning(ctx, c.ContainerName)

	// ── Group 3: Boot ────────────────────────────────────────────────────
	if !alreadyRunning {
		panel.SetBoot("Boot", "Container", streak.Running, "Starting container")
		startSpec := spec
		startSpec.Detach = true
		startSpec.Binary = "sleep"
		startSpec.DefaultFlags = nil
		startSpec.UserArgs = []string{"infinity"}
		startSpec.TTY = false
		if err := ensureRunning(c.ContainerName, 10*time.Second,
			func() bool { return ContainerRunning(ctx, c.ContainerName) },
			func() error {
				startArgv := BuildArgv(startSpec, OsFS, exec.LookPath)
				cmd := exec.CommandContext(ctx, startArgv[0], startArgv[1:]...)
				cmd.Stderr = os.Stderr
				return cmd.Run()
			},
		); err != nil {
			panel.SetBoot("Boot", "Container", streak.Error, "start failed")
			panel.FinishError("start failed")
			if sess != nil {
				_ = sess.Finish(c.BaseDir, err)
			}
			return err
		}
		panel.SetBoot("Boot", "Container", streak.Done, "Container started")

		// Watch JSONL events from docker logs for boot progress.
		logWatcher := NewContainerLogWatcher(ctx, c.ContainerName, startTime)
		defer logWatcher.Close()

		bootDone := make(chan bool, 1)
		go func() {
			bootDone <- ConsumeContainerEventsPanel(logWatcher.Events(), panel)
		}()

		select {
		case ok := <-bootDone:
			if !ok {
				panel.FinishError("boot interrupted")
				if sess != nil {
					_ = sess.Finish(c.BaseDir, fmt.Errorf("boot interrupted"))
				}
				return fmt.Errorf("boot did not complete")
			}
		case <-time.After(90 * time.Second):
			logWatcher.Close()
			panel.FinishError("boot timed out")
			if sess != nil {
				_ = sess.Finish(c.BaseDir, fmt.Errorf("boot timed out"))
			}
			return fmt.Errorf("boot timed out after 90s")
		}
	} else {
		panel.SetBoot("Boot", "Container", streak.Done, "Container running")
		panel.Finish("Cell ready (attached)")
	}
	panel.ClearBelowGroups()

	// ── Exec into container ─────────────────────────────────────────────
	env := s6.LinuxSessionEnv(c.HostUser)
	execEnv := map[string]string{
		"PATH": s6.LinuxExecPATH(env),
		"HOME": "/home/" + c.HostUser,
		"TERM": os.Getenv("TERM"),
	}
	for k, v := range extraEnv {
		execEnv[k] = v
	}
	execArgv := BuildExecArgv(ExecSpec{
		ContainerName: c.ContainerName,
		User:          c.HostUser,
		Binary:        binary,
		Args:          append(defaultFlags, userArgs...),
		TTY:           isatty.IsTerminal(os.Stdin.Fd()),
		Env:           execEnv,
	})
	cmd := exec.Command(execArgv[0], execArgv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		if sess != nil {
			_ = sess.Finish(c.BaseDir, err)
		}
		return fmt.Errorf("exec %q: %w", binary, err)
	}

	waitErr := cmd.Wait()
	signal.Stop(sigCh)
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

// ensureRunning starts the container if it's not already running.
// checkRunning and start are injectable for testing.
func ensureRunning(name string, timeout time.Duration, checkRunning func() bool, start func() error) error {
	if checkRunning() {
		return nil
	}
	if err := start(); err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	deadline := time.After(timeout)
	for {
		if checkRunning() {
			return nil
		}
		select {
		case <-deadline:
			return fmt.Errorf("container %s did not start in time", name)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// containerBooted checks if the container has completed boot by testing
// for the session-ready file inside the container.
func containerBooted(ctx context.Context, name string) bool {
	err := exec.CommandContext(ctx, "docker", "exec", name, "test", "-f", "/run/devcell-session-ready").Run()
	return err == nil
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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/engine/docker"
	"github.com/DimmKirr/devcell/internal/scaffold"
	"github.com/DimmKirr/devcell/internal/telemetry"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "cell",
	SilenceUsage: true, // don't dump usage after handled errors
	Short:        "Run AI coding agents in a devcell container",
	Long: `cell launches AI coding agents (claude, codex, opencode) and utility
tools inside a consistent Docker dev environment.`,
	Args: cobra.ArbitraryArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			fmt.Fprintf(os.Stderr, "cell %s\n", version.Full())
		}
		warnDeprecatedFlags(os.Stderr)
		// Set docker engine globals BEFORE any subcommand RunE so that
		// docker.UserImageTag() / PickImageTag() reflect the project's
		// stack from .devcell.toml.
		//
		// Best-effort: silently skips when config can't be loaded (e.g.,
		// `cell --help` before cwd has a .devcell.toml, or stray cwd).
		// Subcommands that need a working config fail with a better error
		// later in their own RunE.
		if c, err := config.LoadFromOS(); err == nil {
			cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
			docker.Stack = cellCfg.Cell.ResolvedStack()
			docker.Modules = cellCfg.Cell.Modules
			docker.PerCellImage = cellCfg.Cell.ResolvedPerCellImage()
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			_ = cmd.Help()
			return fmt.Errorf("unknown command %q", args[0])
		}
		// A valid default_command never reaches here — applyDefaultCommand
		// rewrites os.Args before Execute, so cobra dispatches to the
		// subcommand directly (with user args forwarded). Only an invalid
		// value falls through; surface the validation error.
		if c, err := config.LoadFromOS(); err == nil {
			cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
			if dc := cellCfg.Cell.ResolvedDefaultCommand(); dc != "" {
				if err := cfg.ValidateDefaultCommand(dc); err != nil {
					return err
				}
			}
		}
		return cmd.Help()
	},
}

// rewriteDefaultCommand injects the configured default command in front of
// the user's args (os.Args[1:]) so flags and positionals reach the inner
// binary: `cell -c` becomes `cell claude -c`. This must happen BEFORE
// rootCmd.Execute() — the root command parses flags, so an agent flag like
// -c would die there as "unknown shorthand flag" and never reach the
// default-command dispatch in RunE. An explicit subcommand, help/version,
// or completion invocation is left untouched.
func rewriteDefaultCommand(args []string, defaultCmd string, knownCmds map[string]bool) []string {
	if defaultCmd == "" {
		return args
	}
	if len(args) > 0 {
		first := args[0]
		if knownCmds[first] {
			return args
		}
		switch first {
		case "--help", "-h", "--version", "help", "completion", "__complete", "__completeNoDesc":
			return args
		}
		// An unknown positional (not a flag) is a typo'd subcommand, not
		// input for the default command — leave it for rootCmd.RunE to
		// report as "unknown command" instead of silently launching the
		// default agent with it.
		if !strings.HasPrefix(first, "-") {
			return args
		}
	}
	return append([]string{defaultCmd}, args...)
}

// applyDefaultCommand resolves default_command from config and rewrites
// os.Args in place. Invalid values are left for rootCmd.RunE to report.
func applyDefaultCommand() {
	c, err := config.LoadFromOS()
	if err != nil {
		return
	}
	cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
	dc := cellCfg.Cell.ResolvedDefaultCommand()
	if dc == "" || cfg.ValidateDefaultCommand(dc) != nil {
		return
	}
	known := make(map[string]bool)
	for _, sub := range rootCmd.Commands() {
		known[sub.Name()] = true
		for _, a := range sub.Aliases {
			known[a] = true
		}
	}
	rewritten := rewriteDefaultCommand(os.Args[1:], dc, known)
	os.Args = append([]string{os.Args[0]}, rewritten...)
	osArgs = os.Args // keep scanFlag/scanStringFlag on the rewritten argv
}

// Instructions shown when a deprecated flag is passed.
const (
	localFlagDeprecation   = "remove it; the libvirt engine was retired, so --engine winkit always runs locally"
	macosFlagDeprecation   = "use --os macos instead"
	vagrantFlagDeprecation = "remove it; the vagrant engine was retired, use --os macos for a Tart VM"
	qemuSSHPortDeprecation = "use --winkit-ssh-port instead, e.g. --winkit-ssh-port 2222"
	qemuISODeprecation     = "use --winkit-windows-iso instead, e.g. --winkit-windows-iso ~/Downloads/Win11_ARM64.iso"
	qemuSSHHostDeprecation = "remove it; it has no effect (winkit forwards SSH on 127.0.0.1)"
	qemuDisplayDeprecation = "remove it; it has no effect (use `cell vnc` or `cell rdp` to see the VM)"
	tartSSHFlagDeprecation = "remove it; it has no effect (tart runs commands with tart exec)"
)

// scannedFlagDeprecations are the deprecated flags read from argv. Each
// stays registered (hidden) so cobra keeps accepting it; the notice comes
// from warnDeprecatedFlags rather than cobra's MarkDeprecated so it renders
// through ux.Deprecated like every other deprecation.
var scannedFlagDeprecations = []struct{ flag, message string }{
	{"--local", localFlagDeprecation},
	{"--macos", macosFlagDeprecation},
	{"--vagrant-provider", vagrantFlagDeprecation},
	{"--vagrant-box", vagrantFlagDeprecation},
	{"--qemu-ssh-port", qemuSSHPortDeprecation},
	{"--qemu-windows-iso", qemuISODeprecation},
	{"--qemu-ssh-host", qemuSSHHostDeprecation},
	{"--qemu-display", qemuDisplayDeprecation},
	{"--tart-ssh-port", tartSSHFlagDeprecation},
	{"--tart-ssh-host", tartSSHFlagDeprecation},
}

// warnDeprecatedFlags warns on w about each deprecated flag in argv. It
// runs once per invocation from rootCmd's PersistentPreRun, which cobra
// runs for every subcommand, including agent commands that set
// DisableFlagParsing (their flags are scanned from argv, not parsed).
func warnDeprecatedFlags(w io.Writer) {
	for _, d := range scannedFlagDeprecations {
		if scanFlag(d.flag) || scanStringFlag(d.flag) != "" {
			ux.Deprecated(w, d.flag, d.message)
		}
	}
}

// warnConfigDeprecations goes to stderr so --format json/yaml stdout stays
// parseable. Only the config file name is shown: the global file is always
// devcell.toml and the project file .devcell.toml, so the base name is
// unambiguous and the row stays short.
func warnConfigDeprecations(w io.Writer, c cfg.CellConfig) {
	for _, u := range c.DeprecatedUses {
		if u.File == "" {
			ux.Deprecated(w, u.Name, u.Message)
			continue
		}
		ux.DeprecatedAt(w, u.Name, u.Message, filepath.Base(u.File))
	}
}

// skipsConfigCheck lists invocations that must work with a broken config:
// shell completion, help, --version, and `cell config` itself (migrate
// loads and reports the files on its own; the startup rows would only
// repeat what it is about to fix).
func skipsConfigCheck(arg string) bool {
	switch arg {
	case "help", "completion", "-h", "--help", "--version", "config", "openapi":
		return true
	}
	return strings.HasPrefix(arg, "__complete")
}

// checkConfig loads the layered config once before any command runs. An
// invalid config (unknown keys, conflicting [llm] settings) stops cell here;
// later cfg.LoadFromOS calls would only warn and fall back to defaults.
func checkConfig() {
	c, err := config.LoadFromOS()
	if err != nil {
		return
	}
	cellCfg, err := cfg.LoadFromOSWithDirs(c.ConfigDir, c.BaseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	warnConfigDeprecations(os.Stderr, cellCfg)
}

func Execute() {
	defer ux.CloseDebugLog()
	telemetry.Init(resolveConfigDir())
	defer telemetry.Close()
	if len(os.Args) < 2 || !skipsConfigCheck(os.Args[1]) {
		checkConfig()
	}
	applyDefaultCommand()
	if err := rootCmd.Execute(); err != nil {
		var ue *usageError
		if errors.As(err, &ue) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "\n cell %s\n", version.Full())
		baseVer, userVer := docker.ImageVersions(context.Background())
		if baseVer != "" {
			fmt.Fprintf(os.Stderr, " Base image: %s\n", baseVer)
		}
		if userVer != "" {
			fmt.Fprintf(os.Stderr, " User image: %s\n", userVer)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = version.Full()
	rootCmd.SetFlagErrorFunc(flagUsageError)
	rootCmd.PersistentFlags().Bool("build", false, "rebuild image before running (forces --no-cache)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "print docker run argv and exit without running")
	rootCmd.PersistentFlags().Bool("plain-text", false, "disable spinners, use plain log output (for CI/non-TTY)")
	rootCmd.PersistentFlags().Bool("debug", false, "plain-text mode plus stream full build log to stdout")
	rootCmd.PersistentFlags().String("format", "text", "output format: text, yaml, or json")
	rootCmd.PersistentFlags().String("engine", "", "execution engine, how the cell runs: docker (Linux container), tart (macOS VM) or winkit (Windows VM); default: picked from --os, else docker")
	rootCmd.PersistentFlags().String("os", "", "guest OS inside the cell: linux (docker), macos (tart), windows or winpe (winkit, Windows PE + WSL1); picks the engine when --engine is unset")
	rootCmd.PersistentFlags().Bool("local", false, "no effect (the libvirt engine was retired)")
	_ = rootCmd.PersistentFlags().MarkHidden("local")
	rootCmd.PersistentFlags().Bool("background", false, "keep VM/container running after shell exit")
	rootCmd.PersistentFlags().Bool("macos", false, "use a macOS VM (deprecated alias for --os macos)")
	_ = rootCmd.PersistentFlags().MarkHidden("macos")
	rootCmd.PersistentFlags().String("vagrant-provider", "", "no effect (the vagrant engine was retired)")
	_ = rootCmd.PersistentFlags().MarkHidden("vagrant-provider")
	rootCmd.PersistentFlags().String("vagrant-box", "", "no effect (the vagrant engine was retired)")
	_ = rootCmd.PersistentFlags().MarkHidden("vagrant-box")
	rootCmd.PersistentFlags().String("winkit-ssh-port", "", "SSH port for winkit engine (default: allocated per bunk)")
	rootCmd.PersistentFlags().String("winkit-windows-iso", "", "path to Windows ARM64 ISO for winkit engine")
	rootCmd.PersistentFlags().String("qemu-ssh-port", "", "deprecated alias for --winkit-ssh-port")
	_ = rootCmd.PersistentFlags().MarkHidden("qemu-ssh-port")
	rootCmd.PersistentFlags().String("qemu-windows-iso", "", "deprecated alias for --winkit-windows-iso")
	_ = rootCmd.PersistentFlags().MarkHidden("qemu-windows-iso")
	rootCmd.PersistentFlags().String("qemu-ssh-host", "", "no effect (deprecated)")
	_ = rootCmd.PersistentFlags().MarkHidden("qemu-ssh-host")
	rootCmd.PersistentFlags().String("qemu-display", "", "no effect (deprecated)")
	_ = rootCmd.PersistentFlags().MarkHidden("qemu-display")
	rootCmd.PersistentFlags().String("tart-ssh-port", "", "no effect (tart runs commands with tart exec)")
	_ = rootCmd.PersistentFlags().MarkHidden("tart-ssh-port")
	rootCmd.PersistentFlags().String("tart-ssh-host", "", "no effect (tart runs commands with tart exec)")
	_ = rootCmd.PersistentFlags().MarkHidden("tart-ssh-host")
	rootCmd.PersistentFlags().String("base-image", "", "core image override for the container build (default: ghcr.io/devcell-sh/devcell:core-local)")
	rootCmd.PersistentFlags().String("cell-name", "", "cell name for persistent home (~/.devcell/<name>)")
	rootCmd.AddCommand(
		claudeCmd,
		codexCmd,
		opencodeCmd,
		geminiCmd,
		shellCmd,
		startCmd,
		stopCmd,
		buildCmd,
		initCmd,
		vncCmd,
		rdpCmd,
		modelsCmd,
		modulesCmd,
		configCmd,
		serveCmd,
		authCmd,
		telemetryCmd,
		openapiCmd,
	)
}

// applyOutputFlags reads --plain-text and --debug and sets ux globals.
// Must be called at the start of each RunE (PersistentPreRun is skipped
// for commands with DisableFlagParsing=true).
// applyOutputFlags scans os.Args for --plain-text and --debug.
// We cannot use cobra's flag parsing here because agent subcommands set
// DisableFlagParsing=true, which prevents cobra from parsing persistent
// flags on the root command.
func applyOutputFlags() {
	for _, arg := range osArgs {
		switch arg {
		case "--plain-text":
			ux.LogPlainText = true
		case "--debug":
			ux.LogPlainText = true
			ux.Verbose = true
		}
	}
	if f := scanStringFlag("--format"); f != "" {
		ux.OutputFormat = f
	}
}

// applyOutputFlagsWithLog calls applyOutputFlags and, when --debug is active,
// opens a persistent log file at <projectDir>/.devcell/debug/<ts>-<cmd>.log.
// commandName should be the bare subcommand name (e.g. "init", "build").
func applyOutputFlagsWithLog(commandName string) {
	applyOutputFlags()
	if !ux.Verbose {
		return
	}
	if c, err := config.LoadFromOS(); err == nil {
		ux.InitDebugLog(c.BaseDir, commandName)
	}
}

// cellBoolFlags are boolean flags consumed by devcell: strip the flag token only.
var cellBoolFlags = map[string]bool{
	"--build":        true,
	"--background":   true,
	"--dry-run":      true,
	"--plain-text":   true,
	"--debug":        true,
	"--macos":        true, // deprecated alias for --os macos
	"--ollama":       true,
	"--openrouter":   true,
	"--nix-daemon":   true, // enable nix-daemon inside container for runtime package installs
	"--thin":         true, // thin image mode (default)
	"--no-thin":      true, // legacy, ignored
	"--thick":        true, // legacy, ignored
	"--no-secrets":   true, // skip all secrets injection: op item get + op run -- prefix
	"--skip-secrets": true, // alias for --no-secrets
	"--no-1password": true, // alias for --no-secrets (legacy, CELL-42)
	"--local":        true, // deprecated no-op: only skipped the retired libvirt auto-default
	"--auto-cleanup": true, // run the CELL-334 root reaper at cell start (CELL-390)
	"--use-flake":    true, // opt-in to project-level flake.nix install (CELL-447)
	"--no-flake":     true, // legacy, ignored (flake is off by default now)
	"--skip-flake":   true, // legacy, ignored
	"--no-ports":     true, // skip allocating docker ports even if [ports] is configured
}

// cellStringFlags are string flags consumed by devcell: strip the flag token
// AND its value (handles both "--flag value" and "--flag=value" forms).
var cellStringFlags = map[string]bool{
	"--engine":             true,
	"--os":                 true,
	"--vagrant-provider":   true, // deprecated no-op: the vagrant engine was retired
	"--vagrant-box":        true, // deprecated no-op: the vagrant engine was retired
	"--winkit-ssh-port":    true,
	"--winkit-windows-iso": true,
	"--qemu-ssh-port":      true, // deprecated alias for --winkit-ssh-port
	"--qemu-windows-iso":   true, // deprecated alias for --winkit-windows-iso
	"--qemu-ssh-host":      true, // deprecated no-op
	"--qemu-display":       true, // deprecated no-op
	"--tart-ssh-port":      true, // deprecated no-op: tart runs commands with tart exec
	"--tart-ssh-host":      true, // deprecated no-op: tart runs commands with tart exec
	"--base-image":         true,
	"--cell-name":          true,
	"--format":             true,
}

// validateCellFlags reports the first cell string flag in args that has no
// value: the last token, followed by another flag, or "--flag=". Agent
// commands set DisableFlagParsing, so cobra never checks this for them, and
// scanStringFlag would read a missing value as unset.
func validateCellFlags(args []string) error {
	for i, a := range args {
		name, value, hasValue := strings.Cut(a, "=")
		if !cellStringFlags[name] {
			continue
		}
		if hasValue {
			if value == "" {
				return fmt.Errorf("flag needs an argument: %s", name)
			}
			continue
		}
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
			return fmt.Errorf("flag needs an argument: %s", name)
		}
	}
	return nil
}

// stripCellFlags removes devcell-specific flags (and their values) from args
// so they are not forwarded to the inner binary.
func stripCellFlags(args []string) []string {
	out := make([]string, 0, len(args))
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if cellBoolFlags[a] {
			continue
		}
		if cellStringFlags[a] {
			skipNext = true
			continue
		}
		// "--flag=value" form for string flags
		stripped := false
		for f := range cellStringFlags {
			if strings.HasPrefix(a, f+"=") {
				stripped = true
				break
			}
		}
		if stripped {
			continue
		}
		out = append(out, a)
	}
	return out
}

// runAgent is the shared pre-exec sequence for all agent and shell commands:
// it does the engine-neutral work (flags, config, first-run scaffold, [env]
// expansion, telemetry) and hands the rest to the cell's engine. extraEnv
// is an optional map of additional env vars injected into the guest (e.g.
// OPENCODE_CONFIG_CONTENT). Pass nil when not needed.
func runAgent(binary string, defaultFlags, userArgs []string, extraEnv map[string]string) error {
	if len(osArgs) > 0 {
		if err := validateCellFlags(osArgs[1:]); err != nil {
			return flagUsageError(rootCmd, err)
		}
	}
	userArgs = stripCellFlags(userArgs)
	applyOutputFlagsWithLog(filepath.Base(binary))

	// Override cell name via --cell-name flag. Must precede config.LoadFromOS,
	// which resolves c.CellName (and c.CellHome) from DEVCELL_CELL_NAME.
	if sn := scanStringFlag("--cell-name"); sn != "" {
		os.Setenv("DEVCELL_CELL_NAME", sn)
	}

	c, err := config.LoadFromOS()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override base image tag if --base-image is set.
	if bi := scanStringFlag("--base-image"); bi != "" {
		os.Setenv("DEVCELL_BASE_IMAGE", bi)
	}

	// First-run: scaffold .devcell.toml + .devcell/ files.
	if !scaffold.IsInitialized(c.BaseDir) {
		globalCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
		result, err := scaffold.RunInitFlow(scaffold.InitFlowOptions{
			BaseDir:    c.BaseDir,
			ConfigDir:  c.ConfigDir,
			NixhomeSrc: globalCfg.Nix.NixhomePath,
			Yes:        false,
			Offline:    scanFlag("--dry-run"),
		})
		if err != nil {
			return err
		}
		c.BuildDir = config.ResolveBuildDir(c.BaseDir, c.ConfigDir, true)
		fmt.Printf(" First run — scaffolding %s (stack: %s)\n", c.BaseDir, result.Stack)
	}

	cellCfg := cfg.LoadFromOS(c.ConfigDir, c.BaseDir)
	engineName, engineErr := resolveEngine(os.Stderr, cellCfg)
	if engineErr != nil {
		return engineErr
	}

	// [env] expansion moved into each engine's Run(), after secret
	// resolution: docker resolves 1Password secrets via os.Setenv before
	// expanding, so $SECRET_VAR references work. Tart and winkit expand
	// immediately (no secrets to wait for).

	opts := runOpts(engineCell(c, cellCfg), binary, defaultFlags, userArgs, extraEnv)
	// thin: docker runs thin images; the VM engines report false.
	telemetry.TrackCommandRun(filepath.Base(binary), string(engineName), opts.Cell.Stack, opts.Cell.Modules, engineName == engine.Docker)
	return runEngine(engineName, opts)
}

// runOpts is the engine.RunOpts for an agent command: cell c, the agent, and
// the cell flags scanned from argv.
func runOpts(c engine.Cell, binary string, defaultFlags, userArgs []string, extraEnv map[string]string) engine.RunOpts {
	return engine.RunOpts{
		Cell:         c,
		Binary:       binary,
		DefaultFlags: defaultFlags,
		Args:         userArgs,
		Env:          extraEnv,
		Rebuild:      scanFlag("--build"),
		Background:   scanFlag("--background"),
		DryRun:       scanFlag("--dry-run"),
		Debug:        scanFlag("--debug"),
		Detach:       startDetach,
		NoSecrets:    scanFlag("--no-secrets") || scanFlag("--skip-secrets") || scanFlag("--no-1password"),
		NixDaemon:    scanFlag("--nix-daemon"),
		NoPorts:      scanFlag("--no-ports"),
		UseFlake:     scanFlag("--use-flake"),
		AutoCleanup:  scanFlag("--auto-cleanup"),
	}
}

// osArgs is the argument source for flag scanning. Overridable in tests.
var osArgs = os.Args

// scanFlag checks osArgs for a boolean flag.
// Needed because DisableFlagParsing prevents cobra from parsing persistent
// flags on agent subcommands.
func scanFlag(flag string) bool {
	for _, arg := range osArgs {
		if arg == flag {
			return true
		}
	}
	return false
}

// scanStringFlag scans osArgs for a string flag, handling both
// "--flag value" and "--flag=value" forms. Returns "" if not found.
func scanStringFlag(flag string) string {
	for i, arg := range osArgs {
		if arg == flag && i+1 < len(osArgs) {
			return osArgs[i+1]
		}
		if strings.HasPrefix(arg, flag+"=") {
			return arg[len(flag)+1:]
		}
	}
	return ""
}

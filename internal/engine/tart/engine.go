package tart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/ux"
)

// Engine runs a cell in a macOS VM on Apple Silicon through the tart CLI.
// It registers itself as engine.Tart.
//
// Init and Build need darwin/arm64 and fail elsewhere. Run fails elsewhere
// too unless opts.Debug or opts.DryRun is set: with --debug on another host
// it simulates every step and prints each with a [MOCK] prefix.
type Engine struct{}

var _ engine.Engine = Engine{}

func init() {
	engine.Register(engine.Tart, Engine{})
}

// Build creates the macOS template VM the cell's instance is cloned from:
// it clones the OCI image (or the local base template when one exists),
// boots it, provisions nix, nix-darwin and the stack, and saves it as a
// template. opts.Stage "base" builds the stack-agnostic base template
// instead. opts.Stack overrides opts.Cell.Stack, and opts.Update implies
// opts.Force. Templates are keyed on the stack only; opts.Cell.Modules is
// not applied.
func (Engine) Build(ctx context.Context, opts engine.BuildOpts) error {
	p, err := newBuildParams(opts)
	if err != nil {
		return err
	}
	return build(ctx, p)
}

// buildParams is a tart build resolved from engine.BuildOpts.
type buildParams struct {
	cellName   string
	hostHome   string
	projectDir string
	stack      string
	ociImage   string // cloned when there is no local base template, and always for stage "base"
	stage      string // "base" or "full"
	force      bool   // replace an existing template
	noCache    bool   // pass --no-cache to `tart clone` of the OCI image
	dryRun     bool
}

func newBuildParams(opts engine.BuildOpts) (buildParams, error) {
	stage := opts.Stage
	if stage == "" {
		stage = "full"
	}
	if stage != "base" && stage != "full" {
		return buildParams{}, fmt.Errorf("--stage must be \"base\" or \"full\", got %q", stage)
	}
	return buildParams{
		cellName:   opts.Cell.Name,
		hostHome:   opts.Cell.HostHome,
		projectDir: opts.Cell.BaseDir,
		stack:      opts.Cell.WithStack(opts.Stack).Stack,
		ociImage:   opts.Cell.Config.Cell.ResolvedTartOCIImage(),
		stage:      stage,
		force:      opts.Force || opts.Update,
		noCache:    opts.NoCache,
		dryRun:     opts.DryRun,
	}, nil
}

// prepareHost creates the cell's SSH directory and keypair. It mirrors what
// Docker init does: scaffold config, no images, no containers. The VM itself
// is created by Build. force replaces an existing keypair.
func prepareHost(cellName, hostHome, stack string, force bool) error {
	ic := InitConfig{
		CellName: cellName,
		HomeDir:  hostHome,
		Stack:    stack,
	}
	ic.ApplyDefaults()
	if err := ic.Validate(); err != nil {
		return err
	}

	sshPaths := NewCellSSHPaths(hostHome, cellName)

	ux.Debugf("init config: cell=%s stack=%s", ic.CellName, ic.Stack)
	ux.Debugf("ssh dir: %s", sshPaths.Dir)

	pr := &ux.PhaseRunner{}

	// --- Phase 1: Prepare SSH directory ---
	if err := pr.PhaseDetailed("Preparing SSH directory", func() (string, error) {
		if err := PrepareArtifactDir(sshPaths.Dir); err != nil {
			return "", fmt.Errorf("creating SSH dir: %w", err)
		}
		return sshPaths.Dir, nil
	}); err != nil {
		return err
	}

	// --- Phase 2: Generate SSH keypair ---
	if err := pr.PhaseDetailed("Generating SSH keypair", func() (string, error) {
		if !force {
			if _, err := os.Stat(sshPaths.PrivateKey); err == nil {
				ux.Debugf("SSH keypair already exists, skipping (use --force to regenerate)")
				return sshPaths.PrivateKey, nil
			}
		}

		pubKey, err := GenerateSSHKeyPair(sshPaths.Dir)
		if err != nil {
			return "", fmt.Errorf("generating SSH keys: %w", err)
		}
		ux.Debugf("SSH public key: %s", pubKey)

		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			existing := CollectSSHPubKeys(filepath.Join(homeDir, ".ssh"))
			if existing != "" {
				pubKey = strings.TrimSpace(pubKey) + "\n" + existing
				ux.Debugf("added existing ~/.ssh pub keys (%d total lines)",
					len(strings.Split(pubKey, "\n")))
			}
		}

		pubKeyPath := filepath.Join(sshPaths.Dir, "authorized_keys")
		if err := os.WriteFile(pubKeyPath, []byte(pubKey), 0644); err != nil {
			return "", fmt.Errorf("writing authorized_keys: %w", err)
		}
		ux.Debugf("wrote authorized_keys to %s", pubKeyPath)

		return sshPaths.PrivateKey, nil
	}); err != nil {
		return err
	}

	pr.Seal("tart artifacts ready")
	fmt.Println("  Run: cell build --engine=tart")
	return nil
}

// Run is the tart equivalent of the docker agent path. With a managed VM:
//
//  1. Acquire the VM (clone the template, or build it first if missing)
//  2. Boot it and wait for the guest agent
//  3. Mount the project directory via VirtioFS
//  4. Run the agent through `tart exec` (like docker exec)
//
// Two args belong to tart and are taken out of opts.Args: --force deletes
// the instance VM and the template first, so both are rebuilt, and
// --stack=<name> overrides opts.Cell.Stack. opts.Env, opts.Rebuild,
// opts.Update and opts.Background are not honored yet; the VM this call
// started is shut down when the agent exits.
func (Engine) Run(ctx context.Context, opts engine.RunOpts) error {
	binary, defaultFlags, userArgs := opts.Binary, opts.DefaultFlags, opts.Args
	cellCfg := opts.Cell.Config
	baseDir, hostHome, cellName := opts.Cell.BaseDir, opts.Cell.HostHome, opts.Cell.Name
	dryRun, background, debug := opts.DryRun, opts.Background, opts.Debug

	if err := cfg.ExpandEnv(cellCfg.Env, os.LookupEnv, nil); err != nil {
		return fmt.Errorf("%w", err)
	}

	if runtime.GOOS != "darwin" && !debug && !dryRun {
		return fmt.Errorf("tart engine requires macOS (use --debug to simulate on %s)", runtime.GOOS)
	}
	mock := runtime.GOOS != "darwin" && !dryRun

	logf := func(format string, args ...any) {
		if mock {
			fmt.Printf("[MOCK %s]: %s\n", runtime.GOOS, fmt.Sprintf(format, args...))
		} else {
			ux.Debugf("tart: "+format, args...)
		}
	}

	if mock {
		logf("runtime.GOOS=%s (not darwin) — entering mock mode", runtime.GOOS)
	}
	logf("binary=%q  defaultFlags=%v  userArgs=%v", binary, defaultFlags, userArgs)
	logf("cellName=%q  baseDir=%q  hostHome=%q", cellName, baseDir, hostHome)
	logf("background=%v  dryRun=%v  debug=%v", background, dryRun, debug)

	// --- env var assembly ---
	logf("assembling env vars to forward into VM")
	envVars := guestEnv(cellCfg, cellName)
	for _, kv := range envVars {
		logf("  env: %s", kv)
	}

	// The cell user matches the host's $USER, like Docker's HOST_USER.
	cellUser := os.Getenv("USER")
	if cellUser == "" {
		cellUser = "admin"
	}
	logf("managed VM mode — using tart exec (no SSH), cellUser=%s", cellUser)

	// --- consume --force and --stack from userArgs (tart-specific) ---
	force := false
	stackOverride := ""
	var filteredUserArgs []string
	for _, a := range userArgs {
		if a == "--force" {
			force = true
			continue
		}
		if strings.HasPrefix(a, "--stack=") {
			stackOverride = strings.TrimPrefix(a, "--stack=")
			continue
		}
		filteredUserArgs = append(filteredUserArgs, a)
	}
	userArgs = filteredUserArgs

	// --- lifecycle: acquire VM (real or mock) ---
	if !dryRun && !mock {
		stack := opts.Cell.Stack
		if stackOverride != "" {
			logf("--stack=%s overrides resolved stack %q", stackOverride, stack)
			stack = stackOverride
		}
		nixVolumePath, _ := EnsureNixVolume(hostHome)
		cellHome, _ := EnsureHomeDir(hostHome, cellName)
		var disks []string
		if nixVolumePath != "" {
			disks = append(disks, nixVolumePath)
		}
		instanceName := InstanceVMName(cellName)
		templateName := TemplateVMName(stack, nil)

		if force {
			logf("--force: stopping + deleting existing instance VM %s (if any)", instanceName)
			_ = exec.CommandContext(ctx, "tart", "stop", instanceName).Run()
			_ = exec.CommandContext(ctx, "tart", "delete", instanceName).Run()
			logf("--force: deleting existing template %s (if any)", templateName)
			_ = exec.CommandContext(ctx, "tart", "delete", templateName).Run()
		}

		acquireIn := AcquireInputs{
			VMName:       instanceName,
			TemplateName: templateName,
			SharedDirs: map[string]string{
				"project": baseDir,
				"home":    cellHome,
			},
			Disks:      disks,
			SSHTimeout: 120 * time.Second,
			InitFunc: func() error {
				logf("auto-build: VM not found — running build with stack=%q", stack)
				return build(ctx, buildParams{
					cellName:   cellName,
					hostHome:   hostHome,
					projectDir: baseDir,
					stack:      stack,
					ociImage:   cellCfg.Cell.ResolvedTartOCIImage(),
					stage:      "full",
				})
			},
		}
		acquireIn.ApplyDefaults()
		logf("acquiring VM: %s", instanceName)

		result, err := AcquireDarwinVM(ctx, acquireIn)
		if err != nil {
			logf("AcquireDarwinVM failed: %v", err)
			return fmt.Errorf("acquiring macOS VM: %w", err)
		}
		if result.Managed {
			logf("started managed VM — will shut down on exit")
			defer func() {
				logf("shutting down managed VM")
				if stopErr := result.VM.Stop(); stopErr != nil {
					logf("graceful shutdown failed: %v — forcing stop", stopErr)
					result.VM.ForceStop()
				}
			}()
		} else {
			logf("VM already running (not managed by this session)")
		}

		// Verify provisioning completed — retry a few times as tart exec
		// may fail immediately after boot while the guest agent starts.
		diagScript := `echo "whoami=$(whoami) home=$HOME"; ls -la /private/var/devcell-provisioned 2>&1; test -f /private/var/devcell-provisioned`
		var checkErr error
		for attempt := 1; attempt <= 10; attempt++ {
			checkCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", diagScript)
			var checkOut, checkStderr strings.Builder
			checkCmd.Stdout = &checkOut
			checkCmd.Stderr = &checkStderr
			checkErr = checkCmd.Run()
			if checkErr == nil {
				logf("provisioned marker check attempt %d/10: OK (stdout: %s)", attempt, strings.TrimSpace(checkOut.String()))
				break
			}
			logf("provisioned marker check attempt %d/10 failed: %v (stdout: %s) (stderr: %s)", attempt, checkErr, strings.TrimSpace(checkOut.String()), strings.TrimSpace(checkStderr.String()))
			time.Sleep(3 * time.Second)
		}
		if checkErr != nil {
			return fmt.Errorf("VM %s exists but provisioning is incomplete — run `cell build --engine=tart --force`", instanceName)
		}
		logf("provisioned marker verified")

		// Create the cell user matching host's $USER (like Docker's HOST_USER)
		logf("session user: %s", cellUser)

		if cellUser != "admin" {
			createUserScript := GenerateCreateSessionUserScript(cellUser)
			logf("creating session user %s in VM", cellUser)
			var cuOut, cuErr strings.Builder
			cuCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", createUserScript)
			cuCmd.Stdout = &cuOut
			cuCmd.Stderr = &cuErr
			if err := cuCmd.Run(); err != nil {
				logf("session user creation failed: %v (stdout: %s) (stderr: %s)", err, strings.TrimSpace(cuOut.String()), strings.TrimSpace(cuErr.String()))
				return fmt.Errorf("creating session user %s in VM: %w", cellUser, err)
			}
			logf("session user setup: %s", strings.TrimSpace(cuOut.String()))

			setupHomeScript := GenerateSetupSessionHomeScript(cellUser)
			var shOut, shErr strings.Builder
			shCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", setupHomeScript)
			shCmd.Stdout = &shOut
			shCmd.Stderr = &shErr
			if err := shCmd.Run(); err != nil {
				logf("session home setup failed: %v (stdout: %s) (stderr: %s)", err, strings.TrimSpace(shOut.String()), strings.TrimSpace(shErr.String()))
			} else {
				logf("session home: %s", strings.TrimSpace(shOut.String()))
			}
		}

		// --- Repair /nix shadow mount (pre-fix templates) ---
		// The installer's darwin-store daemon can mount its tiny APFS volume
		// over the JHFS+ nix disk at clone boot, hiding the nix-darwin system
		// profile and s6. Detect and repair before anything touches /nix.
		{
			repairScript := GenerateNixShadowRepairScript()
			var repOut, repErr strings.Builder
			repCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", repairScript)
			repCmd.Stdout = &repOut
			repCmd.Stderr = &repErr
			if err := repCmd.Run(); err != nil {
				logf("nix shadow repair failed: %v\nstdout: %s\nstderr: %s", err, strings.TrimSpace(repOut.String()), strings.TrimSpace(repErr.String()))
			} else {
				logf("nix shadow: %s", strings.TrimSpace(repOut.String()))
			}
		}

		// --- Activate s6 services for the cell user ---
		// Runs s6 oneshot services (shell-rc, claude-config, etc.) that set up
		// the cell user's environment.
		{
			s6Script := GenerateS6SessionActivateScript(cellUser)
			logf("activating s6 session services for %s (envDir=%s svcDir=%s)", cellUser, S6EnvDir, S6ServicesDir)
			logf("s6 script:\n%s", s6Script)
			var s6Out, s6Err strings.Builder
			s6Cmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", s6Script)
			s6Cmd.Stdout = &s6Out
			s6Cmd.Stderr = &s6Err
			if err := s6Cmd.Run(); err != nil {
				logf("s6 session activation failed: %v\nstdout: %s\nstderr: %s", err, strings.TrimSpace(s6Out.String()), strings.TrimSpace(s6Err.String()))
			} else {
				logf("s6 session: %s", strings.TrimSpace(s6Out.String()))
			}
		}

		// --- Re-activate nix-darwin if /run/current-system is missing ---
		// nix-darwin's boot activation (org.nixos.activate-system) may not
		// have run yet, or may fail if the nix disk wasn't mounted in time.
		// The system profile at /nix/var/nix/profiles/system/activate is the
		// canonical way to re-trigger activation.
		{
			checkScript := `echo "nix_mounted=$(mount | grep /nix | head -1 || echo NONE)"
echo "system_profile=$(ls -la /nix/var/nix/profiles/system 2>/dev/null || echo MISSING)"
echo "activate_bin=$(test -x /nix/var/nix/profiles/system/activate && echo EXISTS || echo MISSING)"
test -e /run/current-system && echo "RESULT=OK" || echo "RESULT=MISSING"`
			var checkOut strings.Builder
			checkCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", checkScript)
			checkCmd.Stdout = &checkOut
			checkErr := checkCmd.Run()
			if checkErr == nil {
				logf("nix-darwin preflight: %s", strings.TrimSpace(checkOut.String()))
			}
			if checkErr == nil && strings.Contains(checkOut.String(), "RESULT=MISSING") {
				logf("nix-darwin: /run/current-system missing — re-activating system profile")
				activateScript := `set -e
if [ -x /nix/var/nix/profiles/system/activate ]; then
  sudo /nix/var/nix/profiles/system/activate 2>&1
  echo "ACTIVATED"
  ls -la /run/current-system 2>&1 || echo "still missing after activate"
  ls /etc/profiles/per-user/devcell/bin/ 2>/dev/null | head -5 || echo "per-user bin still missing"
else
  echo "NO_PROFILE"
  ls -la /nix/var/nix/profiles/ 2>&1
fi`
				var actOut, actErr strings.Builder
				actCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", activateScript)
				actCmd.Stdout = &actOut
				actCmd.Stderr = &actErr
				if err := actCmd.Run(); err != nil {
					logf("nix-darwin activation failed: %v\nstdout: %s\nstderr: %s", err, strings.TrimSpace(actOut.String()), strings.TrimSpace(actErr.String()))
				} else {
					logf("nix-darwin activation: %s", strings.TrimSpace(actOut.String()))
				}
			} else if checkErr == nil {
				logf("nix-darwin: /run/current-system present — system already activated")
			}
		}

		// --- Diagnostic: verify nix-darwin and home-manager paths ---
		{
			diagScript := fmt.Sprintf(`echo "=== VM PATH DIAGNOSTICS ==="

echo "--- nix-daemon profile ---"
ls /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh 2>&1

echo "--- /run/current-system ---"
ls -la /run/current-system 2>&1 || echo "MISSING"

echo "--- nix-darwin system profile ---"
ls -la /nix/var/nix/profiles/system 2>&1 || echo "MISSING"

echo "--- /etc/profiles/per-user/%[1]s/bin (first 20) ---"
ls /etc/profiles/per-user/%[1]s/bin/ 2>/dev/null | head -20
test -d /etc/profiles/per-user/%[1]s/bin || echo "DIR NOT FOUND"

echo "--- which claude ---"
. /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh 2>/dev/null
export PATH="/etc/profiles/per-user/%[1]s/bin:/run/current-system/sw/bin:$PATH"
which claude 2>&1 || echo "NOT FOUND"

echo "--- which node ---"
which node 2>&1 || echo "NOT FOUND"

echo "--- nix-darwin generation ---"
ls -la /run/current-system 2>/dev/null || echo "NOT FOUND"

echo "--- home-manager generations for %[1]s ---"
ls -la /nix/var/nix/profiles/per-user/%[1]s/ 2>/dev/null || echo "NO PROFILES"

echo "--- session user home ---"
ls -la /Users/%[2]s/ 2>/dev/null | head -30

echo "--- .zshenv content ---"
cat /Users/%[2]s/.zshenv 2>/dev/null || echo "NO .zshenv"

echo "--- .zshenv target check ---"
if [ -L /Users/%[2]s/.zshenv ]; then
  echo "symlink -> $(readlink /Users/%[2]s/.zshenv)"
  echo "target exists: $(test -e /Users/%[2]s/.zshenv && echo YES || echo NO)"
fi

echo "--- nix /nix mount ---"
mount | grep /nix || echo "NOT MOUNTED"

echo "--- nix-daemon status ---"
sudo launchctl print system/org.nixos.nix-daemon 2>&1 | head -3 || echo "NOT LOADED"

echo "--- exec PATH (as session user) ---"
echo "$PATH"

echo "=== END DIAGNOSTICS ==="`,
				DarwinVMUser,
				cellUser)
			var diagOut, diagErr strings.Builder
			diagCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", diagScript)
			diagCmd.Stdout = &diagOut
			diagCmd.Stderr = &diagErr
			if err := diagCmd.Run(); err != nil {
				logf("VM diagnostics failed: %v\nstderr: %s", err, strings.TrimSpace(diagErr.String()))
			} else {
				logf("VM diagnostics:\n%s", diagOut.String())
			}
		}

		// Mount project directory inside the VM at the mirrored host path
		// (e.g. /Users/dmitry/dev/acme/proj on the host appears at the same
		// path in the VM), falling back to ~/<basename> for projects outside
		// the host home.
		projectPathVM := ProjectPathInVM(hostHome, baseDir, cellUser)
		mountScript := GenerateProjectMountScript("project", cellUser, projectPathVM)
		logf("mounting project dir: tag=project user=%s target=%s", cellUser, projectPathVM)
		var mountStderr strings.Builder
		mountCmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", mountScript)
		mountCmd.Stderr = &mountStderr
		if err := mountCmd.Run(); err != nil {
			logf("project mount failed: %v (stderr: %s)", err, strings.TrimSpace(mountStderr.String()))
			return fmt.Errorf("mounting project directory in VM: %w (stderr: %s)", err, strings.TrimSpace(mountStderr.String()))
		}
		logf("project directory mounted at %s", projectPathVM)
	}

	// --- lifecycle: simulated VM start (mock only) ---
	if mock {
		logf("[tart] preflight: GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)
		logf("[tart] tart run %s → start VM + wait for guest agent", InstanceVMName(cellName))
		logf("[tart] guest agent ready (simulated)")
	}

	instanceName := InstanceVMName(cellName)

	// --- build the inner command for tart exec ---
	runAsUser := ""
	if cellUser != "admin" {
		runAsUser = cellUser
	}
	execCmd := BuildExecCommand(ExecSpec{
		Binary:     binary,
		Flags:      defaultFlags,
		UserArgs:   userArgs,
		EnvVars:    envVars,
		ProjectDir: baseDir,
		WorkDir:    ProjectPathInVM(hostHome, baseDir, cellUser),
		RunAsUser:  runAsUser,
	})
	logf("exec command: %s", execCmd)

	displayCmd := cell.ShellJoin([]string{"tart", "exec", instanceName, "bash", "-l", "-c", execCmd})
	if dryRun {
		fmt.Println(displayCmd)
		return nil
	}

	if mock {
		logf("would exec: %s", displayCmd)
		logf("skipping exec (mock mode) — on darwin this would open an interactive session in %s",
			instanceName)
		return nil
	}

	// --- exec into VM via tart exec (like docker exec -it) ---
	logf("tart exec -t -i %s bash -l -c ...", instanceName)
	cmd := exec.CommandContext(ctx, "tart", "exec", "-t", "-i", instanceName, "bash", "-l", "-c", execCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &engine.ExitError{Code: exitErr.ExitCode()}
		}
		return err
	}
	return nil
}

// guestEnv collects the env vars forwarded into the macOS VM via
// `tart exec`: the host TERM plus the cell env shared by every runtime.
func guestEnv(cellCfg cfg.CellConfig, cellName string) []string {
	var envs []string
	if term := os.Getenv("TERM"); term != "" {
		envs = append(envs, "TERM="+term)
	}
	return append(envs, cell.GuestEnv(cell.EnvInput{
		CellName:  cellName,
		Git:       cellCfg.Git,
		Timezone:  cellCfg.Cell.Timezone,
		Locale:    cellCfg.Cell.Locale,
		Env:       cellCfg.Env,
		Mise:      cellCfg.Mise,
		Getenv:    os.Getenv,
		GitConfig: cell.HostGitConfig,
	})...)
}

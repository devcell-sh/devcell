package tart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/s6"
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
			NoGraphics: !cellCfg.GUI.ResolvedEnabled(),
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

		// --- Helper: run a script in the VM with PhaseRunner visibility ---
		// When --debug is active, output streams live to the terminal.
		tartExec := func(script string) (string, string, error) {
			var stdout, stderr strings.Builder
			cmd := exec.CommandContext(ctx, "tart", "exec", instanceName, "bash", "-l", "-c", script)
			if ux.Verbose {
				cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
				cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
			} else {
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
			}
			err := cmd.Run()
			return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
		}

		var pr ux.PhaseRunner

		// Verify provisioning completed — retry a few times as tart exec
		// may fail immediately after boot while the guest agent starts.
		if err := pr.Phase("Verifying provisioned marker", func() error {
			diagScript := `echo "whoami=$(whoami) home=$HOME"; ls -la /private/var/devcell-provisioned 2>&1; test -f /private/var/devcell-provisioned`
			var checkErr error
			for attempt := 1; attempt <= 10; attempt++ {
				out, errOut, err := tartExec(diagScript)
				checkErr = err
				if checkErr == nil {
					logf("provisioned marker check attempt %d/10: OK (stdout: %s)", attempt, out)
					break
				}
				logf("provisioned marker check attempt %d/10 failed: %v (stdout: %s) (stderr: %s)", attempt, checkErr, out, errOut)
				time.Sleep(3 * time.Second)
			}
			if checkErr != nil {
				return fmt.Errorf("VM %s exists but provisioning is incomplete — run `cell build --engine=tart --force`", instanceName)
			}
			return nil
		}); err != nil {
			return err
		}

		// Set hostname to cell name so `hostname -s` returns it (used by
		// the nix home-manager set-wallpaper activation for CELL_ID).
		if err := pr.Phase("Set hostname", func() error {
			script := GenerateSetHostnameScript(cellName)
			out, errOut, err := tartExec(script)
			if err != nil {
				logf("set hostname failed: %v (stdout: %s) (stderr: %s)", err, out, errOut)
			} else {
				logf("hostname: %s", out)
			}
			return nil
		}); err != nil {
			return err
		}

		// Create the cell user matching host's $USER (like Docker's HOST_USER)
		if cellUser != "admin" {
			if err := pr.Phase("Session user setup", func() error {
				logf("session user: %s", cellUser)
				createUserScript := GenerateCreateSessionUserScript(cellUser)
				out, errOut, err := tartExec(createUserScript)
				if err != nil {
					logf("session user creation failed: %v (stdout: %s) (stderr: %s)", err, out, errOut)
					return fmt.Errorf("creating session user %s in VM: %w", cellUser, err)
				}
				logf("session user setup: %s", out)

				setupHomeScript := GenerateSetupSessionHomeScript(cellUser)
				out, errOut, err = tartExec(setupHomeScript)
				if err != nil {
					logf("session home setup failed: %v (stdout: %s) (stderr: %s)", err, out, errOut)
				} else {
					logf("session home: %s", out)
				}
				return nil
			}); err != nil {
				return err
			}
		}

		// Repair /nix shadow mount (pre-fix templates).
		// The installer's darwin-store daemon can mount its tiny APFS volume
		// over the JHFS+ nix disk at clone boot, hiding the nix-darwin system
		// profile and s6. Detect and repair before anything touches /nix.
		if err := pr.Phase("Nix shadow repair", func() error {
			repairScript := GenerateNixShadowRepairScript()
			out, errOut, err := tartExec(repairScript)
			if err != nil {
				logf("nix shadow repair failed: %v\nstdout: %s\nstderr: %s", err, out, errOut)
			} else {
				logf("nix shadow: %s", out)
			}
			return nil
		}); err != nil {
			return err
		}

		// Activate s6 services for the cell user via s6-rc.
		if err := pr.Phase("Activate s6 session services", func() error {
			s6Script := GenerateS6SessionActivateScript(cellUser)
			logf("activating s6 session services for %s (envDir=%s compiledDir=%s)", cellUser, s6.EnvDir, s6.CompiledDir)
			logf("s6 script:\n%s", s6Script)
			out, errOut, err := tartExec(s6Script)
			if err != nil {
				logf("s6 session activation failed: %v\nstdout: %s\nstderr: %s", err, out, errOut)
			} else {
				logf("s6 session: %s", out)
			}
			return nil
		}); err != nil {
			return err
		}

		// Re-activate nix-darwin if /run/current-system is missing.
		// nix-darwin's boot activation (org.nixos.activate-system) may not
		// have run yet, or may fail if the nix disk wasn't mounted in time.
		if err := pr.PhaseDetailed("Activate nix-darwin", func() (string, error) {
			checkScript := `echo "nix_mounted=$(mount | grep /nix | head -1 || echo NONE)"
echo "system_profile=$(ls -la /nix/var/nix/profiles/system 2>/dev/null || echo MISSING)"
echo "activate_bin=$(test -x /nix/var/nix/profiles/system/activate && echo EXISTS || echo MISSING)"
test -e /run/current-system && echo "RESULT=OK" || echo "RESULT=MISSING"`
			checkOut, _, checkErr := tartExec(checkScript)
			if checkErr == nil {
				logf("nix-darwin preflight: %s", checkOut)
			}
			if checkErr == nil && strings.Contains(checkOut, "RESULT=MISSING") {
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
				out, errOut, err := tartExec(activateScript)
				if err != nil {
					logf("nix-darwin activation failed: %v\nstdout: %s\nstderr: %s", err, out, errOut)
				} else {
					logf("nix-darwin activation: %s", out)
				}
				return "re-activated", nil
			} else if checkErr == nil {
				logf("nix-darwin: /run/current-system present — system already activated")
				return "already active", nil
			}
			return "", nil
		}); err != nil {
			return err
		}

		// Diagnostic: verify nix-darwin and home-manager paths.
		if err := pr.Phase("VM diagnostics", func() error {
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
			out, errOut, err := tartExec(diagScript)
			if err != nil {
				logf("VM diagnostics failed: %v\nstderr: %s", err, errOut)
			} else {
				logf("VM diagnostics:\n%s", out)
			}
			return nil
		}); err != nil {
			return err
		}

		// Mount project directory inside the VM at the mirrored host path.
		projectPathVM := ProjectPathInVM(hostHome, baseDir, cellUser)
		if err := pr.PhaseDetailed("Mount project directory", func() (string, error) {
			mountScript := GenerateProjectMountScript("project", cellUser, projectPathVM)
			logf("mounting project dir: tag=project user=%s target=%s", cellUser, projectPathVM)
			_, errOut, err := tartExec(mountScript)
			if err != nil {
				logf("project mount failed: %v (stderr: %s)", err, errOut)
				return "", fmt.Errorf("mounting project directory in VM: %w (stderr: %s)", err, errOut)
			}
			logf("project directory mounted at %s", projectPathVM)
			return projectPathVM, nil
		}); err != nil {
			return err
		}
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

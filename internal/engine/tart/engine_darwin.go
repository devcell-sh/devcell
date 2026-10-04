//go:build darwin && arm64

package tart

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/nixhome"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/version"
)

// Cirrus Labs OCI images ship with this default user/password.
const (
	imageUser     = "admin"
	imagePassword = "admin"
)

// Init prepares the local artifact directory and SSH keypair for the cell's
// VM, like Docker init: config and keys, no images, no VMs. Build creates
// the VM. opts.Stack overrides opts.Cell.Stack; opts.Force replaces an
// existing keypair.
func (Engine) Init(_ context.Context, opts engine.InitOpts) error {
	return prepareHost(opts.Cell.Name, opts.Cell.HostHome, opts.Cell.WithStack(opts.Stack).Stack, opts.Force)
}

// build creates a fully provisioned macOS VM template from an OCI image.
//
// Mirrors the Docker build flow: init scaffolds config/keys (no images),
// build creates and provisions the image. The VM is booted for provisioning
// and shut down when done; Run starts it again for the agent.
func build(ctx context.Context, p buildParams) error {
	cellName, hostHome, projectDir, stack := p.cellName, p.hostHome, p.projectDir, p.stack
	force, noCache, stage := p.force, p.noCache, p.stage

	bc := BuildConfig{
		CellName: cellName,
		HomeDir:  hostHome,
		Stack:    stack,
	}
	bc.ApplyDefaults()
	if err := bc.Validate(); err != nil {
		return err
	}

	// Determine template name and clone source based on stage.
	var templateName string
	var cloneSource string
	var cloneFromBase bool

	switch stage {
	case "base":
		templateName = BaseTemplateName
		cloneSource = p.ociImage
	default: // "full"
		templateName = TemplateVMName(stack, nil)
		if _, err := TartGet(ctx, BaseTemplateName); err == nil {
			cloneSource = BaseTemplateName
			cloneFromBase = true
			ux.Debugf("base template %s found: will clone locally (fast path)", BaseTemplateName)
		} else {
			cloneSource = p.ociImage
			ux.Debugf("no base template: full build from OCI")
		}
	}

	buildVM := "devcell-build-tmp"

	nixhomeRef := nixhome.ResolveNixhomeRef(version.Version)

	ux.Debugf("build config: cell=%s stack=%s stage=%s cpus=%d mem=%dGB sshPort=%d",
		bc.CellName, bc.Stack, stage, bc.CPUs, bc.MemoryGB, bc.SSHPort)
	ux.Debugf("template: %s  cloneSource: %s  buildVM: %s  force=%v noCache=%v", templateName, cloneSource, buildVM, force, noCache)
	ux.Debugf("nixhome: %s  projectDir: %s", nixhomeRef, projectDir)

	if p.dryRun {
		fmt.Printf("Would build macOS VM template: %s\n", templateName)
		fmt.Printf("  Stage: %s\n", stage)
		fmt.Printf("  Clone source: %s\n", cloneSource)
		fmt.Printf("  Stack: %s\n", bc.Stack)
		fmt.Printf("  CPUs: %d  Memory: %dGB\n", bc.CPUs, bc.MemoryGB)
		return nil
	}

	pr := &ux.PhaseRunner{}

	// --- Phase 1: Ensure SSH keys exist ---
	sshPaths := NewCellSSHPaths(hostHome, cellName)
	if _, err := os.Stat(sshPaths.PrivateKey); err != nil {
		ux.Debugf("SSH key not found at %s — running auto-init", sshPaths.PrivateKey)
		fmt.Println(ux.StyleSection.Render(" SSH keys not found — running init"))
		if initErr := prepareHost(cellName, hostHome, stack, false); initErr != nil {
			return fmt.Errorf("auto-init failed: %w", initErr)
		}
	}

	// Assemble the public key(s) to inject into the VM's authorized_keys.
	// Primary: the generated per-cell key. Additional: any existing ~/.ssh/*.pub.
	pubKeyBytes, err := os.ReadFile(sshPaths.PublicKey)
	if err != nil {
		return fmt.Errorf("reading SSH public key from %s: %w", sshPaths.PublicKey, err)
	}
	pubKey := strings.TrimSpace(string(pubKeyBytes))
	ux.Debugf("loaded SSH pub key from %s", sshPaths.PublicKey)

	if homeDir, _ := os.UserHomeDir(); homeDir != "" {
		if extra := CollectSSHPubKeys(filepath.Join(homeDir, ".ssh")); extra != "" {
			pubKey = pubKey + "\n" + extra
			ux.Debugf("added existing ~/.ssh pub keys")
		}
	}

	// --- Platform compatibility preflight ---
	if err := pr.PhaseDetailed("Platform compatibility check", func() (string, error) {
		flakeRef := nixhome.ResolveNixhomeRef(version.Version)
		if err := nixhome.PreflightPlatformCheck(ctx, flakeRef, "aarch64-darwin"); err != nil {
			return "", err
		}
		return "aarch64-darwin OK", nil
	}); err != nil {
		return err
	}

	// --- Ensure nix store volume (global) ---
	var nixVolumePath string
	if err := pr.PhaseDetailed("Preparing nix store volume", func() (string, error) {
		path, err := EnsureNixVolume(hostHome)
		if err != nil {
			return "", err
		}
		nixVolumePath = path
		return path, nil
	}); err != nil {
		return err
	}

	// --- Ensure home directory (per-cell, VirtioFS mount) ---
	var cellHome string
	if err := pr.PhaseDetailed("Preparing home directory", func() (string, error) {
		path, err := EnsureHomeDir(hostHome, cellName)
		if err != nil {
			return "", err
		}
		cellHome = path
		return path, nil
	}); err != nil {
		return err
	}

	// --- Phase 2: Clone source → build VM ---
	cloneLabel := "Cloning VM from OCI image"
	if cloneFromBase {
		cloneLabel = "Cloning VM from base template"
	}
	if err := pr.PhaseDetailed(cloneLabel, func() (string, error) {
		if _, getErr := TartGet(ctx, templateName); getErr == nil {
			if !force {
				return "", fmt.Errorf("template %s already exists — use --force to rebuild", templateName)
			}
			ux.Debugf("template %s exists, --force — deleting", templateName)
			_ = TartStop(ctx, templateName)
			_ = TartDelete(ctx, templateName)
		}

		// Clean up any leftover build VM from a previous interrupted build.
		if _, getErr := TartGet(ctx, buildVM); getErr == nil {
			ux.Debugf("stale build VM %s found — deleting", buildVM)
			_ = TartStop(ctx, buildVM)
			_ = TartDelete(ctx, buildVM)
		}

		ux.Debugf("cloning %s → %s (noCache=%v)", cloneSource, buildVM, noCache)
		args := []string{"clone"}
		if noCache && !cloneFromBase {
			args = append(args, "--no-cache")
		}
		args = append(args, cloneSource, buildVM)
		cmd := exec.CommandContext(ctx, "tart", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("tart clone: %w", err)
		}
		return buildVM, nil
	}); err != nil {
		return err
	}

	cleanupVM := func() {
		ux.Debugf("cleanup: deleting build VM %s", buildVM)
		_ = TartDelete(ctx, buildVM)
	}

	// --- Phase 3: Configure resources ---
	if err := pr.PhaseDetailed("Configuring VM resources", func() (string, error) {
		memMB := bc.MemoryGB * 1024
		ux.Debugf("tart set %s: cpu=%d memory=%dMB", buildVM, bc.CPUs, memMB)
		if err := TartSet(ctx, buildVM, bc.CPUs, memMB); err != nil {
			return "", fmt.Errorf("setting VM resources: %w", err)
		}
		return fmt.Sprintf("%d CPUs, %dGB memory", bc.CPUs, bc.MemoryGB), nil
	}); err != nil {
		cleanupVM()
		return err
	}

	// --- Pre-boot diagnostics (host side) ---
	ux.Debugf("tart version: %s", TartVersion(ctx))
	{
		helpOut, _ := exec.CommandContext(ctx, "tart", "run", "--help").CombinedOutput()
		ux.Debugf("tart run --help:\n%s", string(helpOut))
	}
	{
		getOut, _ := exec.CommandContext(ctx, "tart", "get", buildVM).CombinedOutput()
		ux.Debugf("tart get %s (pre-boot):\n%s", buildVM, string(getOut))
	}
	// --- Detect host nix store for caching ---
	hostNixPath := DetectHostNixStore()
	if hostNixPath != "" {
		ux.Debugf("host nix store detected at %s (valid: store/ + db.sqlite present) — will share as read-only substituter", hostNixPath)
	} else {
		ux.Debugf("no host nix store found at /nix — VM will download from cache.nixos.org")
	}

	// --- Phase 4: Boot VM ---
	sharedDirs := map[string]string{
		"nixhome": nixhomeRef,
		"home":    cellHome,
	}
	if hostNixPath != "" {
		sharedDirs["hostnix"] = hostNixPath + ":ro"
	}
	disks := []string{nixVolumePath}
	ux.Debugf("booting VM %s with shared dirs: %v, disks: %v", buildVM, sharedDirs, disks)

	var vm *VM
	if err := pr.PhaseDetailed("Booting VM", func() (string, error) {
		var bootErr error
		vm, bootErr = TartRun(ctx, buildVM, sharedDirs, disks)
		if bootErr != nil {
			return "", fmt.Errorf("starting VM: %w", bootErr)
		}
		return buildVM, nil
	}); err != nil {
		cleanupVM()
		return err
	}

	stopVM := func() {
		ux.Debugf("stopping VM %s", buildVM)
		if err := vm.Stop(); err != nil {
			ux.Debugf("graceful stop failed: %v — forcing", err)
			vm.ForceStop()
		}
	}

	// --- Phase 5: Wait for guest agent ---
	if err := pr.PhaseDetailed("Waiting for guest agent", func() (string, error) {
		ux.Debugf("waiting for tart guest agent (timeout 120s)")
		if err := waitForGuestAgent(ctx, buildVM, 120*time.Second); err != nil {
			return "", err
		}
		return "ready", nil
	}); err != nil {
		stopVM()
		return err
	}

	// --- Phase 6: Bootstrap passwordless sudo (skip when cloning from base) ---
	if !cloneFromBase {
		if err := pr.PhaseDetailed("Bootstrapping passwordless sudo", func() (string, error) {
			bootstrapCmd := fmt.Sprintf(
				"echo '%s' | sudo -S sh -c \"mkdir -p /etc/sudoers.d && echo '%s ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/%s && chmod 440 /etc/sudoers.d/%s\"",
				imagePassword, imageUser, imageUser, imageUser,
			)
			ux.Debugf("bootstrap: configuring passwordless sudo for %s", imageUser)
			if err := TartExec(ctx, buildVM, []string{"bash", "-l", "-c", bootstrapCmd}, os.Stdout, os.Stderr); err != nil {
				return "", fmt.Errorf("bootstrap sudo: %w", err)
			}
			return imageUser, nil
		}); err != nil {
			stopVM()
			return err
		}
	} else {
		ux.Debugf("skipping sudo bootstrap: base template already has passwordless sudo")
	}

	// --- Diagnostic: host-side post-boot checks ---
	{
		if tartErr := vm.Stderr(); tartErr != "" {
			ux.Debugf("tart run stderr (post-boot): %s", strings.TrimSpace(tartErr))
		} else {
			ux.Debugf("tart run stderr (post-boot): (empty — no errors)")
		}
		getOut, _ := exec.CommandContext(ctx, "tart", "get", buildVM).CombinedOutput()
		ux.Debugf("tart get %s (post-boot):\n%s", buildVM, string(getOut))
		listOut, _ := exec.CommandContext(ctx, "tart", "list").CombinedOutput()
		ux.Debugf("tart list:\n%s", string(listOut))
	}

	// --- Diagnostic: guest-side VirtioFS and IO checks ---
	{
		diagScript := `echo "=== GUEST DIAGNOSTICS ==="
echo "--- uname ---"
uname -a
echo "--- diskutil list ---"
diskutil list
echo "--- mount ---"
mount
echo "--- /Volumes/ listing ---"
ls -la /Volumes/ 2>&1
echo "--- /Volumes/My Shared Files/ ---"
ls -la "/Volumes/My Shared Files/" 2>&1 || echo "NOT FOUND"
echo "--- /Volumes/My Shared Files/nixhome/ ---"
ls -la "/Volumes/My Shared Files/nixhome/" 2>&1 | head -10 || echo "NOT FOUND"
echo "=== END GUEST DIAGNOSTICS ==="`
		var diagOut, diagErr strings.Builder
		diagCmd := exec.CommandContext(ctx, "tart", "exec", buildVM, "bash", "-l", "-c", diagScript)
		diagCmd.Stdout = &diagOut
		diagCmd.Stderr = &diagErr
		if err := diagCmd.Run(); err != nil {
			ux.Debugf("guest diagnostics failed: %v\nstdout: %s\nstderr: %s",
				err, diagOut.String(), diagErr.String())
		} else {
			ux.Debugf("guest diagnostics:\n%s", diagOut.String())
		}
	}

	// --- Phase 7: Provisioning via tart exec ---
	initCfg := InitConfig{
		CellName:   cellName,
		HomeDir:    hostHome,
		Stack:      stack,
		Username:   imageUser,
		Password:   imagePassword,
		HasHostNix: hostNixPath != "",
	}
	initCfg.ApplyDefaults()

	var steps []ProvisionStep
	switch {
	case stage == "base":
		steps = BaseProvisionSteps(initCfg, pubKey)
	case cloneFromBase:
		steps = StackProvisionSteps(initCfg)
	default:
		steps = ProvisionSteps(initCfg, pubKey, false)
	}
	ux.Debugf("provisioning: %d steps via tart exec (stage=%s, cloneFromBase=%v)", len(steps), stage, cloneFromBase)

	const reformatMarker = "DEVCELL_REFORMAT_NEEDED:"
	for i, step := range steps {
		stepName := step.Name
		stepIdx := i
		stepCmd := step.Command
		label := fmt.Sprintf("Provisioning (%d/%d): %s", stepIdx+1, len(steps), stepName)
		streamOutput := ux.Verbose && strings.HasPrefix(stepName, "Activate nix-darwin")
		var reformatVolLabel string // set by callback if volume needs reformatting
		err := pr.PhaseDetailed(label, func() (string, error) {
			ux.Debugf("provision [%d/%d] %s", stepIdx+1, len(steps), stepName)
			cmd := exec.CommandContext(ctx, "tart", "exec", buildVM, "bash", "-l", "-c", stepCmd)
			var stdout, stderr strings.Builder
			if streamOutput {
				cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
				cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
			} else {
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
			}
			if err := cmd.Run(); err != nil {
				outStr := strings.TrimSpace(stdout.String())
				ux.Debugf("provision [%d/%d] %s FAILED: %v\nstdout: %s\nstderr: %s",
					stepIdx+1, len(steps), stepName, err,
					outStr,
					strings.TrimSpace(stderr.String()))
				if idx := strings.Index(outStr, reformatMarker); idx >= 0 {
					rest := outStr[idx+len(reformatMarker):]
					reformatVolLabel = strings.TrimSpace(strings.SplitN(rest, "\n", 2)[0])
					return "", fmt.Errorf("volume %q needs reformatting", reformatVolLabel)
				}
				errDetail := strings.TrimSpace(stderr.String())
				return "", fmt.Errorf("%s: %w (stderr: %s)", stepName, err, errDetail)
			}
			if out := strings.TrimSpace(stdout.String()); out != "" {
				ux.Debugf("provision [%d/%d] %s output: %s", stepIdx+1, len(steps), stepName, out)
			}
			ux.Debugf("provision [%d/%d] %s OK", stepIdx+1, len(steps), stepName)
			return "", nil
		})
		if err != nil && reformatVolLabel != "" {
			var imgPath string
			if reformatVolLabel == NixVolumeLabel {
				imgPath = NixVolumePath(hostHome)
			}
			msg := fmt.Sprintf(
				"Volume %q (%s) cannot be mounted — APFS encryption keys changed after VM rebuild.\nReformat? Previous contents will be lost.",
				reformatVolLabel, imgPath)
			confirmed, confirmErr := ux.GetConfirmation(msg)
			if confirmErr != nil {
				stopVM()
				return fmt.Errorf("confirmation prompt: %w", confirmErr)
			}
			if !confirmed {
				stopVM()
				return fmt.Errorf("volume %q needs reformatting — declined by user", reformatVolLabel)
			}
			retryLabel := fmt.Sprintf("Provisioning (%d/%d): %s (reformatting)", stepIdx+1, len(steps), stepName)
			if retryErr := pr.PhaseDetailed(retryLabel, func() (string, error) {
				reformatCmd := "export DEVCELL_ALLOW_REFORMAT=1; " + stepCmd
				cmd2 := exec.CommandContext(ctx, "tart", "exec", buildVM, "bash", "-l", "-c", reformatCmd)
				var stdout2, stderr2 strings.Builder
				cmd2.Stdout = &stdout2
				cmd2.Stderr = &stderr2
				if err2 := cmd2.Run(); err2 != nil {
					ux.Debugf("provision [%d/%d] %s reformat FAILED: %v\nstdout: %s\nstderr: %s",
						stepIdx+1, len(steps), stepName, err2,
						strings.TrimSpace(stdout2.String()),
						strings.TrimSpace(stderr2.String()))
					return "", fmt.Errorf("%s (reformat): %w (stderr: %s)", stepName, err2, strings.TrimSpace(stderr2.String()))
				}
				if out := strings.TrimSpace(stdout2.String()); out != "" {
					ux.Debugf("provision [%d/%d] %s reformat output: %s", stepIdx+1, len(steps), stepName, out)
				}
				ux.Debugf("provision [%d/%d] %s OK (after reformat)", stepIdx+1, len(steps), stepName)
				return "reformatted", nil
			}); retryErr != nil {
				stopVM()
				return retryErr
			}
			continue
		}
		if err != nil {
			stopVM()
			return err
		}
	}

	// --- Phase 7b: Stamp provisioned marker ---
	if err := pr.PhaseDetailed("Stamping provisioned marker", func() (string, error) {
		markerScript := GenerateProvisionedMarkerScript()
		ux.Debugf("marker script: %s", markerScript)
		var stdout, stderr strings.Builder
		cmd := exec.CommandContext(ctx, "tart", "exec", buildVM, "bash", "-l", "-c", markerScript)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			ux.Debugf("failed to stamp provisioned marker: %v (stdout: %s) (stderr: %s)",
				err, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
			return "", fmt.Errorf("stamp provisioned marker: %w", err)
		}
		ux.Debugf("provisioned marker stamped (stdout: %s)", strings.TrimSpace(stdout.String()))
		return "", nil
	}); err != nil {
		stopVM()
		return err
	}

	// --- Phase 7c: Verify marker + flush disk before shutdown ---
	if err := pr.PhaseDetailed("Verifying provisioned marker", func() (string, error) {
		verifyScript := fmt.Sprintf(`ls -la %s && stat -f '%%z bytes, %%m mtime' %s && sudo sync`,
			ProvisionedMarkerPath, ProvisionedMarkerPath)
		ux.Debugf("verify script: %s", verifyScript)
		var stdout, stderr strings.Builder
		cmd := exec.CommandContext(ctx, "tart", "exec", buildVM, "bash", "-l", "-c", verifyScript)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			ux.Debugf("marker verification FAILED: %v (stdout: %s) (stderr: %s)",
				err, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
			return "", fmt.Errorf("provisioned marker missing after stamp: %w", err)
		}
		ux.Debugf("marker verified before shutdown: %s", strings.TrimSpace(stdout.String()))
		return "verified", nil
	}); err != nil {
		stopVM()
		return err
	}

	// --- Phase 8: Shutdown build VM ---
	if err := pr.PhaseDetailed("Shutting down build VM", func() (string, error) {
		ux.Debugf("shutting down build VM %s", buildVM)
		if err := vm.Stop(); err != nil {
			ux.Debugf("graceful shutdown failed: %v — forcing", err)
			vm.ForceStop()
		}
		return buildVM, nil
	}); err != nil {
		return err
	}

	// --- Phase 9: Save as template ---
	if err := pr.PhaseDetailed("Saving template image", func() (string, error) {
		ux.Debugf("cloning build VM %s → template %s", buildVM, templateName)
		if err := TartClone(ctx, buildVM, templateName); err != nil {
			return "", fmt.Errorf("saving template: %w", err)
		}
		ux.Debugf("deleting build VM %s", buildVM)
		_ = TartDelete(ctx, buildVM)
		return templateName, nil
	}); err != nil {
		return err
	}

	pr.Seal(fmt.Sprintf("tart template %s built", templateName))
	return nil
}

// waitForGuestAgent polls `tart exec` until the guest agent responds.
func waitForGuestAgent(ctx context.Context, vmName string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var lastErr error
	var lastStderr string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("tart guest agent not ready after %s (last error: %v, stderr: %s)", timeout, lastErr, lastStderr)
		case <-ticker.C:
			var stderr strings.Builder
			cmd := exec.CommandContext(ctx, "tart", "exec", vmName, "echo", "ready")
			cmd.Stdout = nil
			cmd.Stderr = &stderr
			lastErr = cmd.Run()
			lastStderr = strings.TrimSpace(stderr.String())
			if lastErr == nil {
				ux.Debugf("guest agent responsive")
				return nil
			}
			ux.Debugf("guest agent not ready yet: %v (stderr: %s)", lastErr, lastStderr)
		}
	}
}

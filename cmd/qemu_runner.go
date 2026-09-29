package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	winkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/gosshd"
	vmapi "github.com/devcell-sh/go-winkit/vm"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/ux"
	"github.com/DimmKirr/devcell/internal/vm/qemu"
)

// winkitStartFunc is the function used to start PE VMs. Defaults to
// winkit.Start; tests replace it to avoid booting a real VM.
var winkitStartFunc = winkit.Start

// runQemuAgent boots a PE+WSL1+Nix VM via go-winkit and connects to it
// through gosshd for command execution.
//
// Lifecycle:
//  1. Acquire VM (auto-build template if missing)
//  2. Boot VM via winkit.Start, wait for gosshd SSH
//  3. Execute user command inside WSL1 via gosshd
//
// On non-darwin with --debug: mock/simulate every step with [MOCK] prefix.
// On darwin with --debug: real execution with ux.Debugf logging.
func runQemuAgent(
	binary string,
	defaultFlags, userArgs []string,
	cellCfg cfg.CellConfig,
	baseDir, hostHome, cellName string,
	dryRun, background, debug bool,
) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if runtime.GOOS != "darwin" && !debug && !dryRun {
		return fmt.Errorf("qemu engine requires macOS (use --debug to simulate on %s)", runtime.GOOS)
	}
	mock := runtime.GOOS != "darwin" && !dryRun

	logf := func(format string, args ...any) {
		if mock {
			fmt.Printf("[MOCK %s]: %s\n", runtime.GOOS, fmt.Sprintf(format, args...))
		} else {
			ux.Debugf("qemu: "+format, args...)
		}
	}

	if mock {
		logf("runtime.GOOS=%s (not darwin): entering mock mode", runtime.GOOS)
	}
	logf("binary=%q  defaultFlags=%v  userArgs=%v", binary, defaultFlags, userArgs)
	logf("cellName=%q  baseDir=%q  hostHome=%q", cellName, baseDir, hostHome)
	logf("background=%v  dryRun=%v  debug=%v", background, dryRun, debug)

	// --- env var assembly ---
	logf("assembling env vars to forward into VM")
	envVars := buildQemuEnvVars(cellCfg, cellName)
	for _, kv := range envVars {
		logf("  env: %s", kv)
	}

	// --- consume --force and --stack from userArgs (qemu-specific) ---
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

	stack := cellCfg.Cell.ResolvedStack()
	if stackOverride != "" {
		logf("--stack=%s overrides resolved stack %q", stackOverride, stack)
		stack = stackOverride
	}

	// --- port allocation ---
	c := config.Load(baseDir, os.Getenv)
	taken := config.DockerAllocatedPorts()
	ports := qemu.AllocatePorts(c.PortPrefix, taken)

	sshPort := ports.SSHPortUint16()
	if cellCfg.Cell.QemuSSHPort > 0 || os.Getenv("DEVCELL_QEMU_SSH_PORT") != "" {
		sshPort = uint16(cellCfg.Cell.ResolvedQemuSSHPort())
	}
	rdpPort := ports.RDPPortUint16()

	logf("ports: ssh=%d rdp=%d vnc=%s", sshPort, rdpPort, ports.VNCPort)

	// --- resolve PE image path ---
	imagePath := peImagePath(hostHome, stack, cellCfg.Cell.Modules)
	logf("PE image path: %s", imagePath)

	// --- dry-run: print what would happen and exit ---
	if dryRun {
		cmd := peGuestCommand(userArgs)
		fmt.Printf("PE+WSL1 runner (dry-run)\n")
		fmt.Printf("  image:   %s\n", imagePath)
		fmt.Printf("  ssh:     127.0.0.1:%d\n", sshPort)
		fmt.Printf("  command: %s\n", cmd)
		return nil
	}

	// --- lifecycle: acquire VM (real or mock) ---
	if !mock {
		// Auto-build if PE image is missing
		if _, err := os.Stat(imagePath); err != nil {
			if !force {
				fmt.Printf("PE+WSL1 image not found at %s: a build is required.\n", imagePath)
				ok, promptErr := ux.GetConfirmation("Build now?")
				if promptErr != nil {
					return fmt.Errorf("prompt: %w", promptErr)
				}
				if !ok {
					return fmt.Errorf("build declined: run `cell build --engine=qemu` manually")
				}
			}
			logf("auto-build: PE image missing, building with stack=%q", stack)
			if err := runBuildQemu(cellName, hostHome, baseDir, stack, false, false, false, cellCfg.Cell); err != nil {
				return fmt.Errorf("auto-build failed: %w", err)
			}
		}

		// Boot PE VM via winkit
		startOpts := peStartOpts(cellName, hostHome, stack, cellCfg.Cell, sshPort, rdpPort)
		startOpts.Accel = os.Getenv("DEVCELL_QEMU_ACCEL")
		logf("starting PE VM: image=%s cpus=%d mem=%dGB ssh=%d rdp=%d", startOpts.Image, startOpts.CPUs, startOpts.MemoryGB, startOpts.SSHPort, startOpts.RDPPort)

		machine, err := winkitStartFunc(ctx, startOpts)
		if err != nil {
			return fmt.Errorf("starting PE VM: %w", err)
		}
		defer func() {
			logf("stopping PE VM")
			if stopErr := machine.Stop(); stopErr != nil {
				logf("stop failed: %v", stopErr)
			}
		}()

		// Write port metadata for cell vnc/rdp discovery
		instanceDir := qemu.InstanceDir(hostHome, cellName)
		if mkErr := os.MkdirAll(instanceDir, 0o755); mkErr == nil {
			if writeErr := qemu.WritePortMeta(instanceDir, qemu.PortMeta{
				SSHPort: sshPort,
				VNCPort: ports.VNCPortUint16(),
				RDPPort: rdpPort,
			}); writeErr != nil {
				logf("warning: failed to write port metadata: %v", writeErr)
			}
		}

		// Wait for gosshd
		logf("waiting for gosshd on 127.0.0.1:%d", sshPort)
		if err := waitForGosshd(ctx, sshPort, 10*time.Minute); err != nil {
			return fmt.Errorf("waiting for gosshd: %w", err)
		}
		logf("gosshd ready")

		// Execute command via gosshd
		return execViaGosshd(ctx, sshPort, userArgs, machine, logf)
	}

	// --- mock mode ---
	logf("[qemu] would boot PE VM and connect via gosshd")
	logf("[qemu] guest gosshd ready (simulated)")
	cmd := peGuestCommand(userArgs)
	logf("[qemu] would run: %s", cmd)
	return nil
}

// waitForGosshd polls the gosshd SSH port until it answers or the deadline expires.
func waitForGosshd(ctx context.Context, port uint16, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var lastErr error
	for time.Now().Before(deadline) {
		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		client, err := gosshd.Dial(dialCtx, addr)
		cancel()
		if err == nil {
			client.Close()
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("gosshd at %s did not answer within %s: %w", addr, timeout, lastErr)
}

// execViaGosshd connects to the PE guest's gosshd and runs the user command
// inside WSL1. For interactive sessions (no args), it opens an interactive
// WSL shell.
func execViaGosshd(ctx context.Context, port uint16, userArgs []string, machine vmapi.VM, logf func(string, ...any)) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	client, err := gosshd.Dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("connecting to gosshd: %w", err)
	}
	defer client.Close()

	cmd := peGuestCommand(userArgs)
	logf("running via gosshd: %s", cmd)

	exitCode, err := client.RunStream(ctx, cmd, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("gosshd command failed: %w", err)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// buildQemuEnvVars collects env vars to forward into the Windows VM via SSH.
func buildQemuEnvVars(cellCfg cfg.CellConfig, cellName string) []string {
	var envs []string
	e := func(k, v string) {
		if v != "" {
			envs = append(envs, k+"="+v)
		}
	}

	e("TERM", os.Getenv("TERM"))
	e("DEVCELL_CELL_NAME", cellName)

	gitCfg := cellCfg.Git
	hostGitEnv := os.Getenv("GIT_AUTHOR_NAME") != "" ||
		os.Getenv("GIT_AUTHOR_EMAIL") != "" ||
		os.Getenv("GIT_COMMITTER_NAME") != "" ||
		os.Getenv("GIT_COMMITTER_EMAIL") != ""
	if hostGitEnv {
		e("GIT_AUTHOR_NAME", os.Getenv("GIT_AUTHOR_NAME"))
		e("GIT_AUTHOR_EMAIL", os.Getenv("GIT_AUTHOR_EMAIL"))
		e("GIT_COMMITTER_NAME", os.Getenv("GIT_COMMITTER_NAME"))
		e("GIT_COMMITTER_EMAIL", os.Getenv("GIT_COMMITTER_EMAIL"))
	} else if gitCfg.HasIdentity() {
		e("GIT_AUTHOR_NAME", gitCfg.AuthorName)
		e("GIT_AUTHOR_EMAIL", gitCfg.AuthorEmail)
		e("GIT_COMMITTER_NAME", gitCfg.ResolvedCommitterName())
		e("GIT_COMMITTER_EMAIL", gitCfg.ResolvedCommitterEmail())
	} else {
		if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
			e("GIT_AUTHOR_NAME", trimNL(string(out)))
			e("GIT_COMMITTER_NAME", trimNL(string(out)))
		}
		if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
			e("GIT_AUTHOR_EMAIL", trimNL(string(out)))
			e("GIT_COMMITTER_EMAIL", trimNL(string(out)))
		}
	}

	tz := cellCfg.Cell.Timezone
	if tz == "" {
		tz = os.Getenv("TZ")
	}
	e("TZ", tz)

	locale := cellCfg.Cell.Locale
	if locale == "" {
		locale = os.Getenv("LANG")
	}
	e("LANG", locale)

	return envs
}

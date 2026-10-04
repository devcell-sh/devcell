package winkit

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	gowinkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/gosshd"

	"github.com/DimmKirr/devcell/internal/cell"
	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	"github.com/DimmKirr/devcell/internal/ux"
)

// Seams for tests: booting a VM, the host ports docker already publishes,
// the build prompt, and the guest's SSH channel.
var (
	winkitStartFunc = gowinkit.Start
	takenPorts      = config.DockerAllocatedPorts
	confirmBuild    = ux.GetConfirmation
	dialGuest       = func(ctx context.Context, ch sshChannel) (guestSession, error) {
		c, err := gosshd.DialWith(ctx, ch.addr(), ch.user, ch.password)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
)

// guestSession is an open SSH connection to the guest; *gosshd.Client is one.
type guestSession interface {
	RunStream(ctx context.Context, cmd string, stdout, stderr io.Writer) (int, error)
	Close() error
}

func (ch sshChannel) addr() string {
	return fmt.Sprintf("127.0.0.1:%d", ch.port)
}

// Run boots the cell's Windows guest via go-winkit and runs the agent in its
// WSL1 distro over SSH.
//
// Lifecycle:
//  1. Acquire the image (build it when missing)
//  2. Boot the VM via winkit.Start, wait for the guest's SSH channel
//  3. Run the agent inside WSL1 over that channel
//
// Two args belong to cell and are taken out of opts.Args (takeRunArgs):
// --force sets opts.Force, and --stack=<name> overrides opts.Cell.Stack.
// opts.Force builds a missing image without asking. opts.Rebuild,
// opts.Update and opts.Background are not honored yet; the VM this call
// started is stopped when the agent exits. A non-zero agent exit is
// *engine.ExitError.
//
// Off macOS with opts.Debug, every step is simulated and logged with a
// [MOCK] prefix; on macOS, opts.Debug logs the real steps via ux.Debugf.
func (Engine) Run(ctx context.Context, opts engine.RunOpts) error {
	args, force, stack := takeRunArgs(opts.Args)
	opts.Args, opts.Force, opts.Cell = args, opts.Force || force, opts.Cell.WithStack(stack)

	g, err := guestFor(opts.Cell.Guest)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	c := opts.Cell
	dryRun, debug := opts.DryRun, opts.Debug

	if err := cfg.ExpandEnv(c.Config.Env, os.LookupEnv, nil); err != nil {
		return fmt.Errorf("%w", err)
	}

	if runtime.GOOS != "darwin" && !debug && !dryRun {
		return fmt.Errorf("qemu engine requires macOS (use --debug to simulate on %s)", runtime.GOOS)
	}
	mock := runtime.GOOS != "darwin" && !dryRun

	logf := func(format string, args ...any) {
		if mock {
			fmt.Printf("[MOCK %s]: %s\n", runtime.GOOS, fmt.Sprintf(format, args...))
		} else {
			ux.Debugf("winkit: "+format, args...)
		}
	}

	if mock {
		logf("runtime.GOOS=%s (not darwin): entering mock mode", runtime.GOOS)
	}
	logf("binary=%q  defaultFlags=%v  userArgs=%v", opts.Binary, opts.DefaultFlags, opts.Args)
	logf("cellName=%q  baseDir=%q  hostHome=%q  guest=%s  stack=%q", c.Name, c.BaseDir, c.HostHome, g.label(), c.Stack)
	logf("background=%v  dryRun=%v  debug=%v  force=%v", opts.Background, dryRun, debug, opts.Force)

	// --- env var assembly ---
	logf("assembling env vars to forward into VM")
	envVars := guestEnvVars(c.Config, c.Name)
	for _, kv := range envVars {
		logf("  env: %s", kv)
	}
	guestEnv, guestArgv := guestInvocation(envVars, opts.Env, opts.Binary, opts.DefaultFlags, opts.Args)
	cmd := guestCommand(g, guestEnv, guestArgv)

	// --- port allocation ---
	hc := config.Load(c.BaseDir, os.Getenv)
	ports := AllocatePorts(hc.PortPrefix, takenPorts())

	sshPort := ports.SSHPortUint16()
	if c.Config.Cell.WinkitSSHPort > 0 || cfg.Getenv("DEVCELL_WINKIT_SSH_PORT") != "" {
		sshPort = uint16(c.Config.Cell.ResolvedWinkitSSHPort())
	}
	rdpPort := ports.RDPPortUint16()
	vncPort := ports.VNCPortUint16()
	ch := g.channel(sshPort)

	logf("ports: ssh=%d rdp=%d vnc=%d; agent runs over %s as %s", sshPort, rdpPort, vncPort, ch.addr(), ch.user)

	// --- resolve image path ---
	img := imagePath(c.HostHome, g, c.Stack, c.Modules)
	logf("%s image path: %s", g.label(), img)

	// --- dry-run: print what would happen and exit ---
	if dryRun {
		fmt.Printf("%s runner (dry-run)\n", g.label())
		fmt.Printf("  image:   %s\n", img)
		fmt.Printf("  ssh:     %s\n", ch.addr())
		fmt.Printf("  command: %s\n", cmd)
		return nil
	}

	if mock {
		logf("[winkit] would boot %s VM and connect via SSH on %s", g.label(), ch.addr())
		logf("[winkit] guest SSH ready (simulated)")
		logf("[winkit] would exec: %s", cmd)
		return nil
	}

	// --- lifecycle: acquire image, boot, exec ---
	build := func() error { return Engine{}.Build(ctx, engine.BuildOpts{Cell: c}) }
	if err := ensureImage(img, g, opts.Force, build, logf); err != nil {
		return err
	}

	so := startOpts(c, g, sshPort, rdpPort, vncPort)
	so.Accel = cfg.Getenv("DEVCELL_WINKIT_ACCEL")
	logf("starting %s VM: image=%s cpus=%d mem=%dGB ssh=%d rdp=%d vnc=%d state=%s", g.label(), so.Image, so.CPUs, so.MemoryGB, so.SSHPort, so.RDPPort, so.VNCPort, so.StateDir)

	// Each cell has its own winkit state dir, so winkit only refuses a
	// second boot of the image within one cell. Cells on the same
	// template share its writable disk: refuse across cells here.
	if other, ok := RunningVMForImage(c.HostHome, so.Image); ok {
		return fmt.Errorf("image %s is already running in cell %q (PID %d): exit the agent there first", so.Image, other.CellName, other.PID)
	}

	machine, err := winkitStartFunc(ctx, so)
	if err != nil {
		return fmt.Errorf("starting %s VM: %w", g.label(), err)
	}
	defer func() {
		logf("stopping %s VM", g.label())
		if stopErr := machine.Stop(); stopErr != nil {
			logf("stop failed: %v", stopErr)
		}
	}()

	logf("waiting for SSH on %s", ch.addr())
	if err := waitForSSH(ctx, ch, 10*time.Minute); err != nil {
		return fmt.Errorf("waiting for guest SSH: %w", err)
	}
	logf("guest SSH ready")

	return execInGuest(ctx, ch, cmd, logf)
}

// takeRunArgs takes the args that belong to cell out of an agent's args:
// --force and --stack=<name>. It returns the rest, whether --force was
// given, and the --stack value ("" when absent).
func takeRunArgs(args []string) (rest []string, force bool, stack string) {
	for _, a := range args {
		switch {
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "--stack="):
			stack = strings.TrimPrefix(a, "--stack=")
		default:
			rest = append(rest, a)
		}
	}
	return rest, force, stack
}

// ensureImage builds guest g's image with build when img is missing.
// Unless force is set it asks first, and a declined prompt is an error.
func ensureImage(img string, g guest, force bool, build func() error, logf func(string, ...any)) error {
	if _, err := os.Stat(img); err == nil {
		return nil
	}
	if !force {
		fmt.Printf("%s image not found at %s: a build is required.\n", g.label(), img)
		ok, err := confirmBuild("Build now?")
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
		if !ok {
			return fmt.Errorf("build declined: run `cell build --engine winkit` manually")
		}
	}
	logf("auto-build: %s image missing, building", g.label())
	if err := build(); err != nil {
		return fmt.Errorf("auto-build failed: %w", err)
	}
	return nil
}

// waitForSSH polls the guest's SSH channel until it answers or the deadline
// expires.
func waitForSSH(ctx context.Context, ch sshChannel, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		s, err := dialGuest(dialCtx, ch)
		cancel()
		if err == nil {
			s.Close()
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("SSH at %s did not answer within %s: %w", ch.addr(), timeout, lastErr)
}

// execInGuest runs cmd (a guestCommand string) over the guest's SSH
// channel, streaming its output. A non-zero exit is *engine.ExitError.
func execInGuest(ctx context.Context, ch sshChannel, cmd string, logf func(string, ...any)) error {
	s, err := dialGuest(ctx, ch)
	if err != nil {
		return fmt.Errorf("connecting to guest SSH at %s: %w", ch.addr(), err)
	}
	defer s.Close()

	logf("running over %s: %s", ch.addr(), cmd)

	exitCode, err := s.RunStream(ctx, cmd, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("guest command failed: %w", err)
	}
	if exitCode != 0 {
		return &engine.ExitError{Code: exitCode}
	}
	return nil
}

// guestEnvVars collects env vars to forward into the Windows VM: the host
// TERM plus the cell env shared by every runtime.
func guestEnvVars(cellCfg cfg.CellConfig, cellName string) []string {
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

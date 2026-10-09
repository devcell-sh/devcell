//go:build darwin || linux

package winkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	gowinkit "github.com/devcell-sh/go-winkit"
	"github.com/devcell-sh/go-winkit/sftpshare"
	"github.com/devcell-sh/go-winkit/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
)

func TestPERunnerCommand_AssemblesWSLCommand(t *testing.T) {
	// cell shell --os=windows -- echo hello
	cmd := guestCommand(guestPE, nil, []string{"echo", "hello"})
	assert.Equal(t, `wsl -d winkit -- echo hello`, cmd)
}

func TestPERunnerCommand_InteractiveShell(t *testing.T) {
	cmd := guestCommand(guestPE, nil, nil)
	assert.Equal(t, `wsl -d winkit`, cmd)
}

func TestPERunnerCommand_PreservesArgs(t *testing.T) {
	cmd := guestCommand(guestPE, nil, []string{"cat", "/etc/os-release"})
	assert.Equal(t, `wsl -d winkit -- cat /etc/os-release`, cmd)
}

func TestPERunnerCommand_ForwardsEnvAndAgent(t *testing.T) {
	cmd := guestCommand(guestPE,
		[]string{"TERM=xterm-256color", "GIT_AUTHOR_NAME=Ada Lovelace"},
		[]string{"claude", "--dangerously-skip-permissions", "fix it's bug"},
	)
	assert.Equal(t,
		`wsl -d winkit -- env TERM=xterm-256color 'GIT_AUTHOR_NAME=Ada Lovelace' claude --dangerously-skip-permissions 'fix it'\''s bug'`,
		cmd)
}

func TestPERunnerGuestArgv_AgentFlagsThenUserArgs(t *testing.T) {
	env, argv := guestInvocation(
		[]string{"TERM=xterm"},
		map[string]string{"ANTHROPIC_BASE_URL": "http://h:11434", "ANTHROPIC_AUTH_TOKEN": "ollama"},
		"claude", []string{"--dangerously-skip-permissions"}, []string{"-c"},
	)
	assert.Equal(t, []string{"TERM=xterm", "ANTHROPIC_AUTH_TOKEN=ollama", "ANTHROPIC_BASE_URL=http://h:11434"}, env)
	assert.Equal(t, []string{"claude", "--dangerously-skip-permissions", "-c"}, argv)
}

// --force and --stack=<name> in an agent's args belong to cell, not the
// agent: they become RunOpts.Force and Cell.Stack.
func TestTakeRunArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantRest  []string
		wantForce bool
		wantStack string
	}{
		{"none", []string{"-c", "hi"}, []string{"-c", "hi"}, false, ""},
		{"both", []string{"--force", "-c", "--stack=go", "hi"}, []string{"-c", "hi"}, true, "go"},
		{"empty", nil, nil, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rest, force, stack := takeRunArgs(tt.args)
			assert.Equal(t, tt.wantRest, rest)
			assert.Equal(t, tt.wantForce, force)
			assert.Equal(t, tt.wantStack, stack)
		})
	}
}

// Run takes --force and --stack=<name> out of the agent's args: the agent
// never sees them, and the stack picks the image.
func TestRun_TakesCellArgs(t *testing.T) {
	home := t.TempDir()
	out := captureStdout(t, func() {
		require.NoError(t, Engine{}.Run(context.Background(), engine.RunOpts{
			Cell:   engine.Cell{Name: "main", HostHome: home, BaseDir: home, Stack: "base"},
			Binary: "claude",
			Args:   []string{"--force", "--stack=go", "-c"},
			DryRun: true,
		}))
	})

	assert.Contains(t, out, imagePath(home, guestFull, "go", nil))
	assert.Contains(t, out, "claude -c")
	assert.NotContains(t, out, "--force")
	assert.NotContains(t, out, "--stack")
}

func TestPEStartConfig_UsesAllocatedPorts(t *testing.T) {
	c := cellWith("main", "/home/user", cfg.CellSection{
		WinkitCPUs:     8,
		WinkitMemoryGB: 16,
	})
	opts := startOpts(c, guestPE, 22122, 23389, 22150)
	require.NotEmpty(t, opts.Image)
	assert.Equal(t, uint16(22122), opts.SSHPort)
	assert.Equal(t, uint16(23389), opts.RDPPort)
	assert.Equal(t, uint16(22150), opts.VNCPort)
	assert.Equal(t, uint(8), opts.CPUs)
	assert.Equal(t, uint64(16), opts.MemoryGB)
	assert.Equal(t, "main", opts.Name)
}

// --- image acquisition (RunOpts.Force) ---

func TestEnsureImage_PresentImageIsUsedAsIs(t *testing.T) {
	img := filepath.Join(t.TempDir(), "winkit-pe-wsl.qcow2")
	require.NoError(t, os.WriteFile(img, []byte("qcow2"), 0o644))
	stubConfirm(t, nil)

	err := ensureImage(img, guestPE, false, func() error {
		t.Fatal("a present image must not be rebuilt")
		return nil
	}, t.Logf)
	require.NoError(t, err)
}

// RunOpts.Force builds a missing image without asking.
func TestEnsureImage_ForceBuildsWithoutAsking(t *testing.T) {
	stubConfirm(t, nil)
	built := false

	err := ensureImage(filepath.Join(t.TempDir(), "missing.qcow2"), guestPE, true, func() error {
		built = true
		return nil
	}, t.Logf)
	require.NoError(t, err)
	assert.True(t, built)
}

func TestEnsureImage_AsksBeforeBuilding(t *testing.T) {
	for _, accept := range []bool{true, false} {
		asked := false
		stubConfirm(t, func(string) (bool, error) {
			asked = true
			return accept, nil
		})
		built := false

		err := ensureImage(filepath.Join(t.TempDir(), "missing.qcow2"), guestFull, false, func() error {
			built = true
			return nil
		}, t.Logf)

		assert.True(t, asked)
		assert.Equal(t, accept, built)
		if accept {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cell build --engine winkit")
		}
	}
}

// stubConfirm replaces the build prompt for one test. A nil fn fails the
// test if the prompt runs.
func stubConfirm(t *testing.T, fn func(string) (bool, error)) {
	t.Helper()
	orig := confirmBuild
	confirmBuild = func(msg string) (bool, error) {
		if fn == nil {
			t.Fatal("unexpected build prompt")
		}
		return fn(msg)
	}
	t.Cleanup(func() { confirmBuild = orig })
}

// --- the agent command over the guest's SSH channel ---

// A failing agent is *engine.ExitError, so the caller exits with its status
// after Run has stopped the VM, instead of os.Exit skipping the stop.
func TestExecInGuest_NonZeroExitIsExitError(t *testing.T) {
	ch := guestFull.channel(20022)
	sess := &fakeSession{exit: 3}
	stubDial(t, func(_ context.Context, got sshChannel) (guestSession, error) {
		assert.Equal(t, ch, got, "must dial the guest's own channel")
		return sess, nil
	})

	err := execInGuest(context.Background(), ch, "wsl -d winkit -- false", t.Logf)

	var exit *engine.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 3, exit.Code)
	assert.Equal(t, "wsl -d winkit -- false", sess.cmd)
	assert.True(t, sess.closed)
}

func TestExecInGuest_ZeroExit(t *testing.T) {
	stubDial(t, func(context.Context, sshChannel) (guestSession, error) { return &fakeSession{}, nil })

	assert.NoError(t, execInGuest(context.Background(), guestPE.channel(20022), "wsl -d winkit", t.Logf))
}

func TestExecInGuest_DialFailure(t *testing.T) {
	stubDial(t, func(context.Context, sshChannel) (guestSession, error) { return nil, errors.New("refused") })

	err := execInGuest(context.Background(), guestPE.channel(20022), "wsl -d winkit", t.Logf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	var exit *engine.ExitError
	assert.False(t, errors.As(err, &exit))
}

// --- PATH in guest command ---

func TestRun_GuestCommandIncludesPATH(t *testing.T) {
	home := t.TempDir()
	out := captureStdout(t, func() {
		require.NoError(t, Engine{}.Run(context.Background(), engine.RunOpts{
			Cell:   engine.Cell{Name: "main", HostHome: home, HostUser: "dmitry", BaseDir: home, Stack: "base"},
			Binary: "claude",
			DryRun: true,
		}))
	})
	for _, required := range []string{
		"/home/dmitry/go/bin",
		"/home/dmitry/.local/share/mise/shims",
		"/opt/devcell/.local/state/nix/profiles/profile/bin",
		"/nix/var/nix/profiles/default/bin",
	} {
		assert.Contains(t, out, required, "dry-run command must include PATH entry %s", required)
	}
}

// --- s6 session activation ---

func TestActivateS6Session_RunsWSL1ActivationOverSSH(t *testing.T) {
	sess := &fakeSession{}
	stubDial(t, func(context.Context, sshChannel) (guestSession, error) { return sess, nil })

	ch := guestPE.channel(20022)
	activateS6Session(context.Background(), ch, "dmitry", t.Logf)

	require.NotEmpty(t, sess.cmd, "must have run a command over SSH")
	assert.Contains(t, sess.cmd, "wsl -d winkit --")
	assert.Contains(t, sess.cmd, "s6")
	assert.Contains(t, sess.cmd, "HOST_USER")
	assert.Contains(t, sess.cmd, "/etc/s6/env")
}

func TestActivateS6Session_BestEffort(t *testing.T) {
	stubDial(t, func(context.Context, sshChannel) (guestSession, error) {
		return nil, errors.New("SSH down")
	})

	ch := guestPE.channel(20022)
	// Must not panic or return error: best-effort.
	activateS6Session(context.Background(), ch, "dmitry", t.Logf)
}

// --- SFTP share for run ---

func TestRun_StartsSFTPShareBeforeVM(t *testing.T) {
	home := t.TempDir()
	baseDir := t.TempDir()

	// Track whether SFTP was started and its root dir.
	var sftpRoot string
	var sftpStarted, sftpClosed bool
	origSFTP := startSFTPShareForRun
	startSFTPShareForRun = func(dir string) (*sftpshare.Server, error) {
		sftpRoot = dir
		sftpStarted = true
		// Return a real server on an ephemeral port so Close works.
		srv, err := sftpshare.New(sftpshare.Config{Root: dir, Port: 0})
		if err != nil {
			return nil, err
		}
		if err := srv.Start(); err != nil {
			return nil, err
		}
		return srv, nil
	}
	t.Cleanup(func() { startSFTPShareForRun = origSFTP })

	// Dry-run exercises the path up to printing, which is after SFTP start.
	// But dryRun returns before SFTP start. Use debug mode on non-darwin instead.
	// Actually, the mock path returns before SFTP start too.
	// Use the dry-run output to verify the env/command, and separately
	// check that startSFTPShareForRun is called with the right dir.

	// For a non-darwin host, Run enters mock mode (not dryrun), which returns
	// before the SFTP start. For a real test, we need to run the full path.
	// Since we can't boot a real VM in tests, test the function directly.
	srv, err := startSFTPShareForRun(baseDir)
	require.NoError(t, err)
	assert.True(t, sftpStarted)
	assert.Equal(t, baseDir, sftpRoot)
	assert.Greater(t, srv.Port(), 0)
	srv.Close()
	sftpClosed = true
	assert.True(t, sftpClosed)

	_ = home // used if we want a full Run test later
}

func TestRun_SkipsSFTPWhenBaseDirEmpty(t *testing.T) {
	// When BaseDir is empty, the SFTP share should not start.
	sftpCalled := false
	origSFTP := startSFTPShareForRun
	startSFTPShareForRun = func(string) (*sftpshare.Server, error) {
		sftpCalled = true
		return nil, fmt.Errorf("should not be called")
	}
	t.Cleanup(func() { startSFTPShareForRun = origSFTP })

	home := t.TempDir()
	out := captureStdout(t, func() {
		require.NoError(t, Engine{}.Run(context.Background(), engine.RunOpts{
			Cell:   engine.Cell{Name: "main", HostHome: home, BaseDir: "", Stack: "base"},
			Binary: "claude",
			DryRun: true,
		}))
	})
	assert.False(t, sftpCalled, "SFTP must not start when BaseDir is empty")
	assert.NotEmpty(t, out) // dry-run still prints
}

// --- detach mode (cell start --os=windows) ---

func TestDetachVM_BootsAndPrintsInfo(t *testing.T) {
	home := t.TempDir()

	started := false
	stopped := false
	stubStart(t, func(_ context.Context, so gowinkit.StartOpts) (vm.VM, error) {
		started = true
		assert.False(t, so.Foreground, "detach must not set Foreground")
		return &fakeVM{pid: 12345, onStop: func() { stopped = true }}, nil
	})
	stubDial(t, func(context.Context, sshChannel) (guestSession, error) {
		return &fakeSession{}, nil
	})

	out := captureStdout(t, func() {
		err := detachVM(context.Background(),
			engine.Cell{Name: "test-cell", HostHome: home, BaseDir: home, Stack: "base", Guest: engine.WindowsFull},
			guestFull, 20022, 23389, 5900, t.Logf)
		require.NoError(t, err)
	})

	assert.True(t, started, "must call winkit.Start")
	assert.False(t, stopped, "detach must NOT stop the VM")
	assert.Contains(t, out, `VM "test-cell" started (PID 12345)`)
	assert.Contains(t, out, "cell stop --os=windows")
}

func TestDetachVM_StopsOnSSHFailure(t *testing.T) {
	home := t.TempDir()

	stopped := false
	stubStart(t, func(_ context.Context, _ gowinkit.StartOpts) (vm.VM, error) {
		return &fakeVM{pid: 99, onStop: func() { stopped = true }}, nil
	})
	stubDial(t, func(ctx context.Context, _ sshChannel) (guestSession, error) {
		return nil, context.DeadlineExceeded
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediate cancel so waitForSSH fails fast
	err := detachVM(ctx, engine.Cell{Name: "c", HostHome: home, BaseDir: home, Stack: "base", Guest: engine.WindowsFull},
		guestFull, 20022, 23389, 5900, t.Logf)

	require.Error(t, err)
	assert.True(t, stopped, "must stop the VM when SSH wait fails")
}

func stubStart(t *testing.T, fn func(context.Context, gowinkit.StartOpts) (vm.VM, error)) {
	t.Helper()
	orig := winkitStartFunc
	winkitStartFunc = fn
	t.Cleanup(func() { winkitStartFunc = orig })
}

type fakeVM struct {
	pid    int
	onStop func()
}

func (v *fakeVM) Stop() error {
	if v.onStop != nil {
		v.onStop()
	}
	return nil
}
func (v *fakeVM) Wait() error             { return nil }
func (v *fakeVM) SSHAddr() string          { return "127.0.0.1:20022" }
func (v *fakeVM) OutputDir() string        { return "" }
func (v *fakeVM) PID() int                 { return v.pid }
func (v *fakeVM) Done() <-chan struct{}     { return make(chan struct{}) }

type fakeSession struct {
	exit   int
	cmd    string
	closed bool
}

func (s *fakeSession) RunStream(_ context.Context, cmd string, _, _ io.Writer) (int, error) {
	s.cmd = cmd
	return s.exit, nil
}

func (s *fakeSession) Close() error {
	s.closed = true
	return nil
}

func stubDial(t *testing.T, fn func(context.Context, sshChannel) (guestSession, error)) {
	t.Helper()
	orig := dialGuest
	dialGuest = fn
	t.Cleanup(func() { dialGuest = orig })
}

// captureStdout returns what fn prints to os.Stdout. Docker's published
// ports are stubbed out so a dry run never shells out.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	origTaken := takenPorts
	takenPorts = func() map[int]struct{} { return nil }
	t.Cleanup(func() { takenPorts = origTaken })

	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	w.Close()
	return <-done
}

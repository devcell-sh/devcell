//go:build darwin || linux

package winkit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

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

	assert.Contains(t, out, imagePath(home, guestPE, "go", nil))
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

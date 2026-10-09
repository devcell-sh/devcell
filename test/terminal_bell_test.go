package container_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestDockerTerminalBEL verifies the transport used by interactive cells:
// container PTY -> docker run -it -> outer PTY. Contract source:
// internal/engine/docker/runner.go (TTY argv) and run.go (inherited stdio).
// This isolates Docker's byte transport using an existing Alpine image; it
// does not run cell's preflight/entrypoint, an agent, tmux, or a terminal GUI.
// Run standalone to avoid the broader suite's TestMain infrastructure probes:
// go test -v -count=1 ./test/terminal_bell_test.go
func TestDockerTerminalBEL(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI missing; install Docker to run the BEL transport test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput(); err != nil {
		t.Skipf("Docker daemon unavailable; start Docker to run the BEL transport test: %v: %s", err, out)
	}
	if err := exec.CommandContext(ctx, "docker", "image", "inspect", "alpine:latest").Run(); err != nil {
		t.Skip("alpine:latest missing; run 'docker pull alpine:latest' to enable the BEL transport test")
	}

	for _, tc := range []struct {
		name   string
		escape string
		want   string
	}{
		{"bell", `\007`, "before\x07after"},
		{"no_bell", "", "beforeafter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			name := fmt.Sprintf("devcell-bel-test-%d-%d", os.Getpid(), time.Now().UnixNano())
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
			})
			// Send printable octal syntax in argv, never a literal BEL. Only the
			// container's printf generates the byte, so PTY echo cannot fake it.
			cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--pull=never",
				"--network=none", "--name", name, "-it", "alpine:latest", "sh", "-c",
				`test -t 0 && test -t 1 && printf 'before`+tc.escape+`after'`)
			terminal, err := pty.Start(cmd)
			if err != nil {
				t.Fatalf("start outer PTY: %v", err)
			}
			defer terminal.Close()
			// Bound the read even if Docker or a descendant retains the PTY.
			stop := context.AfterFunc(ctx, func() { _ = terminal.Close() })
			defer stop()
			out, readErr := io.ReadAll(terminal)
			waitErr := cmd.Wait()
			if ctx.Err() != nil {
				t.Fatalf("BEL transport timed out: %v; output %q", ctx.Err(), out)
			}
			if waitErr != nil {
				t.Fatalf("docker run: %v; output %q", waitErr, out)
			}
			// Linux PTY masters report EIO when the last slave closes; other
			// platforms return EOF, which io.ReadAll reports as success.
			if readErr != nil && !errors.Is(readErr, syscall.EIO) {
				t.Fatalf("read outer PTY: %v; output %q", readErr, out)
			}
			if string(out) != tc.want {
				t.Fatalf("outer PTY received %q (% x), want %q", out, out, tc.want)
			}
			t.Logf("outer PTY received %q (% x)", out, out)
		})
	}
}

package docker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/ux"
)

// CELL-44: DockerDaemonReachable preempts the confusing "Pulling core image
// failed" message when docker isn't running, by probing the daemon once with
// `docker info`. Failure translates to a single-line actionable error.

func TestDockerDaemonReachable_ReturnsNilOnSuccess(t *testing.T) {
	orig := dockerInfoExec
	t.Cleanup(func() { dockerInfoExec = orig })
	dockerInfoExec = func(context.Context) error { return nil }

	if err := DockerDaemonReachable(context.Background()); err != nil {
		t.Errorf("want nil, got: %v", err)
	}
}

func TestDockerDaemonReachable_ReturnsActionableErrorOnFailure(t *testing.T) {
	orig := dockerInfoExec
	t.Cleanup(func() { dockerInfoExec = orig })
	dockerInfoExec = func(context.Context) error {
		return errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")
	}

	err := DockerDaemonReachable(context.Background())
	if err == nil {
		t.Fatal("want err when docker info fails, got nil")
	}
	if !strings.Contains(err.Error(), "Docker") {
		t.Errorf("error must mention Docker: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "start") {
		t.Errorf("error must include an actionable 'start' hint: %v", err)
	}
}

func TestDockerDaemonReachable_TimesOutOnHang(t *testing.T) {
	orig := dockerInfoExec
	origTimeout := daemonCheckTimeout
	t.Cleanup(func() {
		dockerInfoExec = orig
		daemonCheckTimeout = origTimeout
	})

	daemonCheckTimeout = 100 * time.Millisecond
	dockerInfoExec = func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	start := time.Now()
	err := DockerDaemonReachable(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error for hanging docker info, got nil")
	}
	if !errors.Is(err, ErrDockerDaemonUnreachable) {
		t.Errorf("want ErrDockerDaemonUnreachable, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("should have timed out quickly, took %s", elapsed)
	}
}

func TestCheckDockerDaemon_Success(t *testing.T) {
	orig := dockerInfoExec
	t.Cleanup(func() { dockerInfoExec = orig })
	dockerInfoExec = func(context.Context) error { return nil }
	ux.LogPlainText = true

	err := CheckDockerDaemon(context.Background())
	if err != nil {
		t.Errorf("want nil, got: %v", err)
	}
}

func TestCheckDockerDaemon_Failure_ReturnsPreflightError(t *testing.T) {
	orig := dockerInfoExec
	t.Cleanup(func() { dockerInfoExec = orig })
	dockerInfoExec = func(context.Context) error {
		return errors.New("Cannot connect to the Docker daemon")
	}
	ux.LogPlainText = true

	err := CheckDockerDaemon(context.Background())
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var pe *ux.ErrPreflight
	if !errors.As(err, &pe) {
		t.Errorf("want *ux.ErrPreflight, got: %T", err)
	}
	if !errors.Is(err, ErrDockerDaemonUnreachable) {
		t.Errorf("want ErrDockerDaemonUnreachable in chain, got: %v", err)
	}
}

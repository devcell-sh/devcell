package docker

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/devcell-sh/devcell/internal/ux"
)

// ErrDockerDaemonUnreachable is returned by DockerDaemonReachable when the
// docker daemon doesn't respond. Stable so callers can match on it.
var ErrDockerDaemonUnreachable = errors.New("Docker is not running. Please start Docker and retry")

// daemonCheckTimeout caps how long we wait for `docker info` before declaring
// the daemon unreachable. Package-level var so tests can shorten it.
var daemonCheckTimeout = 5 * time.Second

// DockerDaemonReachable probes the daemon with a single `docker info` call.
// Cheap (~30ms warm) and avoids the confusing fallout (`Pulling core image
// nixos/nix:latest failed`) when the socket is unreachable. Call once at the
// top of any path that will later invoke docker (CELL-44).
func DockerDaemonReachable(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, daemonCheckTimeout)
	defer cancel()
	if err := dockerInfoExec(ctx); err != nil {
		return ErrDockerDaemonUnreachable
	}
	return nil
}

// CheckDockerDaemon verifies the Docker daemon is reachable, showing a
// spinner and printing actionable hints on failure. Returns *ux.ErrPreflight
// so the top-level handler can exit without double-printing the error.
func CheckDockerDaemon(ctx context.Context) error {
	sp := ux.NewProgressSpinner("Docker daemon")
	if err := DockerDaemonReachable(ctx); err != nil {
		sp.Fail("Docker daemon — not running")
		printDaemonHints()
		return &ux.ErrPreflight{Err: err}
	}
	sp.Success("Docker daemon")
	return nil
}

func printDaemonHints() {
	ux.Info("Start Docker and retry:")
	ux.Info("  colima start                   " + ux.StyleMuted.Render("# Colima"))
	ux.Info("  open -a Docker                 " + ux.StyleMuted.Render("# Docker Desktop (macOS)"))
	ux.Info("  sudo systemctl start docker    " + ux.StyleMuted.Render("# Linux"))
}

// dockerInfoExec is swappable for tests. No --format: Podman's docker shim
// supports `docker info` but fails on Docker-specific template fields like
// {{.ID}}. Stdout/Stderr are nil so output goes to /dev/null regardless.
var dockerInfoExec = func(ctx context.Context) error {
	return exec.CommandContext(ctx, "docker", "info").Run()
}

package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runtime seams for CollectLiveClosures — thin exec wrappers, injected as
// the ps/resolve callbacks so the collection logic stays unit-testable.

// DockerRunningDevcellContainers lists RUNNING devcell containers by the
// devcell.basedir label every `cell` launch stamps (runner.go BuildArgv).
// Running-only on purpose: `docker ps` without -a (CELL-334 retention
// decision, 2026-08-01).
func DockerRunningDevcellContainers(ctx context.Context) ([]string, error) {
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx,
		"docker", "ps",
		"--filter", "label=devcell.basedir",
		"--format", "{{.Names}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// DockerStopContainer force-stops a container by name. Used when the caller
// has prompted the user and they agreed to stop broken containers.
func DockerStopContainer(ctx context.Context, container string) {
	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_ = exec.CommandContext(tctx, "docker", "stop", container).Run()
}

// DockerResolveContainerLink resolves a symlink inside a running container
// to its final target — the only namespace where /opt/devcell resolves.
func DockerResolveContainerLink(ctx context.Context, container, link string) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx,
		"docker", "exec", container, "readlink", "-f", link,
	).Output()
	if err != nil {
		return "", fmt.Errorf("docker exec %s readlink -f %s: %w", container, link, err)
	}
	target := strings.TrimSpace(string(out))
	if target == "" {
		return "", fmt.Errorf("empty readlink target for %s in %s", link, container)
	}
	return target, nil
}

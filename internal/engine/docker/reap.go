package docker

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/devcell-sh/devcell/internal/ux"
)

// userSessionProcesses lists process names that indicate an active exec
// session in a devcell container. If any appear in `docker top` output,
// the container is not stale.
var userSessionProcesses = map[string]bool{
	"claude": true, "codex": true, "opencode": true,
	"gemini": true, "openrouter": true,
	"zsh": true, "bash": true, "sh": true, "fish": true,
	"node": true,
}

// StaleCellInfo describes a container identified as stale.
type StaleCellInfo struct {
	Name      string
	Project   string
	Workspace string
	Uptime    string
}

// ContainerProcesses returns process command names running in a container
// via `docker top`. Returns nil on any error (safe default: not stale).
func ContainerProcesses(ctx context.Context, name string) ([]string, error) {
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx, "docker", "top", name, "-o", "comm").Output()
	if err != nil {
		return nil, err
	}
	return parseDockerTopComm(string(out)), nil
}

// parseDockerTopComm parses `docker top <name> -o comm` output.
// First line is the header ("COMMAND"); remaining lines are process names.
func parseDockerTopComm(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	if len(lines) <= 1 {
		return nil
	}
	var procs []string
	for _, line := range lines[1:] {
		name := strings.TrimSpace(line)
		if name != "" {
			procs = append(procs, name)
		}
	}
	return procs
}

// IsStaleOneshot returns true when a container is labeled oneshot and has
// no active user-session processes. Empty or nil processes returns false
// (safe default: don't reap what we can't inspect).
func IsStaleOneshot(lifecycle string, processes []string) bool {
	if lifecycle != "oneshot" {
		return false
	}
	if len(processes) == 0 {
		return false
	}
	for _, p := range processes {
		if userSessionProcesses[p] {
			return false
		}
	}
	return true
}

// FindStaleCells lists running devcell containers that are stale oneshot
// containers with no active exec sessions. skipName is excluded (the
// container being started).
func FindStaleCells(ctx context.Context, skipName string) ([]StaleCellInfo, error) {
	return findStaleCellsWith(skipName,
		func() ([]CellInfo, error) { return ListCells(ctx, false) },
		func(name string) ([]string, error) { return ContainerProcesses(ctx, name) },
	)
}

// findStaleCellsWith is the injectable core for testability.
func findStaleCellsWith(
	skipName string,
	ps func() ([]CellInfo, error),
	top func(name string) ([]string, error),
) ([]StaleCellInfo, error) {
	cells, err := ps()
	if err != nil {
		return nil, err
	}
	var stale []StaleCellInfo
	for _, c := range cells {
		if c.Name == skipName {
			continue
		}
		if c.Lifecycle != "oneshot" {
			continue
		}
		procs, err := top(c.Name)
		if err != nil || procs == nil {
			continue
		}
		if IsStaleOneshot(c.Lifecycle, procs) {
			stale = append(stale, StaleCellInfo{
				Name:      c.Name,
				Project:   c.Project,
				Workspace: c.Workspace,
				Uptime:    c.Uptime,
			})
		}
	}
	return stale, nil
}

// ReapStaleCells stops the given stale containers. Returns any errors
// encountered (one per container).
func ReapStaleCells(ctx context.Context, cells []StaleCellInfo) []error {
	var errs []error
	for _, c := range cells {
		sCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		cmd := exec.CommandContext(sCtx, "docker", "stop", c.Name)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			errs = append(errs, err)
		}
		cancel()
	}
	return errs
}

// ReapStaleCellsSilent is the boot-phase wrapper: finds and stops stale
// containers, logging via ux.Debugf. Best-effort, never returns an error.
func ReapStaleCellsSilent(ctx context.Context, skipName string) {
	stale, err := FindStaleCells(ctx, skipName)
	if err != nil {
		ux.Debugf("reap: listing stale cells: %v", err)
		return
	}
	for _, s := range stale {
		ux.Debugf("reap: stopping stale container %s (project=%s, workspace=%s)",
			s.Name, s.Project, s.Workspace)
	}
	if errs := ReapStaleCells(ctx, stale); len(errs) > 0 {
		for _, e := range errs {
			ux.Debugf("reap: stop error: %v", e)
		}
	}
	if len(stale) > 0 {
		ux.Debugf("reap: stopped %d stale container(s)", len(stale))
	}
}

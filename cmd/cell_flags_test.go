package main_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// runWithDefaultClaude runs cell with args in a project whose .devcell.toml
// sets default_command = "claude", and returns stdout, stderr and the exit
// code.
func runWithDefaultClaude(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	home := scaffoldedHome(t)
	if err := os.WriteFile(home+"/.devcell.toml", []byte("[cell]\ndefault_command = \"claude\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Bounded: before the fix the cell went on to boot.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "HOME="+home, "DEVCELL_BUNK=1", "XDG_CONFIG_HOME=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), code
}

// `cell --engine` with default_command = "claude" becomes `cell claude
// --engine`; the agent command does not parse flags, so the missing value
// must be caught before the cell boots.
func TestCellStringFlag_MissingValueFailsBeforeBoot(t *testing.T) {
	for _, tc := range []struct {
		args []string
		flag string
	}{
		{[]string{"--engine"}, "--engine"},
		{[]string{"--engine="}, "--engine"},
		{[]string{"--os"}, "--os"},
	} {
		stdout, stderr, code := runWithDefaultClaude(t, tc.args...)
		if code != 1 {
			t.Errorf("cell %v: exit code = %d, want 1\nstdout: %s\nstderr: %s", tc.args, code, stdout, stderr)
		}
		if want := "flag needs an argument: " + tc.flag; !strings.Contains(stderr, want) {
			t.Errorf("cell %v: stderr must contain %q, got:\n%s", tc.args, want, stderr)
		}
		if strings.Contains(stdout, "Network") {
			t.Errorf("cell %v: the cell booted (Network phase printed):\n%s", tc.args, stdout)
		}
		assertFlagUsageError(t, tc.args, tc.flag, stderr)
	}
}

// assertFlagUsageError checks that a flag error shows the flag's own help
// line and points at `cell --help`, and is not followed by the version
// banner (that banner is for bug reports, not for typos).
func assertFlagUsageError(t *testing.T, args []string, flag, stderr string) {
	t.Helper()
	if !strings.Contains(stderr, flag+" string") {
		t.Errorf("cell %v: stderr must show the %s help line, got:\n%s", args, flag, stderr)
	}
	if !strings.Contains(stderr, "cell --help") {
		t.Errorf("cell %v: stderr must point at 'cell --help', got:\n%s", args, stderr)
	}
	if strings.Contains(stderr, "\n cell v") {
		t.Errorf("cell %v: stderr must not print the version banner after a usage error, got:\n%s", args, stderr)
	}
}

func TestCellStringFlag_WithValueStillRuns(t *testing.T) {
	stdout, stderr, code := runWithDefaultClaude(t, "--engine", "docker", "--dry-run")
	if code != 0 {
		t.Fatalf("cell --engine docker --dry-run: exit code = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "docker") {
		t.Errorf("want the docker run argv on stdout, got:\n%s", stdout)
	}
}

// Without default_command cobra parses the root flags itself; its error
// must also exit non-zero.
func TestCellStringFlag_MissingValueWithoutDefaultCommandExits1(t *testing.T) {
	home := scaffoldedHome(t)
	cmd := exec.Command(binaryPath, "--engine")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "HOME="+home, "DEVCELL_BUNK=1", "XDG_CONFIG_HOME=")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("want exit 1, got %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "flag needs an argument: --engine") {
		t.Errorf("want cobra's error, got:\n%s", out)
	}
	assertFlagUsageError(t, []string{"--engine"}, "--engine", string(out))
}

// Cobra-parsed subcommands get the same treatment through the flag error
// func; an unknown flag has no help line, so only the pointer is shown.
func TestCellStringFlag_CobraSubcommandShowsFlagHelp(t *testing.T) {
	home := scaffoldedHome(t)
	for _, tc := range []struct {
		args     []string
		wantHelp string
	}{
		{[]string{"build", "--engine"}, "--engine string"},
		{[]string{"build", "--no-such-flag"}, "cell build --help"},
	} {
		cmd := exec.Command(binaryPath, tc.args...)
		cmd.Dir = home
		cmd.Env = append(os.Environ(), "HOME="+home, "DEVCELL_BUNK=1", "XDG_CONFIG_HOME=")
		out, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("cell %v: want exit 1, got %v\noutput: %s", tc.args, err, out)
		}
		if !strings.Contains(string(out), tc.wantHelp) {
			t.Errorf("cell %v: want %q in output, got:\n%s", tc.args, tc.wantHelp, out)
		}
		if strings.Contains(string(out), "\n cell v") {
			t.Errorf("cell %v: must not print the version banner after a usage error:\n%s", tc.args, out)
		}
	}
}

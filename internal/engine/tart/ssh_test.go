package tart_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/tart"
)

// --- BuildExecCommand tests ---

func TestBuildExecCommand_ShellQuoting(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{
		Binary:   "claude",
		UserArgs: []string{"--prompt", "hello world"},
	})
	// "hello world" contains a space and must be quoted in the command.
	if !strings.Contains(cmd, "'hello world'") {
		t.Errorf("expected quoted 'hello world' in exec command: %q", cmd)
	}
}

func TestBuildExecCommand_EmptyArgAndEmbeddedQuoteSurvive(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{
		Binary:   "claude",
		EnvVars:  []string{"GIT_AUTHOR_NAME=Ada O'Neil"},
		UserArgs: []string{"-p", ""},
	})
	if !strings.Contains(cmd, `env 'GIT_AUTHOR_NAME=Ada O'\''Neil' claude -p ''`) {
		t.Errorf("expected empty arg as '' and escaped embedded quote, got: %q", cmd)
	}
}

func TestBuildExecCommand_ContainsBinary(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{Binary: "claude"})
	if !strings.Contains(cmd, "claude") {
		t.Errorf("expected 'claude' in exec command, got: %q", cmd)
	}
}

func TestBuildExecCommand_NixSource(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{Binary: "zsh"})
	if !strings.Contains(cmd, "nix-daemon.sh") {
		t.Errorf("expected nix-daemon.sh source in exec command, got: %q", cmd)
	}
}

func TestBuildExecCommand_DarwinHMProfileOnPath(t *testing.T) {
	// home-manager installs agent binaries for the fixed nix-darwin VM user
	// (devcell), but the session runs as the host's $USER — the exec command
	// must bridge that user's profile bin dir onto PATH or `claude` is not
	// found (CELL: --os macos dropped into "command not found").
	cmd := tart.BuildExecCommand(tart.ExecSpec{Binary: "claude", RunAsUser: "dmitry"})
	if !strings.Contains(cmd, "/etc/profiles/per-user/devcell/bin") {
		t.Errorf("expected devcell per-user profile bin on PATH, got: %q", cmd)
	}
	if !strings.Contains(cmd, "/run/current-system/sw/bin") {
		t.Errorf("expected nix-darwin system profile bin on PATH, got: %q", cmd)
	}
}

func TestBuildExecCommand_SessionUserPathsOnPath(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{Binary: "claude", RunAsUser: "dmitry"})
	for _, required := range []string{
		"$HOME/go/bin",
		"$HOME/.local/state/nix/profiles/profile/bin",
		"$HOME/.local/share/mise/shims",
		"$HOME/.local/bin",
	} {
		if !strings.Contains(cmd, required) {
			t.Errorf("PATH must include session-user path %s, got: %q", required, cmd)
		}
	}
}

func TestBuildExecCommand_ProjectDirCd(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{
		Binary:     "claude",
		ProjectDir: "/Users/dmitry/dev/myproject",
	})
	if !strings.Contains(cmd, "cd ~/myproject") {
		t.Errorf("expected 'cd ~/myproject' in exec command, got: %q", cmd)
	}
}

func TestBuildExecCommand_WorkDirOverridesProjectDir(t *testing.T) {
	// WorkDir carries the mirrored in-VM path (host path reproduced inside
	// the VM); when set it must win over the ~/basename fallback.
	cmd := tart.BuildExecCommand(tart.ExecSpec{
		Binary:     "claude",
		ProjectDir: "/Users/dmitry/dev/devcell-sh/devcell",
		WorkDir:    "/Users/dmitry/dev/devcell-sh/devcell",
	})
	if !strings.Contains(cmd, "cd /Users/dmitry/dev/devcell-sh/devcell") {
		t.Errorf("expected cd to mirrored WorkDir, got: %q", cmd)
	}
	if strings.Contains(cmd, "cd ~/devcell") {
		t.Errorf("basename fallback must not be used when WorkDir is set: %q", cmd)
	}
}

func TestBuildExecCommand_EnvVars(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{
		Binary:  "claude",
		EnvVars: []string{"TERM=xterm-256color", "TZ=UTC"},
	})
	if !strings.Contains(cmd, "env") {
		t.Errorf("expected 'env' prefix for env vars, got: %q", cmd)
	}
	if !strings.Contains(cmd, "TERM=xterm-256color") {
		t.Errorf("expected TERM in exec command, got: %q", cmd)
	}
}

func TestBuildExecCommand_RunAsUser(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{
		Binary:    "claude",
		RunAsUser: "dmitry",
	})
	if !strings.Contains(cmd, "sudo -u dmitry -i") {
		t.Errorf("expected 'sudo -u dmitry -i' in exec command, got: %q", cmd)
	}
	if !strings.Contains(cmd, "bash -l -c") {
		t.Errorf("expected 'bash -l -c' wrapper for su, got: %q", cmd)
	}
}

func TestBuildExecCommand_NoRunAsUser(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{Binary: "zsh"})
	if strings.Contains(cmd, "sudo -u") {
		t.Errorf("expected no sudo wrapper when RunAsUser is empty, got: %q", cmd)
	}
}

func TestBuildExecCommand_RunAsUserWithProjectDir(t *testing.T) {
	cmd := tart.BuildExecCommand(tart.ExecSpec{
		Binary:     "claude",
		ProjectDir: "/Users/dmitry/dev/devcell",
		RunAsUser:  "dmitry",
	})
	if !strings.Contains(cmd, "sudo -u dmitry -i") {
		t.Errorf("expected sudo wrapper, got: %q", cmd)
	}
	if !strings.Contains(cmd, "cd ~/devcell") {
		t.Errorf("expected 'cd ~/devcell' inside wrapped command, got: %q", cmd)
	}
}

func TestCreateSessionUserScript(t *testing.T) {
	script := tart.GenerateCreateSessionUserScript("dmitry")
	for _, want := range []string{
		`USERNAME="dmitry"`,
		"dscl . -create",
		"UserShell /bin/zsh",
		"NFSHomeDirectory",
		"dseditgroup",
		"sudoers.d",
		"NOPASSWD",
		"set -e",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("expected %q in create user script, got:\n%s", want, script)
		}
	}
}

func TestCreateSessionUserScript_Idempotent(t *testing.T) {
	script := tart.GenerateCreateSessionUserScript("dmitry")
	if !strings.Contains(script, "already exists") {
		t.Errorf("expected idempotent check in create user script, got:\n%s", script)
	}
}

func TestSetupSessionHomeScript(t *testing.T) {
	script := tart.GenerateSetupSessionHomeScript("dmitry")
	for _, want := range []string{
		"My Shared Files/home",
		"/Users/$USERNAME",
		"set -e",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("expected %q in setup home script, got:\n%s", want, script)
		}
	}
}

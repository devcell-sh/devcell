//go:build darwin || linux

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestPERunnerCommand_AssemblesWSLCommand(t *testing.T) {
	// cell shell --os=windows -- echo hello
	cmd := peGuestCommand(nil, []string{"echo", "hello"})
	assert.Equal(t, `wsl -d winkit -- echo hello`, cmd)
}

func TestPERunnerCommand_InteractiveShell(t *testing.T) {
	cmd := peGuestCommand(nil, nil)
	assert.Equal(t, `wsl -d winkit`, cmd)
}

func TestPERunnerCommand_PreservesArgs(t *testing.T) {
	cmd := peGuestCommand(nil, []string{"cat", "/etc/os-release"})
	assert.Equal(t, `wsl -d winkit -- cat /etc/os-release`, cmd)
}

func TestPERunnerCommand_ForwardsEnvAndAgent(t *testing.T) {
	cmd := peGuestCommand(
		[]string{"TERM=xterm-256color", "GIT_AUTHOR_NAME=Ada Lovelace"},
		[]string{"claude", "--dangerously-skip-permissions", "fix it's bug"},
	)
	assert.Equal(t,
		`wsl -d winkit -- env TERM=xterm-256color 'GIT_AUTHOR_NAME=Ada Lovelace' claude --dangerously-skip-permissions 'fix it'\''s bug'`,
		cmd)
}

func TestPERunnerGuestArgv_AgentFlagsThenUserArgs(t *testing.T) {
	env, argv := peGuestInvocation(
		[]string{"TERM=xterm"},
		map[string]string{"ANTHROPIC_BASE_URL": "http://h:11434", "ANTHROPIC_AUTH_TOKEN": "ollama"},
		"claude", []string{"--dangerously-skip-permissions"}, []string{"-c"},
	)
	assert.Equal(t, []string{"TERM=xterm", "ANTHROPIC_AUTH_TOKEN=ollama", "ANTHROPIC_BASE_URL=http://h:11434"}, env)
	assert.Equal(t, []string{"claude", "--dangerously-skip-permissions", "-c"}, argv)
}

func TestPEStartConfig_UsesAllocatedPorts(t *testing.T) {
	cellCfg := cfg.CellSection{
		QemuCPUs:     8,
		QemuMemoryGB: 16,
	}
	opts := peStartOpts("main", "/home/user", "base", cellCfg, 22122, 23389)
	require.NotEmpty(t, opts.Image)
	assert.Equal(t, uint16(22122), opts.SSHPort)
	assert.Equal(t, uint16(23389), opts.RDPPort)
	assert.Equal(t, uint(8), opts.CPUs)
	assert.Equal(t, uint64(16), opts.MemoryGB)
	assert.Equal(t, "main", opts.Name)
}

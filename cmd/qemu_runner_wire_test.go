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
	// should become: wsl -d winkit -- echo hello
	cmd := peGuestCommand([]string{"echo", "hello"})
	assert.Equal(t, `wsl -d winkit -- echo hello`, cmd)
}

func TestPERunnerCommand_InteractiveShell(t *testing.T) {
	// cell shell --os=windows (no command)
	// should open an interactive WSL shell
	cmd := peGuestCommand(nil)
	assert.Equal(t, `wsl -d winkit`, cmd)
}

func TestPERunnerCommand_PreservesArgs(t *testing.T) {
	cmd := peGuestCommand([]string{"cat", "/etc/os-release"})
	assert.Equal(t, `wsl -d winkit -- cat /etc/os-release`, cmd)
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

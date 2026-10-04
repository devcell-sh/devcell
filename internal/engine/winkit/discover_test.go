package winkit

import (
	"os"
	"testing"
	"time"

	"github.com/devcell-sh/go-winkit/vm/vmstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// saveState writes a winkit state file for cell into its instance dir, the
// way winkit.Start does when devcell passes StateDir: InstanceDir.
func saveState(t *testing.T, home, cell string, pid int) {
	t.Helper()
	require.NoError(t, vmstate.Save(InstanceDir(home, cell), &vmstate.State{
		Name:      cell,
		ImagePath: "/templates/base/winkit-pe-wsl.qcow2",
		PID:       pid,
		Backend:   "qemu",
		StartedAt: time.Now(),
		SSHPort:   10122,
		RDPPort:   10189,
		VNCPort:   10150,
	}))
}

func TestDiscoverRunningVMs_Empty(t *testing.T) {
	assert.Empty(t, DiscoverRunningVMs(t.TempDir()))
}

func TestDiscoverRunningVMs_InstanceDirWithoutState(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(InstanceDir(home, "main"), 0o755))

	assert.Empty(t, DiscoverRunningVMs(home))
}

func TestDiscoverRunningVMs_ReadsWinkitState(t *testing.T) {
	home := t.TempDir()
	saveState(t, home, "main", os.Getpid())

	vms := DiscoverRunningVMs(home)

	require.Len(t, vms, 1)
	assert.Equal(t, "main", vms[0].CellName)
	assert.Equal(t, os.Getpid(), vms[0].PID)
	assert.Equal(t, "/templates/base/winkit-pe-wsl.qcow2", vms[0].ImagePath)
	assert.Equal(t, VMPorts{SSHPort: 10122, VNCPort: 10150, RDPPort: 10189}, vms[0].Ports)
}

func TestDiscoverRunningVMs_MultipleCells(t *testing.T) {
	home := t.TempDir()
	saveState(t, home, "main", os.Getpid())
	saveState(t, home, "work", os.Getpid())

	names := map[string]bool{}
	for _, vm := range DiscoverRunningVMs(home) {
		names[vm.CellName] = true
	}

	assert.Equal(t, map[string]bool{"main": true, "work": true}, names)
}

func TestDiscoverRunningVMs_SkipsDeadPID(t *testing.T) {
	home := t.TempDir()
	saveState(t, home, "main", 999999999)

	assert.Empty(t, DiscoverRunningVMs(home), "a stale state file must not appear as running")
}

// ~/.devcell/windows/ is the template root and ~/.devcell/cache/ the media
// cache; neither is a cell, whatever JSON a stack tag's dir happens to hold.
func TestDiscoverRunningVMs_IgnoresNonCellDirs(t *testing.T) {
	home := t.TempDir()
	saveState(t, home, "windows", os.Getpid())
	saveState(t, home, "cache", os.Getpid())

	assert.Empty(t, DiscoverRunningVMs(home))
}

// Cells get their own winkit state dir, so winkit's own "image already
// running" guard no longer sees VMs from other cells. Two cells on one stack
// share the template's writable data disk and must not boot it twice.
func TestRunningVMForImage_FindsOtherCell(t *testing.T) {
	home := t.TempDir()
	saveState(t, home, "main", os.Getpid())

	vm, ok := RunningVMForImage(home, "/templates/base/winkit-pe-wsl.qcow2")

	require.True(t, ok)
	assert.Equal(t, "main", vm.CellName)
}

func TestRunningVMForImage_NoMatch(t *testing.T) {
	home := t.TempDir()
	saveState(t, home, "main", os.Getpid())

	_, ok := RunningVMForImage(home, "/templates/other/winkit-pe-wsl.qcow2")

	assert.False(t, ok)
}

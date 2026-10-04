package winkit

import (
	"testing"

	gowinkit "github.com/devcell-sh/go-winkit"
	"github.com/stretchr/testify/assert"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
)

// cellWith is a PE cell on stack base with the given [cell] section.
func cellWith(name, home string, section cfg.CellSection) engine.Cell {
	return engine.Cell{Name: name, HostHome: home, Stack: "base", Config: cfg.CellConfig{Cell: section}}
}

func TestPEStartOpts_UsesWinkitImage(t *testing.T) {
	c := cellWith("my-cell", "/home/testuser", cfg.CellSection{
		WinkitCPUs:     8,
		WinkitMemoryGB: 16,
	})
	opts := startOpts(c, guestPE, 22122, 23389, 25950)

	assert.Equal(t, gowinkit.StartOpts{
		Image:    imagePath("/home/testuser", guestPE, "base", nil),
		Name:     "my-cell",
		StateDir: InstanceDir("/home/testuser", "my-cell"),
		SSHPort:  22122,
		RDPPort:  23389,
		VNCPort:  25950,
		CPUs:     8,
		MemoryGB: 16,
	}, opts)
}

// winkit defaults VNC to 5900 when the port is zero, so every cell's VM
// would contend for the same display. The allocated port must reach winkit.
func TestPEStartOpts_VNCPortFromAllocation(t *testing.T) {
	opts := startOpts(cellWith("cell", "/home/u", cfg.CellSection{}), guestPE, 20022, 23389, 10150)

	assert.Equal(t, uint16(10150), opts.VNCPort)
}

// Runtime state belongs to the cell (~/.devcell/<cell>/), not winkit's
// global ~/.winkit/run, so cell vnc/rdp discovery can find it.
func TestPEStartOpts_StateLivesUnderCell(t *testing.T) {
	opts := startOpts(cellWith("work", "/home/u", cfg.CellSection{}), guestPE, 20022, 23389, 10150)

	assert.Equal(t, "/home/u/.devcell/work/windows", opts.StateDir)
}

func TestPEStartOpts_DefaultCPUsAndMemory(t *testing.T) {
	for _, k := range []string{"DEVCELL_WINKIT_CPUS", "DEVCELL_WINKIT_MEMORY_GB", "DEVCELL_QEMU_CPUS", "DEVCELL_QEMU_MEMORY_GB"} {
		t.Setenv(k, "")
	}
	opts := startOpts(cellWith("cell", "/home/u", cfg.CellSection{}), guestPE, 20022, 23389, 10150)

	assert.Equal(t, uint(4), opts.CPUs, "default CPUs from ResolvedWinkitCPUs")
	assert.Equal(t, uint64(4), opts.MemoryGB, "default memory from ResolvedWinkitMemoryGB")
}

func TestPEStartOpts_CPUsAndMemoryFromWinkitEnv(t *testing.T) {
	t.Setenv("DEVCELL_WINKIT_CPUS", "6")
	t.Setenv("DEVCELL_WINKIT_MEMORY_GB", "12")
	opts := startOpts(cellWith("cell", "/home/u", cfg.CellSection{WinkitCPUs: 8}), guestPE, 20022, 23389, 10150)

	assert.Equal(t, uint(6), opts.CPUs)
	assert.Equal(t, uint64(12), opts.MemoryGB)
}

func TestPEImagePath_ContainsPEArtifactName(t *testing.T) {
	p := imagePath("/home/testuser", guestPE, "base", nil)
	assert.Contains(t, p, "winkit-pe-wsl.qcow2")
}

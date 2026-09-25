package main

import (
	"testing"

	winkit "github.com/devcell-sh/go-winkit"
	"github.com/stretchr/testify/assert"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestPEStartOpts_UsesWinkitImage(t *testing.T) {
	cellCfg := cfg.CellSection{
		QemuCPUs:     8,
		QemuMemoryGB: 16,
	}
	opts := peStartOpts("my-cell", "/home/testuser", "base", cellCfg, 22122, 23389)

	assert.Equal(t, winkit.StartOpts{
		Image:    peImagePath("/home/testuser", "base", nil),
		Name:     "my-cell",
		SSHPort:  22122,
		RDPPort:  23389,
		CPUs:     8,
		MemoryGB: 16,
	}, opts)
}

func TestPEStartOpts_DefaultCPUsAndMemory(t *testing.T) {
	cellCfg := cfg.CellSection{}
	opts := peStartOpts("cell", "/home/u", "base", cellCfg, 20022, 23389)

	assert.Equal(t, uint(4), opts.CPUs, "default CPUs from ResolvedQemuCPUs")
	assert.Equal(t, uint64(4), opts.MemoryGB, "default memory from ResolvedQemuMemoryGB")
}

func TestPEImagePath_ContainsPEArtifactName(t *testing.T) {
	p := peImagePath("/home/testuser", "base", nil)
	assert.Contains(t, p, "winkit-core.qcow2")
}

package tart

import (
	"fmt"
)

// BuildConfig holds the parameters for building a macOS VM image.
type BuildConfig struct {
	CellName string
	HomeDir  string
	Stack    string
	Modules  []string
	CPUs     uint
	MemoryGB uint64
	SSHPort  uint16
	Username string
}

// ApplyDefaults fills zero-value fields.
func (c *BuildConfig) ApplyDefaults() {
	if c.CellName == "" {
		c.CellName = "main"
	}
	if c.Stack == "" {
		c.Stack = "base"
	}
	if c.CPUs == 0 {
		c.CPUs = 4
	}
	if c.MemoryGB == 0 {
		c.MemoryGB = 8
	}
	if c.SSHPort == 0 {
		c.SSHPort = 22
	}
	if c.Username == "" {
		c.Username = "admin"
	}
}

// Validate checks required fields.
func (c *BuildConfig) Validate() error {
	if c.HomeDir == "" {
		return fmt.Errorf("HomeDir is required")
	}
	return nil
}

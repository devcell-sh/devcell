package tart

import (
	"fmt"
	"time"
)

// AcquireResult holds the outcome of AcquireDarwinVM.
type AcquireResult struct {
	VM      *VM    // non-nil only if we started a managed VM (caller must shut it down)
	SSHHost string // resolved SSH host (guest IP or external)
	SSHPort uint16 // resolved SSH port
	Managed bool   // true if we started the VM (vs. external/already-running)
}

// AcquireInputs are the parameters for AcquireDarwinVM.
type AcquireInputs struct {
	VMName       string            // tart VM name (instance, e.g. "DIMM-tart")
	TemplateName string            // template VM to clone from if instance doesn't exist
	SharedDirs   map[string]string // tag -> host path (VirtioFS via --dir)
	Disks        []string          // raw disk image paths (VirtIO block devices via --disk)
	SSHHost      string
	SSHPort      uint16
	NoGraphics   bool // pass --no-graphics to tart run (headless); false shows native display
	ExternalVM   bool // user explicitly configured SSH target — skip lifecycle
	SSHTimeout   time.Duration
	InitFunc     func() error // called to auto-init when VM doesn't exist; nil = error instead
}

// ApplyDefaults fills zero values.
func (a *AcquireInputs) ApplyDefaults() {
	if a.SSHHost == "" {
		a.SSHHost = "localhost"
	}
	if a.SSHPort == 0 {
		a.SSHPort = 22
	}
	if a.SSHTimeout == 0 {
		a.SSHTimeout = 120 * time.Second
	}
}

// Validate checks required fields.
func (a *AcquireInputs) Validate() error {
	if a.ExternalVM {
		return nil
	}
	if a.VMName == "" {
		return fmt.Errorf("VMName is required for managed VM")
	}
	return nil
}

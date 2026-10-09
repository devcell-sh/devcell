package tart

import (
	"testing"
)

func TestBuildConfig_Defaults(t *testing.T) {
	c := BuildConfig{HomeDir: "/home/test"}
	c.ApplyDefaults()

	if c.CellName != "main" {
		t.Errorf("CellName = %q, want main", c.CellName)
	}
	if c.Stack != "base" {
		t.Errorf("Stack = %q, want base", c.Stack)
	}
	if c.CPUs != 4 {
		t.Errorf("CPUs = %d, want 4", c.CPUs)
	}
	if c.MemoryGB != 8 {
		t.Errorf("MemoryGB = %d, want 8", c.MemoryGB)
	}
	if c.SSHPort != 22 {
		t.Errorf("SSHPort = %d, want 22", c.SSHPort)
	}
	if c.Username != "admin" {
		t.Errorf("Username = %q, want admin", c.Username)
	}
}

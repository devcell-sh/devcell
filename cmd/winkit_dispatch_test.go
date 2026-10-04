package main

import (
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
)

// --winkit-ssh-port and --winkit-windows-iso override [cell] winkit_ssh_port
// and winkit_windows_iso in the Config the winkit engine gets. The
// deprecated --qemu-* spellings still do.
func TestEngineCell_FoldsWinkitFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--winkit-ssh-port=3333", "--winkit-windows-iso", "/isos/win.iso", "shell"},
		{"--qemu-ssh-port=3333", "--qemu-windows-iso", "/isos/win.iso", "shell"},
	} {
		withOSArgs(t, args...)
		cellCfg := cfg.CellConfig{Cell: cfg.CellSection{WinkitSSHPort: 2222, WinkitWindowsISO: "/old.iso"}}

		got := engineCell(config.Config{}, cellCfg).Config.Cell

		if got.WinkitSSHPort != 3333 {
			t.Errorf("%v: WinkitSSHPort = %d, want 3333", args, got.WinkitSSHPort)
		}
		if got.WinkitWindowsISO != "/isos/win.iso" {
			t.Errorf("%v: WinkitWindowsISO = %q, want /isos/win.iso", args, got.WinkitWindowsISO)
		}
		if cellCfg.Cell.WinkitSSHPort != 2222 {
			t.Errorf("%v: engineCell modified the caller's config", args)
		}
	}
}

func TestEngineCell_WinkitFlagWinsOverQemuFlag(t *testing.T) {
	withOSArgs(t, "--qemu-windows-iso=/old.iso", "--winkit-windows-iso=/new.iso", "shell")
	if got := engineCell(config.Config{}, cfg.CellConfig{}).Config.Cell.WinkitWindowsISO; got != "/new.iso" {
		t.Errorf("WinkitWindowsISO = %q, want /new.iso", got)
	}
}

func TestEngineCell_IgnoresInvalidWinkitSSHPort(t *testing.T) {
	withOSArgs(t, "--winkit-ssh-port", "ssh", "shell")
	cellCfg := cfg.CellConfig{Cell: cfg.CellSection{WinkitSSHPort: 2200}}
	if got := engineCell(config.Config{}, cellCfg).Config.Cell.WinkitSSHPort; got != 2200 {
		t.Errorf("WinkitSSHPort = %d, want the [cell] value 2200", got)
	}
}

func TestWinkitEngineIsLinkedIn(t *testing.T) {
	if _, err := engine.For(engine.Winkit); err != nil {
		t.Errorf("engine.For(winkit): %v", err)
	}
}

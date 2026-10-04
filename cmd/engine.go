package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine"
	_ "github.com/DimmKirr/devcell/internal/engine/docker" // registers engine.Docker
	_ "github.com/DimmKirr/devcell/internal/engine/tart"   // registers engine.Tart
	_ "github.com/DimmKirr/devcell/internal/engine/winkit" // registers engine.Winkit
	"github.com/DimmKirr/devcell/internal/ux"
)

// resolveEngine picks the engine from --engine, --os and the [cell] engine
// and os keys, in engine.Resolve's precedence. The deprecated --macos flag
// stands for --os macos when --os is unset; its own warning comes from
// warnDeprecatedFlags or cobra. A deprecated engine name ("qemu") warns on
// w here, once per invocation.
func resolveEngine(w io.Writer, cellCfg cfg.CellConfig) (engine.Name, error) {
	n, deprecation, err := engine.Resolve(scanStringFlag("--engine"), scanOSFlag(), cellCfg.Cell.Engine, cellCfg.Cell.OS)
	if err != nil {
		return "", err
	}
	if deprecation != "" {
		ux.DeprecatedLine(w, deprecation)
	}
	return n, nil
}

// scanOSFlag returns the --os value. The deprecated --macos flag stands for
// --os macos when --os is unset.
func scanOSFlag() string {
	if v := scanStringFlag("--os"); v != "" {
		return v
	}
	if scanFlag("--macos") {
		return string(engine.MacOS)
	}
	return ""
}

// engineCell is the engine.Cell for cell c with the merged config cellCfg.
//
// Engine flags that override a [cell] key are folded into the engine's copy
// of cellCfg: --winkit-ssh-port and --winkit-windows-iso set
// winkit_ssh_port and winkit_windows_iso. A non-numeric --winkit-ssh-port is
// ignored. The DEVCELL_WINKIT_* env vars still win over both, as they do
// over the TOML keys. Guest is the one the cell's engine runs (see
// cellGuest).
func engineCell(c config.Config, cellCfg cfg.CellConfig) engine.Cell {
	if v := winkitFlag("--winkit-ssh-port", "--qemu-ssh-port"); v != "" { // --qemu-ssh-port: deprecated spelling
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cellCfg.Cell.WinkitSSHPort = port
		}
	}
	if v := winkitFlag("--winkit-windows-iso", "--qemu-windows-iso"); v != "" { // --qemu-windows-iso: deprecated spelling
		cellCfg.Cell.WinkitWindowsISO = v
	}
	return engine.Cell{
		Name:          c.CellName,
		Home:          c.CellHome,
		HostHome:      c.HostHome,
		HostUser:      c.HostUser,
		BaseDir:       c.BaseDir,
		ConfigDir:     c.ConfigDir,
		BuildDir:      c.BuildDir,
		Config:        cellCfg,
		Stack:         cellCfg.Cell.ResolvedStack(),
		Modules:       cellCfg.Cell.Modules,
		VNCPort:       c.VNCPort,
		RDPPort:       c.RDPPort,
		Bunk:          c.Bunk,
		AppName:       c.AppName,
		ContainerName: c.ContainerName,
		Hostname:      c.Hostname,
		Guest:         cellGuest(cellCfg),
	}
}

// winkitFlag returns the value of flag, else of its deprecated spelling
// (which warnDeprecatedFlags or cobra warns about).
func winkitFlag(flag, deprecated string) string {
	if v := scanStringFlag(flag); v != "" {
		return v
	}
	return scanStringFlag(deprecated)
}

// cellGuest is the guest the cell's engine runs: the first of --os and
// [cell] os that names a guest the engine supports, else the engine's
// default guest (engine.GuestFor). A stale [cell] os under an explicit
// --engine is skipped rather than handed to an engine that cannot run it.
//
// The engine is resolved again here without warnings: callers already
// resolved it, and warned, with resolveEngine. It is empty only when the
// engine does not resolve, which resolveEngine has already reported.
func cellGuest(cellCfg cfg.CellConfig) engine.Guest {
	osFlag := scanOSFlag()
	n, _, err := engine.Resolve(scanStringFlag("--engine"), osFlag, cellCfg.Cell.Engine, cellCfg.Cell.OS)
	if err != nil {
		return ""
	}
	return engine.GuestFor(n, osFlag, cellCfg.Cell.OS)
}

// exitProcess is os.Exit; tests replace it.
var exitProcess = os.Exit

// runEngine runs the agent on engine n. When the agent exits non-zero it
// exits the process with the agent's status; any other error is returned.
func runEngine(n engine.Name, opts engine.RunOpts) error {
	e, err := engine.For(n)
	if err != nil {
		return err
	}
	err = e.Run(context.Background(), opts)
	var exitErr *engine.ExitError
	if errors.As(err, &exitErr) {
		exitProcess(exitErr.Code)
	}
	return err
}

package main

// Shared implementation of `cell rdp` and `cell vnc`. Both find a running
// cell (docker container or winkit VM) that publishes the protocol's port
// and hand its host port to a protocol-specific viewer: openRDP in rdp.go,
// openVNC in vnc.go.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/DimmKirr/devcell/internal/config"
	"github.com/DimmKirr/devcell/internal/engine/winkit"
	"github.com/DimmKirr/devcell/internal/gui"
	"github.com/DimmKirr/devcell/internal/ux"
)

// guiCommand holds what `cell rdp` and `cell vnc` differ in beyond what
// gui.Protocol carries.
type guiCommand struct {
	qemuPort  func(winkit.VMPorts) uint16
	emptyList string // text-mode --list output when no cell is running
}

var guiCommands = map[gui.Protocol]guiCommand{
	gui.RDP: {
		qemuPort:  func(p winkit.VMPorts) uint16 { return p.RDPPort },
		emptyList: "No running cell containers with RDP found.",
	},
	gui.VNC: {
		qemuPort:  func(p winkit.VMPorts) uint16 { return p.VNCPort },
		emptyList: "No running cell containers found.",
	},
}

// configLoader returns the project config. The shared code calls it only
// where it needs the config: `cell vnc` passes config.LoadFromOS so the
// config is read lazily, `cell rdp` (whose viewer always needs it) passes
// the one it already loaded via loadedConfig.
type configLoader func() (config.Config, error)

func loadedConfig(c config.Config) configLoader {
	return func() (config.Config, error) { return c, nil }
}

// cellPort returns the host port to connect to: that of the cell named by
// args[0] (a full cell name or its numeric suffix) or, with no args, that of
// the default cell.
func cellPort(p gui.Protocol, args []string, global bool, load configLoader) (string, error) {
	if len(args) > 0 {
		return namedCellPort(p, resolveAppArg(args[0]))
	}
	return defaultCellPort(p, global, load)
}

// namedCellPort returns the host port of the named docker cell, looked up
// with docker inspect.
func namedCellPort(p gui.Protocol, name string) (string, error) {
	containerName := "cell-" + name + "-run"
	out, err := exec.Command("docker", "inspect", containerName).Output()
	if err != nil {
		return "", fmt.Errorf("container %q not found: %w", containerName, err)
	}
	port, err := p.ParseInspectPort(string(out))
	if err != nil {
		return "", fmt.Errorf("%s port not published for %q: %w", p.Label(), name, err)
	}
	return port, nil
}

// defaultCellPort returns the host port to connect to when no cell is named:
// the protocol's port env var when run inside a cell, else the only running
// cell's port, else the port of the cell the user picks.
func defaultCellPort(p gui.Protocol, global bool, load configLoader) (string, error) {
	// Fast path: the port env var is injected at container start with the
	// published host port. When set, we are inside a devcell container.
	if port := os.Getenv(p.PortEnv()); port != "" {
		guiDebug(p, "%s=%s (fast path)", p.PortEnv(), port)
		return port, nil
	}

	c, err := load()
	if err != nil {
		return "", err
	}
	guiDebug(p, "basedir: %s  bunk: %s", c.BaseDir, c.Bunk)

	cells := collectCells(p, c, global)
	guiDebug(p, "found %d cells: %v", len(cells), cells)

	switch len(cells) {
	case 0:
		return "", fmt.Errorf("no running cell found for %q — run 'cell %s --list' to see all", c.BaseDir, p)
	case 1:
		for name, port := range cells {
			guiDebug(p, "auto-selecting only cell: %s (port %s)", name, port)
			return port, nil
		}
	}
	guiDebug(p, "multiple cells — showing picker")
	selected, err := selectCell(cells)
	if err != nil {
		return "", err
	}
	guiDebug(p, "selected: %s (port %s)", selected, cells[selected])
	return cells[selected], nil
}

// listCells implements --list.
func listCells(p gui.Protocol, global bool) error {
	c, err := config.LoadFromOS()
	if err != nil {
		return err
	}
	return renderCellList(p, collectCells(p, c, global))
}

// renderCellList renders a cell name to host port map in the current
// OutputFormat. Split from listCells so tests need no docker daemon.
func renderCellList(p gui.Protocol, m map[string]string) error {
	headers := []string{"APP_NAME", "PORT", "URL"}
	if len(m) == 0 {
		if ux.OutputFormat != "text" {
			ux.PrintTable(headers, nil)
		} else {
			fmt.Println(guiCommands[p].emptyList)
		}
		return nil
	}
	var rows [][]string
	for cell, port := range m {
		rows = append(rows, []string{cell, port, p.URL(port)})
	}
	ux.PrintTable(headers, rows)
	return nil
}

// collectCells returns a map of cell name to host port for running cells
// that publish p's port. When global is false only the current project's
// docker containers are included; when true, all docker cells. Winkit VMs
// are always included, since there is one per cell rather than per project.
func collectCells(p gui.Protocol, c config.Config, global bool) map[string]string {
	result := make(map[string]string)
	guiDebug(p, "collect%sCells: global=%v baseDir=%s", p.Label(), global, c.BaseDir)

	if global {
		guiDebug(p, "docker: scanning all cell- containers")
		collectDockerCells(p, "cell-", result)
	} else {
		// Current project docker cells only, filtered by project prefix (all cell IDs)
		projectPrefix := "cell-" + filepath.Base(c.BaseDir) + "-"
		guiDebug(p, "docker: scanning with filter name=%s", projectPrefix)
		collectDockerCells(p, projectPrefix, result)
	}

	guiDebug(p, "qemu: scanning for running VMs")
	for _, vm := range winkit.DiscoverRunningVMs(c.HostHome) {
		if n := guiCommands[p].qemuPort(vm.Ports); n > 0 {
			name := "qemu-" + vm.CellName
			port := strconv.Itoa(int(n))
			guiDebug(p, "qemu cell found: %s → %s", name, port)
			result[name] = port
		}
	}

	guiDebug(p, "collect%sCells result: %v", p.Label(), result)
	return result
}

// collectDockerCells adds to result every running docker cell whose name
// matches filter and that publishes p's port.
func collectDockerCells(p gui.Protocol, filter string, result map[string]string) {
	out, err := exec.Command("docker", "ps",
		"--filter", "name="+filter,
		"--format", "{{.Names}}\t{{.Ports}}").Output()
	if err != nil {
		guiDebug(p, "docker ps error: %v", err)
		return
	}
	guiDebug(p, "docker ps output (%d bytes): %s", len(out), bytes.TrimSpace(out))
	dm, _ := p.ParseDockerPS(string(bytes.TrimSpace(out)))
	for name, port := range dm {
		guiDebug(p, "docker cell found: %s → %s", name, port)
		result[name] = port
	}
}

// guiDebug prints a "[rdp] " or "[vnc] " prefixed line when --verbose is active.
func guiDebug(p gui.Protocol, format string, args ...any) {
	if ux.Verbose {
		fmt.Fprintf(os.Stderr, "["+string(p)+"] "+format+"\n", args...)
	}
}

// openURL prints url and, on macOS, opens it with the registered handler.
func openURL(url string) error {
	fmt.Println(url)
	if runtime.GOOS != "darwin" {
		return nil
	}
	return exec.Command("open", url).Run()
}

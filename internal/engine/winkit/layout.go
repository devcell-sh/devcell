package winkit

import (
	"path/filepath"

	"github.com/DimmKirr/devcell/internal/cell"
)

// templateDir returns the directory holding guest g's template for a stack
// and modules. Every template lives under ~/.devcell/windows/, which
// DiscoverRunningVMs skips as a cell name:
//
//	WindowsPE:   ~/.devcell/windows/<stackTag>/
//	WindowsFull: ~/.devcell/windows/full-wsl/<stackTag>/
//
// PE keeps the layout it had before the Full guest existed, so templates
// already built (hours each) stay where Run looks for them.
func templateDir(home string, g guest, stack string, modules []string) string {
	root := filepath.Join(home, ".devcell", "windows")
	if g.full {
		root = filepath.Join(root, "full-wsl")
	}
	return filepath.Join(root, cell.StackTag(stack, modules))
}

// InstanceDir returns the per-cell instance directory for Windows VMs. It is
// also winkit's state dir for the cell's VM (StartOpts.StateDir).
// Layout: ~/.devcell/<cellName>/windows/
func InstanceDir(home, cellName string) string {
	return filepath.Join(home, ".devcell", cellName, "windows")
}

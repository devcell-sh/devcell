package tart

import (
	"path/filepath"
)

// CellHome returns the per-cell persistent home directory.
// Layout: ~/.devcell/<cellName>/
// Same path on both Linux (bind-mount → /home/<user>) and macOS (VirtioFS → /Users/<user>).
func CellHome(home, cellName string) string {
	return filepath.Join(home, ".devcell", cellName)
}

// CellSSHPaths holds paths to per-cell SSH keys.
// Layout: ~/.devcell/<cellName>/.ssh/
type CellSSHPaths struct {
	Dir        string // ~/.devcell/<cellName>/.ssh/
	PrivateKey string // id_ed25519
	PublicKey  string // id_ed25519.pub
}

// NewCellSSHPaths returns paths for a cell's SSH keys.
func NewCellSSHPaths(home, cellName string) CellSSHPaths {
	dir := filepath.Join(CellHome(home, cellName), ".ssh")
	key := filepath.Join(dir, "id_ed25519")
	return CellSSHPaths{
		Dir:        dir,
		PrivateKey: key,
		PublicKey:  key + ".pub",
	}
}

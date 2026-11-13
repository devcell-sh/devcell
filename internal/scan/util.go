package scan

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devcell-sh/devcell/internal/config"
)

// MustCwd returns the current working directory or "." on error.
func MustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// DirNonEmpty reports whether path is a non-empty directory.
func DirNonEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

// DirSize returns the total size in bytes of all files under path.
func DirSize(path string) int64 {
	var total int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// ResolveWorkspaces returns workspace names under devcellHome.
// If wsFlag is set, returns just that workspace (validated).
// If all is true, returns every subdirectory except "cache".
// Otherwise returns the workspace from the current project config.
func ResolveWorkspaces(devcellHome, wsFlag string, all bool) ([]string, error) {
	if wsFlag != "" {
		info, err := os.Stat(filepath.Join(devcellHome, wsFlag))
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("workspace %q not found in %s", wsFlag, devcellHome)
		}
		return []string{wsFlag}, nil
	}
	if all {
		entries, err := os.ReadDir(devcellHome)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", devcellHome, err)
		}
		var ws []string
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "cache" {
				continue
			}
			ws = append(ws, e.Name())
		}
		return ws, nil
	}
	c := config.Load(MustCwd(), os.Getenv)
	return []string{c.WorkspaceName}, nil
}

package docker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devcell-sh/devcell/internal/config"
	"github.com/devcell-sh/devcell/internal/ux"
)

// solitaryRunPrefix returns the shared timestamp-cellID prefix for a
// solitary run's directories under .devcell/solitary/.
func solitaryRunPrefix(cellID string) string {
	return time.Now().UTC().Format("20060102T150405Z") + "-" + cellID
}

// solitaryDir returns a path under .devcell/solitary/ with the given suffix.
func solitaryDir(baseDir, prefix, suffix string) string {
	return filepath.Join(baseDir, ".devcell", "solitary", prefix+"-"+suffix)
}

// createSolitaryDir creates an empty directory under .devcell/solitary/
// for the given suffix (e.g. "project", "home") and returns its path.
func createSolitaryDir(baseDir, prefix, suffix string) (string, error) {
	dst := solitaryDir(baseDir, prefix, suffix)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", fmt.Errorf("solitary %s: mkdir: %w", suffix, err)
	}
	return dst, nil
}

// cloneProject copies the project directory into an isolated clone under
// .devcell/solitary/<prefix>-project/ and returns a new config.Config
// with HostBaseDir pointing to the clone. Skips .devcell/ entirely
// (build artifacts, nix symlinks, prior clones). Symlinks are preserved.
func cloneProject(c config.Config, prefix string) (config.Config, error) {
	dst := solitaryDir(c.BaseDir, prefix, "project")
	ux.Debugf("solitary: cloning project %s → %s", c.BaseDir, dst)

	if err := copyTree(c.BaseDir, dst, ".devcell"); err != nil {
		return c, fmt.Errorf("solitary project clone: %w", err)
	}

	clone := c
	clone.HostBaseDir = dst
	return clone, nil
}

// cloneDir copies an arbitrary source directory into .devcell/solitary/
// under the given suffix and returns the destination path.
func cloneDir(src, baseDir, prefix, suffix string) (string, error) {
	dst := solitaryDir(baseDir, prefix, suffix)
	ux.Debugf("solitary: cloning %s %s → %s", suffix, src, dst)

	if err := copyTree(src, dst, ""); err != nil {
		return "", fmt.Errorf("solitary %s clone: %w", suffix, err)
	}
	return dst, nil
}

// copyTree recursively copies src into dst, preserving modes and symlinks.
// If skipRel is non-empty, any path matching that relative prefix is skipped.
func copyTree(src, dst, skipRel string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}

		if skipRel != "" && (rel == skipRel || strings.HasPrefix(rel, skipRel+string(filepath.Separator))) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)

		if d.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

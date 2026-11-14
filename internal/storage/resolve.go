package storage

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

const (
	GuestBase    = "/var/cache/devcell"
	GuestGoMod   = GuestBase + "/go/mod"
	GuestGoBuild = GuestBase + "/go/build"
	GuestNpm     = GuestBase + "/npm"
	GuestUv      = GuestBase + "/uv"
	GuestWinkit  = GuestBase + "/winkit"
	GuestMise    = GuestBase + "/mise"
)

// Mount is one host:guest bind-mount pair with its env var name.
type Mount struct {
	HostPath  string
	GuestPath string
	EnvVar    string
}

// Plan holds resolved cache paths for one cell.
type Plan struct {
	GoMod   Mount
	GoBuild Mount
	Npm     Mount
	Uv      Mount
	Mise    Mount
	Winkit  Mount
}

// Mounts returns all mounts in a stable order.
func (p Plan) Mounts() []Mount {
	return []Mount{p.GoMod, p.GoBuild, p.Npm, p.Uv, p.Mise, p.Winkit}
}

// Input holds the values needed to resolve a storage plan.
type Input struct {
	CacheDir      string // resolved absolute host cache root
	SharedCache   bool
	WorkspaceName string
	CellName      string // <projectBasename>-<cellID>
	BaseDir       string // canonical absolute host project path
	StackTag      string // cell.StackTag(stack, modules)
}

// CellKey returns a collision-safe directory name for per-cell storage.
// Format: <workspace>/<cellName>-<sha8(baseDir)>.
func CellKey(workspaceName, cellName, baseDir string) string {
	h := sha256.Sum256([]byte(baseDir))
	return filepath.Join(workspaceName, fmt.Sprintf("%s-%x", cellName, h[:4]))
}

// Resolve computes the storage plan for a cell.
func Resolve(in Input) Plan {
	scope := sharedScope(in.CacheDir)
	if !in.SharedCache {
		scope = privateScope(in.CacheDir, CellKey(in.WorkspaceName, in.CellName, in.BaseDir))
	}

	return Plan{
		GoMod: Mount{
			HostPath:  goModPath(in.CacheDir, scope, in.SharedCache),
			GuestPath: GuestGoMod,
			EnvVar:    "GOMODCACHE",
		},
		GoBuild: Mount{
			HostPath:  envScopedPath(scope, in.StackTag, "go-build", in.SharedCache),
			GuestPath: GuestGoBuild,
			EnvVar:    "GOCACHE",
		},
		Npm: Mount{
			HostPath:  envScopedPath(scope, in.StackTag, "npm", in.SharedCache),
			GuestPath: GuestNpm,
			EnvVar:    "npm_config_cache",
		},
		Uv: Mount{
			HostPath:  envScopedPath(scope, in.StackTag, "uv", in.SharedCache),
			GuestPath: GuestUv,
			EnvVar:    "UV_CACHE_DIR",
		},
		Mise: Mount{
			HostPath:  MiseCachePath(in.CacheDir),
			GuestPath: GuestMise,
			EnvVar:    "MISE_DATA_DIR",
		},
		Winkit: Mount{
			HostPath:  winkitPath(in.CacheDir),
			GuestPath: GuestWinkit,
			EnvVar:    "DEVCELL_WINKIT_CACHE_DIR",
		},
	}
}

func sharedScope(cacheDir string) string {
	return filepath.Join(cacheDir, "v1", "shared")
}

func privateScope(cacheDir, cellKey string) string {
	return filepath.Join(cacheDir, "v1", "private", cellKey)
}

func goModPath(cacheDir, scope string, shared bool) string {
	if shared {
		return filepath.Join(cacheDir, "v1", "shared", "go", "mod")
	}
	return filepath.Join(scope, "go", "mod")
}

func envScopedPath(scope, stackTag, tool string, shared bool) string {
	if shared {
		return filepath.Join(scope, "env", stackTag, tool)
	}
	return filepath.Join(scope, tool)
}

func winkitPath(cacheDir string) string {
	return filepath.Join(cacheDir, "v1", "shared", "winkit")
}

// MiseCachePath returns the shared mise install cache directory on the host.
func MiseCachePath(cacheDir string) string {
	return filepath.Join(cacheDir, "v1", "shared", "mise")
}

// EnsureHostDirs creates all host-side cache directories.
func EnsureHostDirs(p Plan) error {
	for _, m := range p.Mounts() {
		if err := os.MkdirAll(m.HostPath, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", m.HostPath, err)
		}
	}
	return nil
}

// FindOldWinkitCaches finds winkit cache in old per-cell locations.
// Old location: ~/.devcell/<cell>/.devcell/cache/qemu/
func FindOldWinkitCaches(devcellHome string) ([]string, error) {
	var found []string
	entries, err := os.ReadDir(devcellHome)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		old := filepath.Join(devcellHome, e.Name(), ".devcell", "cache", "qemu")
		if fi, err := os.Stat(old); err == nil && fi.IsDir() {
			children, _ := os.ReadDir(old)
			if len(children) > 0 {
				found = append(found, old)
			}
		}
	}
	return found, nil
}

// MigrateWinkitCache moves files from an old per-cell cache dir to the new
// shared dir. Files that already exist at the destination are skipped.
func MigrateWinkitCache(oldDir, newDir string) error {
	return migrateCache(oldDir, newDir, true)
}

// MackitPath returns the unified mackit cache dir.
func MackitPath(cacheDir string) string {
	return filepath.Join(cacheDir, "v1", "shared", "mackit")
}

// FindOldMackitCache checks the default mackit cache at <userHome>/.cache/mackit.
// Returns the path and true when the dir exists and has files.
func FindOldMackitCache(userHome string) (string, bool) {
	dir := filepath.Join(userHome, ".cache", "mackit")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "", false
	}
	return dir, true
}

// MigrateMackitCache moves files from the old ~/.cache/mackit to the unified
// shared mackit cache dir. Files that already exist are skipped.
func MigrateMackitCache(oldDir, newDir string) error {
	return migrateCache(oldDir, newDir, false)
}

// migrateCache moves files from oldDir to newDir. Files that already exist at
// the destination are removed from oldDir. When removeParent is true, the
// parent of oldDir is also removed (for winkit's .devcell/cache/ nesting).
func migrateCache(oldDir, newDir string, removeParent bool) error {
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", newDir, err)
	}
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		src := filepath.Join(oldDir, e.Name())
		dst := filepath.Join(newDir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			os.Remove(src)
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("move %s -> %s: %w", src, dst, err)
		}
	}
	os.Remove(oldDir)
	if removeParent {
		os.Remove(filepath.Dir(oldDir))
	}
	return nil
}

// OldUnifiedCacheDir returns the pre-XDG unified cache path (~/.devcell/cache).
func OldUnifiedCacheDir(userHome string) string {
	return filepath.Join(userHome, ".devcell", "cache")
}

// MigrateUnifiedCache moves the v1/ tree from oldRoot to newRoot.
// Operates on the top-level "v1" directory: if newRoot/v1 already exists,
// sub-entries are merged (existing files win). Returns true if migration ran.
func MigrateUnifiedCache(oldRoot, newRoot string) (bool, error) {
	oldV1 := filepath.Join(oldRoot, "v1")
	fi, err := os.Stat(oldV1)
	if err != nil || !fi.IsDir() {
		return false, nil
	}
	newV1 := filepath.Join(newRoot, "v1")
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", newRoot, err)
	}
	if _, err := os.Stat(newV1); os.IsNotExist(err) {
		if err := os.Rename(oldV1, newV1); err != nil {
			return false, fmt.Errorf("move %s -> %s: %w", oldV1, newV1, err)
		}
	} else {
		if err := mergeDir(oldV1, newV1); err != nil {
			return false, err
		}
	}
	os.RemoveAll(oldRoot)
	return true, nil
}

// mergeDir recursively moves entries from src into dst. Existing files
// at the destination are kept (src copy removed).
func mergeDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := mergeDir(s, d); err != nil {
				return err
			}
			os.Remove(s)
			continue
		}
		if _, err := os.Stat(d); err == nil {
			os.Remove(s)
			continue
		}
		if err := os.Rename(s, d); err != nil {
			return fmt.Errorf("move %s -> %s: %w", s, d, err)
		}
	}
	return nil
}

// oldWorkspaceCacheRelPaths lists cache directories that accumulate in the
// workspace home (container's $HOME) before unified storage redirects them.
var oldWorkspaceCacheRelPaths = []string{
	"go/pkg/mod",
	".cache/go-build",
	".npm",
	".cache/uv",
}

// FindOldWorkspaceCaches returns absolute paths of non-empty old cache dirs
// found under workspaceHome.
func FindOldWorkspaceCaches(workspaceHome string) []string {
	var found []string
	for _, rel := range oldWorkspaceCacheRelPaths {
		dir := filepath.Join(workspaceHome, rel)
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) == 0 {
			continue
		}
		found = append(found, dir)
	}
	return found
}

// CleanOldWorkspaceCaches removes each listed directory tree. Missing dirs
// are silently skipped. Read-only trees (e.g. Go module cache) are made
// writable before removal.
func CleanOldWorkspaceCaches(dirs []string) error {
	for _, d := range dirs {
		if err := ForceRemoveAll(d); err != nil {
			return fmt.Errorf("remove %s: %w", d, err)
		}
	}
	return nil
}

// ForceRemoveAll makes every directory in the tree writable, then removes it.
func ForceRemoveAll(dir string) error {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Mode()&0o200 == 0 {
			os.Chmod(path, info.Mode()|0o200)
		}
		return nil
	})
	err := os.RemoveAll(dir)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

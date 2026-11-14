package nixhome

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

const preflightCacheFile = ".preflight-ok"

// PreflightCacheValid returns true when a prior platform check succeeded for
// the same flake inputs. The cache key is a hash of flake.nix + flake.lock in
// buildDir. Any I/O error or missing file returns false (cache miss).
func PreflightCacheValid(buildDir string) bool {
	key, ok := preflightCacheKey(buildDir)
	if !ok {
		return false
	}
	stored, err := os.ReadFile(filepath.Join(buildDir, preflightCacheFile))
	if err != nil {
		return false
	}
	return string(stored) == key
}

// PreflightCacheWrite records a successful platform check so subsequent builds
// with the same flake inputs can skip it.
func PreflightCacheWrite(buildDir string) {
	key, ok := preflightCacheKey(buildDir)
	if !ok {
		return
	}
	_ = os.WriteFile(filepath.Join(buildDir, preflightCacheFile), []byte(key), 0o644)
}

func preflightCacheKey(buildDir string) (string, bool) {
	h := sha256.New()
	paths := []string{
		filepath.Join(buildDir, "flake.nix"),
		filepath.Join(buildDir, "flake.lock"),
		filepath.Join(buildDir, "nixhome", "flake.lock"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", false
		}
		h.Write([]byte(p))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

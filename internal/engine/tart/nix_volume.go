package tart

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DimmKirr/devcell/internal/ux"
)

const (
	NixVolumeFileName = "nix.img"
	NixVolumeSizeGB   = 100
	NixVolumeLabel    = "DevcellNix"
)

// IsNixStore checks whether the given path contains a functional Nix store
// (has store/ directory and var/nix/db/db.sqlite database).
func IsNixStore(nixPath string) bool {
	storePath := filepath.Join(nixPath, "store")
	dbPath := filepath.Join(nixPath, "var", "nix", "db", "db.sqlite")
	info, err := os.Stat(storePath)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(dbPath)
	return err == nil
}

// DetectHostNixStore returns "/nix" if the host has a functional Nix store,
// empty string otherwise.
func DetectHostNixStore() string {
	if IsNixStore("/nix") {
		return "/nix"
	}
	return ""
}

// NixVolumePath returns the path to the global nix store disk image.
// Layout: ~/.devcell/darwin/nix-store.img
// Shared across all cells — mirrors Docker's single devcell-nix-store volume.
func NixVolumePath(home string) string {
	return filepath.Join(home, ".devcell", "darwin", NixVolumeFileName)
}

// EnsureNixVolume creates a sparse nix store disk image if it doesn't exist.
// Returns the path to the (existing or newly created) image.
// Callers should log the returned path and whether it was created or reused.
func EnsureNixVolume(home string) (string, error) {
	imgPath := NixVolumePath(home)
	wantBytes := int64(NixVolumeSizeGB) * 1024 * 1024 * 1024
	if info, err := os.Stat(imgPath); err == nil {
		currentGB := info.Size() / (1024 * 1024 * 1024)
		ux.Debugf("nix volume exists: %s (%d GB logical)", imgPath, currentGB)
		if info.Size() < wantBytes {
			ux.Debugf("nix volume undersized (%d GB < %d GB) — growing sparse image", currentGB, NixVolumeSizeGB)
			if err := os.Truncate(imgPath, wantBytes); err != nil {
				return "", fmt.Errorf("resizing nix volume from %dGB to %dGB: %w", currentGB, NixVolumeSizeGB, err)
			}
			ux.Debugf("nix volume resized to %d GB (sparse — no extra host disk used)", NixVolumeSizeGB)
		}
		return imgPath, nil
	}
	dir := filepath.Dir(imgPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating artifact dir: %w", err)
	}
	f, err := os.Create(imgPath)
	if err != nil {
		return "", fmt.Errorf("creating nix volume image: %w", err)
	}
	defer f.Close()
	sizeBytes := int64(NixVolumeSizeGB) * 1024 * 1024 * 1024
	if err := f.Truncate(sizeBytes); err != nil {
		os.Remove(imgPath)
		return "", fmt.Errorf("setting nix volume size: %w", err)
	}
	ux.Debugf("nix volume created: %s (sparse %d GB)", imgPath, NixVolumeSizeGB)
	return imgPath, nil
}

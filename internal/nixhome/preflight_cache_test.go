package nixhome_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devcell-sh/devcell/internal/nixhome"
)

func TestPreflightCacheValid_MissOnEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFlakeFiles(t, dir, "v1", "lock1", "nhlock1")
	if nixhome.PreflightCacheValid(dir) {
		t.Error("cache should miss when no marker exists")
	}
}

func TestPreflightCacheValid_HitAfterWrite(t *testing.T) {
	dir := t.TempDir()
	writeFlakeFiles(t, dir, "v1", "lock1", "nhlock1")
	nixhome.PreflightCacheWrite(dir)
	if !nixhome.PreflightCacheValid(dir) {
		t.Error("cache should hit after write with same flake files")
	}
}

func TestPreflightCacheValid_MissAfterFlakeLockChange(t *testing.T) {
	dir := t.TempDir()
	writeFlakeFiles(t, dir, "v1", "lock1", "nhlock1")
	nixhome.PreflightCacheWrite(dir)

	writeFlakeFiles(t, dir, "v1", "lock2", "nhlock1")
	if nixhome.PreflightCacheValid(dir) {
		t.Error("cache should miss after flake.lock changed")
	}
}

func TestPreflightCacheValid_MissAfterFlakeNixChange(t *testing.T) {
	dir := t.TempDir()
	writeFlakeFiles(t, dir, "v1", "lock1", "nhlock1")
	nixhome.PreflightCacheWrite(dir)

	writeFlakeFiles(t, dir, "v2", "lock1", "nhlock1")
	if nixhome.PreflightCacheValid(dir) {
		t.Error("cache should miss after flake.nix changed")
	}
}

func TestPreflightCacheValid_MissAfterNixhomeLockChange(t *testing.T) {
	dir := t.TempDir()
	writeFlakeFiles(t, dir, "v1", "lock1", "nhlock1")
	nixhome.PreflightCacheWrite(dir)

	writeFlakeFiles(t, dir, "v1", "lock1", "nhlock2")
	if nixhome.PreflightCacheValid(dir) {
		t.Error("cache should miss after nixhome/flake.lock changed")
	}
}

func TestPreflightCacheValid_MissWhenFlakeLockMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("nix"), 0o644); err != nil {
		t.Fatal(err)
	}
	if nixhome.PreflightCacheValid(dir) {
		t.Error("cache should miss when flake.lock is absent")
	}
}

func writeFlakeFiles(t *testing.T, dir, nixContent, lockContent, nixhomeLock string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "flake.nix"), []byte(nixContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "flake.lock"), []byte(lockContent), 0o644); err != nil {
		t.Fatal(err)
	}
	nhDir := filepath.Join(dir, "nixhome")
	if err := os.MkdirAll(nhDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nhDir, "flake.lock"), []byte(nixhomeLock), 0o644); err != nil {
		t.Fatal(err)
	}
}

package tart

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestNixVolumePath(t *testing.T) {
	got := NixVolumePath("/home/user")
	want := filepath.Join("/home/user", ".devcell", "darwin", "nix.img")
	if got != want {
		t.Errorf("NixVolumePath = %q, want %q", got, want)
	}
}

func TestEnsureNixVolume_CreatesSparseDisk(t *testing.T) {
	home := t.TempDir()

	imgPath, err := EnsureNixVolume(home)
	if err != nil {
		t.Fatalf("EnsureNixVolume failed: %v", err)
	}

	info, err := os.Stat(imgPath)
	if err != nil {
		t.Fatalf("image file not created: %v", err)
	}

	wantSize := int64(NixVolumeSizeGB) * 1024 * 1024 * 1024
	if info.Size() != wantSize {
		t.Errorf("image logical size = %d, want %d", info.Size(), wantSize)
	}

	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		allocBytes := sys.Blocks * 512
		if allocBytes > 1024*1024 {
			t.Errorf("expected sparse file (< 1MB on disk), got %d bytes allocated", allocBytes)
		}
	}
}

func TestEnsureNixVolume_Idempotent(t *testing.T) {
	home := t.TempDir()

	path1, err := EnsureNixVolume(home)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	info1, _ := os.Stat(path1)

	path2, err := EnsureNixVolume(home)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	info2, _ := os.Stat(path2)

	if path1 != path2 {
		t.Errorf("paths differ: %q vs %q", path1, path2)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("second call modified existing file — should be no-op")
	}
}

func TestGenerateNixVolumeMountScript_ContainsKeyElements(t *testing.T) {
	script := GenerateNixVolumeMountScript("main")

	for _, want := range []string{
		".devcell.json",
		"DevcellNix",
		"eraseDisk JHFS+",
		"synthetic.conf",
		"/nix",
		`"main"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script should contain %q", want)
		}
	}
}

func TestNixVolumeSizeGB(t *testing.T) {
	if NixVolumeSizeGB != 100 {
		t.Errorf("NixVolumeSizeGB = %d, want 100", NixVolumeSizeGB)
	}
}

func TestIsNixStore_ValidStore(t *testing.T) {
	root := t.TempDir()
	nixPath := filepath.Join(root, "nix")
	os.MkdirAll(filepath.Join(nixPath, "store"), 0o755)
	os.MkdirAll(filepath.Join(nixPath, "var", "nix", "db"), 0o755)
	os.WriteFile(filepath.Join(nixPath, "var", "nix", "db", "db.sqlite"), []byte("x"), 0o644)

	if !IsNixStore(nixPath) {
		t.Error("expected IsNixStore to return true for a valid store layout")
	}
}

func TestIsNixStore_MissingDB(t *testing.T) {
	root := t.TempDir()
	nixPath := filepath.Join(root, "nix")
	os.MkdirAll(filepath.Join(nixPath, "store"), 0o755)

	if IsNixStore(nixPath) {
		t.Error("expected IsNixStore to return false when db.sqlite is missing")
	}
}

func TestIsNixStore_MissingStore(t *testing.T) {
	root := t.TempDir()
	nixPath := filepath.Join(root, "nix")
	os.MkdirAll(nixPath, 0o755)

	if IsNixStore(nixPath) {
		t.Error("expected IsNixStore to return false when store dir is missing")
	}
}

func TestEnsureNixVolume_ResizesUndersized(t *testing.T) {
	home := t.TempDir()

	// Create a 30GB sparse image (old default).
	imgPath := NixVolumePath(home)
	os.MkdirAll(filepath.Dir(imgPath), 0o755)
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	oldSize := int64(30) * 1024 * 1024 * 1024
	f.Truncate(oldSize)
	f.Close()

	info, _ := os.Stat(imgPath)
	if info.Size() != oldSize {
		t.Fatalf("setup: expected %d, got %d", oldSize, info.Size())
	}

	// EnsureNixVolume should grow it to NixVolumeSizeGB.
	path, err := EnsureNixVolume(home)
	if err != nil {
		t.Fatalf("EnsureNixVolume failed: %v", err)
	}

	info, _ = os.Stat(path)
	wantSize := int64(NixVolumeSizeGB) * 1024 * 1024 * 1024
	if info.Size() != wantSize {
		t.Errorf("after resize: size = %d GB, want %d GB", info.Size()/(1024*1024*1024), NixVolumeSizeGB)
	}

	// Verify it's still sparse.
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		allocBytes := sys.Blocks * 512
		if allocBytes > 1024*1024 {
			t.Errorf("expected sparse after resize (< 1MB on disk), got %d bytes", allocBytes)
		}
	}
}

func TestNixVolumeMetadataFormat(t *testing.T) {
	meta := map[string]any{
		"type":    "nix-store",
		"cell":    "main",
		"created": "2026-07-18T00:00:00Z",
		"sizeGB":  NixVolumeSizeGB,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["type"] != "nix-store" {
		t.Errorf("type = %v, want nix-store", parsed["type"])
	}
}

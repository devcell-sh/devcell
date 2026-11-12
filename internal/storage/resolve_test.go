package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_SharedGoModUnconditional(t *testing.T) {
	p := Resolve(Input{
		CacheDir:    "/cache",
		SharedCache: true,
		StackTag:    "go-abcd1234",
	})
	if !strings.Contains(p.GoMod.HostPath, "/v1/shared/go/mod") {
		t.Errorf("shared GoMod should be under /v1/shared/go/mod, got %q", p.GoMod.HostPath)
	}
}

func TestResolve_SharedEnvScoped(t *testing.T) {
	p := Resolve(Input{
		CacheDir:    "/cache",
		SharedCache: true,
		StackTag:    "go-abcd1234",
	})
	if !strings.Contains(p.GoBuild.HostPath, "/v1/shared/env/go-abcd1234/go-build") {
		t.Errorf("shared GoBuild should be scoped by stackTag, got %q", p.GoBuild.HostPath)
	}
}

func TestResolve_PrivateScoped(t *testing.T) {
	p := Resolve(Input{
		CacheDir:      "/cache",
		SharedCache:   false,
		WorkspaceName: "main",
		CellName:      "proj-0",
		BaseDir:       "/x",
		StackTag:      "go",
	})
	if !strings.Contains(p.GoMod.HostPath, "/v1/private/main/proj-0-") {
		t.Errorf("private GoMod should be under /v1/private/main/proj-0-*, got %q", p.GoMod.HostPath)
	}
}

func TestResolve_DifferentStackTagsDiverge(t *testing.T) {
	base := Input{CacheDir: "/cache", SharedCache: true}
	base.StackTag = "go-aaaa"
	p1 := Resolve(base)
	base.StackTag = "node-bbbb"
	p2 := Resolve(base)
	if p1.GoBuild.HostPath == p2.GoBuild.HostPath {
		t.Errorf("different stackTags should produce different GoBuild paths")
	}
}

func TestResolve_SameStackTagsConverge(t *testing.T) {
	in1 := Input{
		CacheDir:      "/cache",
		SharedCache:   true,
		WorkspaceName: "ws1",
		CellName:      "a-0",
		BaseDir:       "/a",
		StackTag:      "go-same",
	}
	in2 := Input{
		CacheDir:      "/cache",
		SharedCache:   true,
		WorkspaceName: "ws2",
		CellName:      "b-0",
		BaseDir:       "/b",
		StackTag:      "go-same",
	}
	p1 := Resolve(in1)
	p2 := Resolve(in2)
	if p1.GoBuild.HostPath != p2.GoBuild.HostPath {
		t.Errorf("same stackTag should produce same GoBuild path, got %q vs %q", p1.GoBuild.HostPath, p2.GoBuild.HostPath)
	}
}

func TestResolve_GuestPathsStable(t *testing.T) {
	p := Resolve(Input{CacheDir: "/cache", SharedCache: true, StackTag: "x"})
	if p.GoMod.GuestPath != GuestGoMod {
		t.Errorf("GoMod guest path: want %s, got %s", GuestGoMod, p.GoMod.GuestPath)
	}
	if p.GoBuild.GuestPath != GuestGoBuild {
		t.Errorf("GoBuild guest path: want %s, got %s", GuestGoBuild, p.GoBuild.GuestPath)
	}
	if p.Npm.GuestPath != GuestNpm {
		t.Errorf("Npm guest path: want %s, got %s", GuestNpm, p.Npm.GuestPath)
	}
	if p.Uv.GuestPath != GuestUv {
		t.Errorf("Uv guest path: want %s, got %s", GuestUv, p.Uv.GuestPath)
	}
	if p.Winkit.GuestPath != GuestWinkit {
		t.Errorf("Winkit guest path: want %s, got %s", GuestWinkit, p.Winkit.GuestPath)
	}
}

func TestResolve_MountsReturnsFive(t *testing.T) {
	p := Resolve(Input{CacheDir: "/cache", SharedCache: true, StackTag: "x"})
	if got := len(p.Mounts()); got != 5 {
		t.Errorf("Mounts() should return 5, got %d", got)
	}
}

func TestResolve_WinkitAlwaysShared(t *testing.T) {
	p := Resolve(Input{
		CacheDir:      "/cache",
		SharedCache:   false,
		WorkspaceName: "main",
		CellName:      "proj-0",
		BaseDir:       "/x",
		StackTag:      "go",
	})
	if !strings.Contains(p.Winkit.HostPath, "/v1/shared/winkit") {
		t.Errorf("winkit should always be shared, got %q", p.Winkit.HostPath)
	}
}

func TestResolve_CustomCacheDir(t *testing.T) {
	p := Resolve(Input{
		CacheDir:    "/custom/path",
		SharedCache: true,
		StackTag:    "go",
	})
	for _, m := range p.Mounts() {
		if !strings.HasPrefix(m.HostPath, "/custom/path") {
			t.Errorf("all host paths should be under /custom/path, got %q", m.HostPath)
		}
	}
}

func TestCellKey_DifferentBaseDirsDiverge(t *testing.T) {
	k1 := CellKey("main", "proj-0", "/a")
	k2 := CellKey("main", "proj-0", "/b")
	if k1 == k2 {
		t.Errorf("different baseDirs should produce different cell keys")
	}
}

func TestCellKey_SameInputsStable(t *testing.T) {
	k1 := CellKey("main", "proj-0", "/a")
	k2 := CellKey("main", "proj-0", "/a")
	if k1 != k2 {
		t.Errorf("same inputs should produce same cell key: %q vs %q", k1, k2)
	}
}

func TestEnsureHostDirs_CreatesAll(t *testing.T) {
	tmp := t.TempDir()
	p := Resolve(Input{
		CacheDir:    tmp,
		SharedCache: true,
		StackTag:    "go-test",
	})
	if err := EnsureHostDirs(p); err != nil {
		t.Fatalf("EnsureHostDirs: %v", err)
	}
	for _, m := range p.Mounts() {
		fi, err := os.Stat(m.HostPath)
		if err != nil {
			t.Errorf("dir %s should exist: %v", m.HostPath, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s should be a directory", m.HostPath)
		}
		if perm := fi.Mode().Perm(); perm != 0o755 {
			t.Errorf("%s: want 0755, got %04o", m.HostPath, perm)
		}
	}
}

func TestEnsureHostDirs_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	p := Resolve(Input{CacheDir: tmp, SharedCache: true, StackTag: "x"})
	if err := EnsureHostDirs(p); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := EnsureHostDirs(p); err != nil {
		t.Fatalf("second call should not error: %v", err)
	}
}

func TestFindOldWinkitCaches_FindsPopulated(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, "cell-a", ".devcell", "cache", "qemu")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "windows.iso"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := FindOldWinkitCaches(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0] != old {
		t.Errorf("expected [%s], got %v", old, found)
	}
}

func TestFindOldWinkitCaches_SkipsEmpty(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, "cell-a", ".devcell", "cache", "qemu")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := FindOldWinkitCaches(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("empty old dir should be skipped, got %v", found)
	}
}

func TestMigrateWinkitCache_MovesFiles(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, "old")
	newDir := filepath.Join(tmp, "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "a.iso"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateWinkitCache(oldDir, newDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "a.iso")); err != nil {
		t.Errorf("file should appear in new dir: %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("old dir should be removed, got err=%v", err)
	}
}

func TestMigrateWinkitCache_SkipsExisting(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, "old")
	newDir := filepath.Join(tmp, "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "a.iso"), []byte("old-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "a.iso"), []byte("existing-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateWinkitCache(oldDir, newDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(newDir, "a.iso"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing-data" {
		t.Errorf("existing file should be kept, got %q", data)
	}
}

// ── Mackit cache migration ──────────────────────────────────────────────

func TestMackitPath(t *testing.T) {
	got := MackitPath("/test/cache")
	want := "/test/cache/v1/shared/mackit"
	if got != want {
		t.Errorf("MackitPath: want %q, got %q", want, got)
	}
}

func TestFindOldMackitCache_Found(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, ".cache", "mackit")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "BaseSystem.dmg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, ok := FindOldMackitCache(home)
	if !ok {
		t.Fatal("expected to find old mackit cache")
	}
	if dir != old {
		t.Errorf("want %q, got %q", old, dir)
	}
}

func TestFindOldMackitCache_NotFound(t *testing.T) {
	home := t.TempDir()
	_, ok := FindOldMackitCache(home)
	if ok {
		t.Error("should not find mackit cache in empty home")
	}
}

func TestFindOldMackitCache_SkipsEmpty(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, ".cache", "mackit")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	_, ok := FindOldMackitCache(home)
	if ok {
		t.Error("empty mackit dir should be skipped")
	}
}

func TestMigrateMackitCache_MovesFiles(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, "old")
	newDir := filepath.Join(tmp, "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "BaseSystem.dmg"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateMackitCache(oldDir, newDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "BaseSystem.dmg")); err != nil {
		t.Errorf("file should appear in new dir: %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("old dir should be removed, got err=%v", err)
	}
}

func TestMigrateMackitCache_SkipsExisting(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, "old")
	newDir := filepath.Join(tmp, "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "BaseSystem.dmg"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "BaseSystem.dmg"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateMackitCache(oldDir, newDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(newDir, "BaseSystem.dmg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Errorf("existing file should be kept, got %q", data)
	}
}

// ── Old workspace cache detection + cleanup ─────────────────────────────

func TestFindOldWorkspaceCaches_FindsPopulated(t *testing.T) {
	wsHome := t.TempDir()
	dirs := []string{"go/pkg/mod", ".cache/go-build", ".npm"}
	for _, d := range dirs {
		p := filepath.Join(wsHome, d)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "dummy"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found := FindOldWorkspaceCaches(wsHome)
	if len(found) != 3 {
		t.Errorf("expected 3 old caches, got %d: %v", len(found), found)
	}
}

func TestFindOldWorkspaceCaches_SkipsEmpty(t *testing.T) {
	wsHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wsHome, "go/pkg/mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	found := FindOldWorkspaceCaches(wsHome)
	if len(found) != 0 {
		t.Errorf("empty dirs should be skipped, got %v", found)
	}
}

func TestFindOldWorkspaceCaches_SkipsNonexistent(t *testing.T) {
	wsHome := t.TempDir()
	found := FindOldWorkspaceCaches(wsHome)
	if len(found) != 0 {
		t.Errorf("nonexistent dirs should be skipped, got %v", found)
	}
}

func TestCleanOldWorkspaceCaches_RemovesDirs(t *testing.T) {
	wsHome := t.TempDir()
	dirs := []string{
		filepath.Join(wsHome, "go/pkg/mod"),
		filepath.Join(wsHome, ".cache/go-build"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "f"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CleanOldWorkspaceCaches(dirs); err != nil {
		t.Fatalf("clean: %v", err)
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("dir %s should be removed", d)
		}
	}
}

func TestCleanOldWorkspaceCaches_IdempotentOnMissing(t *testing.T) {
	if err := CleanOldWorkspaceCaches([]string{"/nonexistent/path"}); err != nil {
		t.Errorf("cleaning nonexistent path should not error: %v", err)
	}
}

// ── Unified cache migration (XDG move) ─────────────────────────────────

func TestMigrateUnifiedCache_MovesV1(t *testing.T) {
	tmp := t.TempDir()
	oldRoot := filepath.Join(tmp, "old")
	newRoot := filepath.Join(tmp, "new")

	// Set up old v1 tree
	if err := os.MkdirAll(filepath.Join(oldRoot, "v1", "shared", "go", "mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "v1", "shared", "go", "mod", "pkg.zip"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigrateUnifiedCache(oldRoot, newRoot)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration to run")
	}

	// File should be at new location
	if _, err := os.Stat(filepath.Join(newRoot, "v1", "shared", "go", "mod", "pkg.zip")); err != nil {
		t.Errorf("file should exist at new location: %v", err)
	}
	// Old root should be gone
	if _, err := os.Stat(oldRoot); !os.IsNotExist(err) {
		t.Errorf("old root should be removed")
	}
}

func TestMigrateUnifiedCache_MergesWithExisting(t *testing.T) {
	tmp := t.TempDir()
	oldRoot := filepath.Join(tmp, "old")
	newRoot := filepath.Join(tmp, "new")

	// Old has file A
	if err := os.MkdirAll(filepath.Join(oldRoot, "v1", "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "v1", "shared", "a.txt"), []byte("old-a"), 0o644); err != nil {
		t.Fatal(err)
	}

	// New already has file A (different content) and file B
	if err := os.MkdirAll(filepath.Join(newRoot, "v1", "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newRoot, "v1", "shared", "a.txt"), []byte("new-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newRoot, "v1", "shared", "b.txt"), []byte("new-b"), 0o644); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigrateUnifiedCache(oldRoot, newRoot)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration to run")
	}

	// Existing file should be kept
	data, _ := os.ReadFile(filepath.Join(newRoot, "v1", "shared", "a.txt"))
	if string(data) != "new-a" {
		t.Errorf("existing file should be preserved, got %q", data)
	}
	// File B should still exist
	if _, err := os.Stat(filepath.Join(newRoot, "v1", "shared", "b.txt")); err != nil {
		t.Errorf("b.txt should still exist: %v", err)
	}
}

func TestMigrateUnifiedCache_NoV1(t *testing.T) {
	tmp := t.TempDir()
	oldRoot := filepath.Join(tmp, "old")
	if err := os.MkdirAll(oldRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigrateUnifiedCache(oldRoot, filepath.Join(tmp, "new"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated {
		t.Error("should not migrate when no v1/ exists")
	}
}

func TestMigrateUnifiedCache_Nonexistent(t *testing.T) {
	migrated, err := MigrateUnifiedCache("/nonexistent", "/also-nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated {
		t.Error("should not migrate nonexistent dir")
	}
}

func TestForceRemoveAll_ReadOnlyDirs(t *testing.T) {
	tmp := t.TempDir()
	// Simulate Go module cache: read-only directories
	modDir := filepath.Join(tmp, "go", "pkg", "mod", "example@v1.0.0")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module example"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make dir read-only (same as go mod download does)
	os.Chmod(modDir, 0o555)

	// Plain os.RemoveAll would fail
	target := filepath.Join(tmp, "go")
	if err := ForceRemoveAll(target); err != nil {
		t.Fatalf("ForceRemoveAll: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("directory should be removed")
	}
}

func TestForceRemoveAll_Nonexistent(t *testing.T) {
	if err := ForceRemoveAll("/nonexistent/path"); err != nil {
		t.Fatalf("unexpected error for nonexistent path: %v", err)
	}
}

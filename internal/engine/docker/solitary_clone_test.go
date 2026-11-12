package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/config"
)

func TestSolitaryDir_Format(t *testing.T) {
	prefix := solitaryRunPrefix("847")
	dir := solitaryDir("/proj", prefix, "project")
	if !strings.HasPrefix(dir, "/proj/.devcell/solitary/") {
		t.Fatalf("unexpected prefix: %s", dir)
	}
	if !strings.HasSuffix(dir, "-847-project") {
		t.Fatalf("unexpected suffix: %s", dir)
	}

	home := solitaryDir("/proj", prefix, "home")
	if !strings.HasSuffix(home, "-847-home") {
		t.Fatalf("home unexpected suffix: %s", home)
	}
}

func TestCreateSolitaryDir_CreatesEmptyDir(t *testing.T) {
	base := t.TempDir()
	prefix := solitaryRunPrefix("99")
	dir, err := createSolitaryDir(base, prefix, "project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, "-project") {
		t.Fatalf("should end with -project, got %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty dir, got %d entries", len(entries))
	}
}

func TestCloneProject_CopiesFiles(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "main.go"), []byte("package main"), 0o644)
	os.MkdirAll(filepath.Join(src, "internal"), 0o755)
	os.WriteFile(filepath.Join(src, "internal", "lib.go"), []byte("package internal"), 0o644)

	c := config.Config{BaseDir: src}
	prefix := solitaryRunPrefix("test1")
	clone, err := cloneProject(c, prefix)
	if err != nil {
		t.Fatal(err)
	}

	if clone.BaseDir != src {
		t.Fatalf("BaseDir should stay original %s, got %s", src, clone.BaseDir)
	}
	if clone.HostBaseDir == "" || clone.HostBaseDir == src {
		t.Fatal("HostBaseDir should be the clone path, not empty or original")
	}
	if !strings.HasSuffix(clone.HostBaseDir, "-project") {
		t.Fatalf("HostBaseDir should end with -project, got %s", clone.HostBaseDir)
	}

	data, err := os.ReadFile(filepath.Join(clone.HostBaseDir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package main" {
		t.Errorf("main.go content mismatch: %q", data)
	}

	data, err = os.ReadFile(filepath.Join(clone.HostBaseDir, "internal", "lib.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package internal" {
		t.Errorf("internal/lib.go content mismatch: %q", data)
	}
}

func TestCloneProject_SkipsDevcellDir(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "main.go"), []byte("package main"), 0o644)

	os.MkdirAll(filepath.Join(src, ".devcell", "solitary", "20260101T000000Z-old-project"), 0o755)
	os.WriteFile(filepath.Join(src, ".devcell", "result"), []byte("nix"), 0o644)

	c := config.Config{BaseDir: src}
	clone, err := cloneProject(c, solitaryRunPrefix("test2"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(clone.HostBaseDir, ".devcell")); err == nil {
		t.Errorf(".devcell/ should not be copied into the clone")
	}

	if _, err := os.Stat(filepath.Join(clone.HostBaseDir, "main.go")); err != nil {
		t.Errorf("main.go should be in the clone")
	}
}

func TestCloneProject_PreservesFileMode(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "script.sh"), []byte("#!/bin/sh"), 0o755)

	c := config.Config{BaseDir: src}
	clone, err := cloneProject(c, solitaryRunPrefix("test3"))
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(clone.HostBaseDir, "script.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("script.sh should preserve executable bit, got %o", info.Mode())
	}
}

func TestCloneProject_PreservesSymlinks(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "real.txt"), []byte("hello"), 0o644)
	os.Symlink("real.txt", filepath.Join(src, "link.txt"))
	os.Symlink("/nix/store/nonexistent", filepath.Join(src, "dangling"))

	c := config.Config{BaseDir: src}
	clone, err := cloneProject(c, solitaryRunPrefix("test4"))
	if err != nil {
		t.Fatal(err)
	}

	target, err := os.Readlink(filepath.Join(clone.HostBaseDir, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "real.txt" {
		t.Errorf("link.txt target: want real.txt, got %q", target)
	}

	target, err = os.Readlink(filepath.Join(clone.HostBaseDir, "dangling"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "/nix/store/nonexistent" {
		t.Errorf("dangling target: want /nix/store/nonexistent, got %q", target)
	}
}

func TestCloneDir_CopiesArbitraryDir(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "workspace-home")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, ".bashrc"), []byte("alias l=ls"), 0o644)

	prefix := solitaryRunPrefix("test5")
	dst, err := cloneDir(src, base, prefix, "home")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dst, "-home") {
		t.Fatalf("should end with -home, got %s", dst)
	}
	data, err := os.ReadFile(filepath.Join(dst, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alias l=ls" {
		t.Errorf(".bashrc content mismatch: %q", data)
	}
}

func TestSolitaryRunPrefix_SharedForHomeAndProject(t *testing.T) {
	prefix := solitaryRunPrefix("42")
	if !strings.HasSuffix(prefix, "-42") {
		t.Fatalf("prefix should end with -42, got %q", prefix)
	}

	base := t.TempDir()
	homeDir, err := createSolitaryDir(base, prefix, "home")
	if err != nil {
		t.Fatal(err)
	}
	projDir := solitaryDir(base, prefix, "project")

	homeBase := filepath.Base(homeDir)
	projBase := filepath.Base(projDir)
	homeParts := strings.TrimSuffix(homeBase, "-home")
	projParts := strings.TrimSuffix(projBase, "-project")
	if homeParts != projParts {
		t.Errorf("home and project should share prefix: home=%q project=%q", homeParts, projParts)
	}
}

//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DimmKirr/devcell/internal/vm/qemu"
)

// Keys lived at ~/.devcell/<cell>/qemu/, but they are not qemu's: libvirt reuses
// the same pair, and .ssh is where anyone looks first. The engine name in the
// path was an accident of which engine happened to need keys first.
func TestQemuKeyDir_PrefersDotSSH(t *testing.T) {
	home := t.TempDir()

	dir := qemuKeyDir(home, "DIMM")

	if want := filepath.Join(home, ".devcell", "DIMM", ".ssh"); dir != want {
		t.Errorf("new cells must use %s, got %s", want, dir)
	}
}

// A template already built has the old key baked into the guest. Moving the
// path must not orphan it — that would silently turn a 3-hour template into
// one nothing can log into.
func TestQemuKeyDir_KeepsUsingTheLegacyPathWhenAKeyIsAlreadyThere(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".devcell", "DIMM", "qemu")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "id_ed25519"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	if dir := qemuKeyDir(home, "DIMM"); dir != legacy {
		t.Errorf("an existing key must keep its path, got %s", dir)
	}
}

// Every qemu template path passes modules=nil, so two cells on the same stack
// with different module sets resolve to one disk-base.qcow2 and one
// .provisioned marker. The first build wins; the second either reuses a
// template missing its modules or, with --force, destroys the first. StackTag
// exists precisely to keep them apart — tart passes modules, qemu does not.
func TestQemuTemplatePaths_SeparateTemplatesPerModuleSet(t *testing.T) {
	home := t.TempDir()

	bare := qemu.TemplateDir(home, "base", nil)
	withMods := qemu.TemplateDir(home, "base", []string{"docker", "node"})

	if bare == withMods {
		t.Fatalf("module sets must not share a template dir: both resolved to %s", bare)
	}
	if qemu.ImageName("base", nil) == qemu.ImageName("base", []string{"docker", "node"}) {
		t.Error("module sets must not share a disk image name")
	}
	if qemu.ProvisionedMarker(home, "base", nil) == qemu.ProvisionedMarker(home, "base", []string{"docker", "node"}) {
		t.Error("module sets must not share a provisioned marker")
	}
}

// Module order is not meaningful, so it must not fork the template.
func TestQemuTemplatePaths_ModuleOrderDoesNotMatter(t *testing.T) {
	home := t.TempDir()

	if a, b := qemu.TemplateDir(home, "base", []string{"node", "docker"}),
		qemu.TemplateDir(home, "base", []string{"docker", "node"}); a != b {
		t.Errorf("the same modules in a different order must be one template: %s vs %s", a, b)
	}
}

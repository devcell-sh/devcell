package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `cell init --upgrade` rewrites the config files to the current syntax
// (the same migration as `cell config migrate`) and stops there: no
// scaffold, no flake resolution, no engine init.
func TestInitUpgrade_MigratesConfigAndStops(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	if err := os.WriteFile(global, []byte(legacyGlobalTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCellIn(t, home, "init", "--upgrade")
	if err != nil {
		t.Fatalf("exit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "2 changes") || !strings.Contains(out, ".bak-") {
		t.Errorf("want change count and backup name:\n%s", out)
	}
	b, _ := os.ReadFile(global)
	if !strings.Contains(string(b), "winkit_ssh_port = 3333") || !strings.Contains(string(b), "volumes = [\"/data\"]") {
		t.Errorf("file not migrated:\n%s", b)
	}
	for _, unwanted := range []string{"flake", "Resolving", "Scaffold", "nixhome"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("--upgrade must not run the scaffold or flake steps, saw %q:\n%s", unwanted, out)
		}
	}
}

func TestInitUpgrade_FlagRegistered(t *testing.T) {
	out, err := runCellIn(t, t.TempDir(), "init", "--help")
	if err != nil {
		t.Fatalf("exit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "--upgrade") {
		t.Errorf("want --upgrade in init help:\n%s", out)
	}
}

// The project file comes first: it is the one the user is working in, and
// inside a container the global file is often a read-only bind mount.
func TestInitUpgrade_ProjectFileFirst(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	if err := os.WriteFile(global, []byte(legacyGlobalTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".devcell.toml"), []byte("[cell]\nthin = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCellIn(t, home, "init", "--upgrade")
	if err != nil {
		t.Fatalf("exit: %v\n%s", err, out)
	}
	p, g := strings.Index(out, "/.devcell.toml —"), strings.Index(out, "/devcell.toml —")
	if p < 0 || g < 0 || p > g {
		t.Errorf("want the project row before the global row:\n%s", out)
	}
}

// A global file that cannot be written (read-only bind mount inside a cell)
// is skipped with a hint, and does not fail the run or leave a backup.
func TestInitUpgrade_ReadOnlyGlobalIsSkipped(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	if err := os.WriteFile(global, []byte(legacyGlobalTOML), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(global, 0o444); err != nil { // WriteFile keeps an existing file's mode
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".devcell.toml"), []byte("[cell]\nthin = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCellIn(t, home, "init", "--upgrade")
	if err != nil {
		t.Fatalf("a read-only global file must not fail the run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "read-only") || !strings.Contains(out, "on the host") || !strings.Contains(out, global) {
		t.Errorf("want a read-only skip row with a host hint:\n%s", out)
	}
	if b, _ := os.ReadFile(global); string(b) != legacyGlobalTOML {
		t.Errorf("read-only file must be untouched:\n%s", b)
	}
	if matches, _ := filepath.Glob(global + ".bak-*"); len(matches) != 0 {
		t.Errorf("no backup expected for a skipped file, got %v", matches)
	}
	if b, _ := os.ReadFile(filepath.Join(home, ".devcell.toml")); strings.Contains(string(b), "thin") {
		t.Errorf("project file must still be migrated:\n%s", b)
	}
}

// A global file that is a home-manager symlink into /nix/store is read-only
// everywhere, so "run on the host" is the wrong hint. The row names the
// link target, lists the changes to make in the source, and the run still
// succeeds.
func TestInitUpgrade_NixStoreSymlinkGlobalNamesSource(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	target := filepath.Join(home, "nix", "store", "abc-home-manager-files", ".config", "devcell", "devcell.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(legacyGlobalTOML), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(global); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, global); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".devcell.toml"), []byte("[cell]\nthin = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCellIn(t, home, "init", "--upgrade")
	if err != nil {
		t.Fatalf("a read-only symlinked global file must not fail the run: %v\n%s", err, out)
	}
	if strings.Contains(out, "on the host") {
		t.Errorf("host hint is wrong for a symlink into a read-only target:\n%s", out)
	}
	if !strings.Contains(out, target) || !strings.Contains(out, "home-manager") {
		t.Errorf("want the row to name the link target and home-manager:\n%s", out)
	}
	if !strings.Contains(out, "[[volumes]]: moved to [cell] volumes") {
		t.Errorf("want the changes listed so they can be applied in the source:\n%s", out)
	}
	if matches, _ := filepath.Glob(global + ".bak-*"); len(matches) != 0 {
		t.Errorf("no backup expected for a skipped file, got %v", matches)
	}
}

// A file that cannot be rewritten in place still gets its migrated form
// written next to it as <name>.migrated, so there is a complete file to
// copy into whatever generates the read-only one.
func TestInitUpgrade_ReadOnlyGlobalWritesMigratedCopy(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	final := filepath.Join(home, "nix", "store", "7rn1-devcell.toml")
	hm := filepath.Join(home, "nix", "store", "abc-home-manager-files", ".config", "devcell", "devcell.toml")
	for _, d := range []string{filepath.Dir(final), filepath.Dir(hm)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(final, []byte(legacyGlobalTOML), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(final, hm); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(global); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hm, global); err != nil {
		t.Fatal(err)
	}
	out, err := runCellIn(t, home, "init", "--upgrade")
	if err != nil {
		t.Fatalf("exit: %v\n%s", err, out)
	}
	migrated := global + ".migrated"
	if !strings.Contains(out, migrated) || !strings.Contains(out, "home-manager switch") {
		t.Errorf("want the .migrated path and the home-manager hint in the row:\n%s", out)
	}
	b, err := os.ReadFile(migrated)
	if err != nil {
		t.Fatalf("migrated copy: %v\n%s", err, out)
	}
	if !strings.Contains(string(b), "winkit_ssh_port = 3333") || !strings.Contains(string(b), "volumes = [\"/data\"]") {
		t.Errorf("migrated copy must hold the rewritten config:\n%s", b)
	}
	if b, _ := os.ReadFile(final); string(b) != legacyGlobalTOML {
		t.Errorf("store file must be untouched:\n%s", b)
	}
}

// Nothing to migrate means no .migrated file either, and a stale one from
// an earlier run is removed so it cannot be mistaken for current.
func TestInitUpgrade_UpToDateReadOnlyLeavesNoMigratedCopy(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	if err := os.WriteFile(global, []byte("[cell]\nstack = \"base\"\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(global, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global+".migrated", []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCellIn(t, home, "init", "--upgrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(global + ".migrated"); !os.IsNotExist(err) {
		t.Errorf("stale .migrated copy must be removed when the file is up to date")
	}
}

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const legacyGlobalTOML = "[cell]\nqemu_ssh_port = 3333\n\n[[volumes]]\nmount = \"/data\"\n"

func runCellIn(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestConfigMigrate_DryRunShowsChangesWithoutWriting(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	if err := os.WriteFile(global, []byte(legacyGlobalTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCellIn(t, home, "config", "migrate", "--dry-run")
	if err != nil {
		t.Fatalf("exit: %v\n%s", err, out)
	}
	for _, want := range []string{"devcell.toml", "winkit_ssh_port", "[cell] volumes"} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in output:\n%s", want, out)
		}
	}
	if b, _ := os.ReadFile(global); string(b) != legacyGlobalTOML {
		t.Errorf("dry run must not write:\n%s", b)
	}
	if strings.Contains(out, "is deprecated") {
		t.Errorf("config migrate must not print the startup deprecation rows first:\n%s", out)
	}
}

func TestConfigMigrate_RewritesAndSilencesWarnings(t *testing.T) {
	home := scaffoldedHome(t)
	global := filepath.Join(home, ".config", "devcell", "devcell.toml")
	if err := os.WriteFile(global, []byte(legacyGlobalTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCellIn(t, home, "config", "migrate")
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
	if matches, _ := filepath.Glob(global + ".bak-*"); len(matches) != 1 {
		t.Errorf("want one backup, got %v", matches)
	}
	out, err = runCellIn(t, home, "shell", "--dry-run")
	if err != nil {
		t.Fatalf("shell after migrate: %v\n%s", err, out)
	}
	if strings.Contains(out, "is deprecated") {
		t.Errorf("no deprecation rows expected after migrate:\n%s", out)
	}
	out, _ = runCellIn(t, home, "config", "migrate")
	if !strings.Contains(out, "up to date") {
		t.Errorf("second run should report up to date:\n%s", out)
	}
}

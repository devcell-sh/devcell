package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runCellWithGlobalConfig(t *testing.T, toml string, args ...string) (string, error) {
	t.Helper()
	home := scaffoldedHome(t)
	if err := os.WriteFile(filepath.Join(home, ".config", "devcell", "devcell.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "DEVCELL_BUNK=1", "HOME="+home)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// An invalid config stops cell instead of being ignored with a warning.
func TestInvalidConfig_StopsCell(t *testing.T) {
	out, err := runCellWithGlobalConfig(t, "[cell]\nstak = \"go\"\n", "shell", "--dry-run")
	if err == nil {
		t.Fatalf("want a non-zero exit, got success:\n%s", out)
	}
	if !strings.Contains(out, "cell.stak") {
		t.Errorf("output should name the unknown key:\n%s", out)
	}
	if strings.Contains(out, "config ignored") {
		t.Errorf("config must not be silently ignored:\n%s", out)
	}
}

// --version and help still run, so a broken config can be diagnosed.
func TestInvalidConfig_VersionStillRuns(t *testing.T) {
	if out, err := runCellWithGlobalConfig(t, "[cell]\nstak = \"go\"\n", "--version"); err != nil {
		t.Fatalf("cell --version failed: %v\n%s", err, out)
	}
}

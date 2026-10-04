package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newLocalNixhomeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for path, body := range map[string]string{
		"stacks/base.nix":  "{ imports = [ ../modules/base.nix ]; }",
		"modules/base.nix": "{}",
	} {
		full := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func TestResolveNixhome_DefaultSourceClonesRepo(t *testing.T) {
	repo := newLocalNixhomeRepo(t)
	orig := nixhomeRepo
	nixhomeRepo = "file://" + repo
	t.Cleanup(func() { nixhomeRepo = orig })

	buildDir := t.TempDir()
	if err := ResolveNixhome("", buildDir, "v0.0.0", false); err != nil {
		t.Fatalf("ResolveNixhome: %v", err)
	}

	dest := filepath.Join(buildDir, "nixhome")
	for _, f := range []string{"stacks/base.nix", "modules/base.nix"} {
		if _, err := os.Stat(filepath.Join(dest, f)); err != nil {
			t.Errorf("expected %s in cloned nixhome: %v", f, err)
		}
	}
	src, err := os.ReadFile(filepath.Join(dest, NixhomeSourceFile))
	if err != nil {
		t.Fatalf("read %s: %v", NixhomeSourceFile, err)
	}
	if got := strings.TrimSpace(string(src)); got != nixhomeRepo {
		t.Errorf("source file: got %q, want %q", got, nixhomeRepo)
	}
}

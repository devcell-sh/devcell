package docker_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/docker"
)

// CELL-391: stale cell warning at start. Drift policy (2026-08-01):
// inform, not enforce — a nudge with `cell build --update`, never a gate.

func TestProjectNixpkgsRev_ReadsScaffoldedLock(t *testing.T) {
	dir := t.TempDir()
	lockDir := filepath.Join(dir, ".devcell")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := `{"nodes":{"nixpkgs":{"locked":{"rev":"4a1b2c3deadbeef"}}}}`
	if err := os.WriteFile(filepath.Join(lockDir, "flake.lock"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	rev, err := docker.ProjectNixpkgsRev(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rev != "4a1b2c3deadbeef" {
		t.Errorf("want 4a1b2c3deadbeef, got %q", rev)
	}
}

func TestProjectNixpkgsRev_MissingLockIsEmptyNotError(t *testing.T) {
	rev, err := docker.ProjectNixpkgsRev(t.TempDir())
	if err != nil {
		t.Fatalf("missing lock must degrade silently, got %v", err)
	}
	if rev != "" {
		t.Errorf("want empty rev, got %q", rev)
	}
}

func TestStaleCellWarning_BehindNewestWarns(t *testing.T) {
	h := docker.NixStoreHealth{DistinctRevs: 2, NewestRev: "9f8e7d6abcdef", NewestProjects: 3}
	msg, stale := docker.StaleCellWarning("4a1b2c3deadbeef", h)
	if !stale {
		t.Fatal("project behind newest volume rev must warn")
	}
	for _, want := range []string{
		"4a1b2c3", "9f8e7d6", "3 project", "parallel reality",
		"cell build --update", "Continue anyway? [Y/n]",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("warning missing %q:\n%s", want, msg)
		}
	}
}

func TestStaleCellWarning_OnNewestIsSilent(t *testing.T) {
	h := docker.NixStoreHealth{DistinctRevs: 1, NewestRev: "9f8e7d6abcdef"}
	if _, stale := docker.StaleCellWarning("9f8e7d6abcdef", h); stale {
		t.Error("project on the newest rev must not warn")
	}
}

// Detection failure degrades to silence — never a prompt, never fatal.
func TestStaleCellWarning_UnknownRevsAreSilent(t *testing.T) {
	if _, stale := docker.StaleCellWarning("", docker.NixStoreHealth{NewestRev: "abc"}); stale {
		t.Error("unknown project rev must not warn")
	}
	if _, stale := docker.StaleCellWarning("abc", docker.NixStoreHealth{}); stale {
		t.Error("no volume rev data must not warn")
	}
}

// ConfirmProceed is the default-YES twin of ConfirmDestructive: a nudge,
// not a gate. Enter / y / anything-but-n proceeds; only n/no aborts.
// Non-TTY always proceeds after printing the warning — never blocks CI.
func TestConfirmProceed_EnterProceeds(t *testing.T) {
	var out strings.Builder
	if !docker.ConfirmProceed(&out, strings.NewReader("\n"), true, "WARN") {
		t.Error("bare Enter must proceed (default yes)")
	}
}

func TestConfirmProceed_NoAborts(t *testing.T) {
	var out strings.Builder
	if docker.ConfirmProceed(&out, strings.NewReader("n\n"), true, "WARN") {
		t.Error("answering n must abort")
	}
}

func TestConfirmProceed_NonTTYProceedsWithWarning(t *testing.T) {
	var out strings.Builder
	if !docker.ConfirmProceed(&out, strings.NewReader(""), false, "WARN") {
		t.Error("non-TTY must proceed unconditionally")
	}
	if !strings.Contains(out.String(), "WARN") {
		t.Error("non-TTY must still print the warning")
	}
}

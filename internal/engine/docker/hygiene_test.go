package docker_test

import (
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

// The hygiene script removes only dangling symlinks and orphaned metadata.
// It must NEVER run nix garbage collection or touch profile generations.
func TestHygieneNixScript_ContainsNoGCOrOptimise(t *testing.T) {
	for _, forbidden := range []string{
		"nix-store --gc",
		"nix-collect-garbage",
		"nix-store --optimise",
		"profile-*-link",
	} {
		if strings.Contains(docker.HygieneNixScript, forbidden) {
			t.Errorf("hygiene script must not contain %q", forbidden)
		}
	}
}

// The hygiene script must not touch auto/ roots (container-private namespace,
// CELL-330). Only gcroots/devcell/ entries are managed.
func TestHygieneNixScript_DoesNotTouchAutoRoots(t *testing.T) {
	if strings.Contains(docker.HygieneNixScript, "gcroots/auto") {
		t.Error("hygiene script must not reference gcroots/auto")
	}
}

// The script must produce a machine-readable summary line with stale= and
// orphaned_meta= fields so the Go caller can parse and surface the result.
func TestHygieneNixScript_ProducesMachineReadableOutput(t *testing.T) {
	if !strings.Contains(docker.HygieneNixScript, `echo "stale=`) {
		t.Error("hygiene script must echo a stale= line")
	}
	if !strings.Contains(docker.HygieneNixScript, "orphaned_meta=") {
		t.Error("hygiene script must echo an orphaned_meta= field")
	}
}

// The script must remove companion files (-generation, -meta) when a stale
// root is detected, not just the -profile symlink itself.
func TestHygieneNixScript_RemovesCompanionFiles(t *testing.T) {
	for _, suffix := range []string{"-profile", "-generation", "-meta"} {
		pattern := `rm -f "/nix/var/nix/gcroots/devcell/${hash}` + suffix + `"`
		if !strings.Contains(docker.HygieneNixScript, pattern) {
			t.Errorf("hygiene script must remove %s companion: missing %q", suffix, pattern)
		}
	}
}

// The script must handle the case where /nix/var/nix/gcroots/devcell/ does
// not exist (fresh install). It wraps its loops in an existence check.
func TestHygieneNixScript_HandlesNonexistentDir(t *testing.T) {
	if !strings.Contains(docker.HygieneNixScript, "if [ -d /nix/var/nix/gcroots/devcell ]") {
		t.Error("hygiene script must guard on gcroots/devcell existence")
	}
}

func TestParseHygieneResult_ParsesCounts(t *testing.T) {
	stale, orphaned := docker.ParseHygieneResult("stale=3 orphaned_meta=1\n")
	if stale != 3 {
		t.Errorf("stale: got %d, want 3", stale)
	}
	if orphaned != 1 {
		t.Errorf("orphaned_meta: got %d, want 1", orphaned)
	}
}

func TestParseHygieneResult_ZeroCounts(t *testing.T) {
	stale, orphaned := docker.ParseHygieneResult("stale=0 orphaned_meta=0\n")
	if stale != 0 || orphaned != 0 {
		t.Errorf("got stale=%d orphaned=%d, want both 0", stale, orphaned)
	}
}

// Docker may prepend image-pull noise before the data line. The parser must
// skip it (same pattern as ParseNixStoreHealth).
func TestParseHygieneResult_SkipsPullNoise(t *testing.T) {
	output := "Unable to find image 'nixos/nix:2.34.9' locally\n" +
		"2.34.9: Pulling from nixos/nix\n" +
		"Digest: sha256:abc123\n" +
		"Status: Downloaded newer image for nixos/nix:2.34.9\n" +
		"stale=2 orphaned_meta=0\n"
	stale, orphaned := docker.ParseHygieneResult(output)
	if stale != 2 {
		t.Errorf("stale: got %d, want 2", stale)
	}
	if orphaned != 0 {
		t.Errorf("orphaned_meta: got %d, want 0", orphaned)
	}
}

func TestParseHygieneResult_NoDataLine(t *testing.T) {
	stale, orphaned := docker.ParseHygieneResult("some random output\n")
	if stale != 0 || orphaned != 0 {
		t.Errorf("no data line should yield zeros, got stale=%d orphaned=%d", stale, orphaned)
	}
}

func TestHygieneNixArgv_RunsInContainerOnNixVolume(t *testing.T) {
	argv := docker.HygieneNixArgv("test-volume")
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "docker run --rm") {
		t.Error("must use docker run --rm")
	}
	if !strings.Contains(joined, "test-volume:/nix") {
		t.Error("must mount the nix volume at /nix")
	}
	if !strings.Contains(joined, docker.NixCoreImage) {
		t.Errorf("must use NixCoreImage (%s)", docker.NixCoreImage)
	}
	if !strings.Contains(joined, "/bin/sh") {
		t.Error("must run via /bin/sh")
	}
}

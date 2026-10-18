package docker_test

import (
	"strings"
	"testing"

	docker "github.com/devcell-sh/devcell/internal/engine/docker"
)

func TestEnsureVolumeShellArgv_MountsVolumeAtMntNix(t *testing.T) {
	argv := docker.EnsureVolumeShellArgv("devcell-nix-store", "docker.io/nixos/nix:2.34.7")
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "devcell-nix-store:/mnt/nix") {
		t.Errorf("expected volume mount at /mnt/nix, got: %s", joined)
	}
	// Must NOT mount at /nix: that shadows the image's own /nix and recreates
	// the chicken-and-egg problem the bootstrap exists to solve.
	for _, a := range argv {
		if a == "devcell-nix-store:/nix" || a == "devcell-nix-store:/nix:rw" {
			t.Fatalf("volume must NOT mount at /nix: %s", joined)
		}
	}
}

func TestEnsureVolumeShellArgv_UsesImageBinSh(t *testing.T) {
	argv := docker.EnsureVolumeShellArgv("test-vol", "docker.io/nixos/nix:2.34.7")
	// The last three elements must be: /bin/sh -c <script>
	if len(argv) < 3 {
		t.Fatalf("argv too short: %v", argv)
	}
	shell := argv[len(argv)-3]
	flag := argv[len(argv)-2]
	if shell != "/bin/sh" || flag != "-c" {
		t.Errorf("expected entrypoint /bin/sh -c, got: %s %s", shell, flag)
	}
}

func TestEnsureVolumeShellArgv_UsesCorrectImage(t *testing.T) {
	const img = "docker.io/nixos/nix:2.99.0"
	argv := docker.EnsureVolumeShellArgv("vol", img)
	found := false
	for i, a := range argv {
		if a == img && i > 0 && argv[i-1] != "-v" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected image %s in argv: %v", img, argv)
	}
}

func TestEnsureVolumeShellArgv_ScriptReconcilesWithoutEarlyExit(t *testing.T) {
	argv := docker.EnsureVolumeShellArgv("vol", "img")
	script := argv[len(argv)-1]

	// No fast-path exit: the profile's store directory being present does
	// not prove the volume is usable (a hydrated cache can leave
	// /nix/var/nix/profiles/default dangling), so the script must always
	// reach the link repair below.
	if strings.Contains(script, "exit 0") {
		t.Error("script must not exit early when the closure is already present")
	}
	// Store paths already on the volume are skipped, not re-copied.
	if !strings.Contains(script, `[ ! -d "/mnt/nix/store/$BASE" ]`) {
		t.Error("script must copy only store paths missing from the volume")
	}
	// The profile link is repointed unconditionally.
	if !strings.Contains(script, `ln -sfT "$PROFILE_TARGET" /mnt/nix/var/nix/profiles/default`) {
		t.Error("script must repoint /mnt/nix/var/nix/profiles/default on every run")
	}
	// The script must use nix-store to resolve the full closure.
	if !strings.Contains(script, "nix-store") {
		t.Error("script must use nix-store to resolve the closure")
	}
	// The script must write to /mnt/nix/store, not /nix/store.
	if !strings.Contains(script, "/mnt/nix/store") {
		t.Error("script must copy to /mnt/nix/store")
	}
}

func TestEnsureVolumeShellArgv_ContainerIsRm(t *testing.T) {
	argv := docker.EnsureVolumeShellArgv("vol", "img")
	found := false
	for _, a := range argv {
		if a == "--rm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("bootstrap container must be --rm")
	}
}

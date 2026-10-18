package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// requireDocker skips the test when no daemon answers. The package's unit
// tests otherwise never touch docker; this one has to, because the state it
// reproduces only exists inside a named volume.
func requireDocker(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not reachable")
	}
}

func dockerRun(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", append([]string{"run", "--rm"}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// A nix-store volume can end up with the core image's store paths present
// but /nix/var/nix/profiles/default pointing at a path that is gone:
// `cell build prune` garbage-collects the builder's own profile closure
// (the image ships no gcroot for it), and a nix cache published from such a
// volume carries the dangling symlink. Hydrating that cache over a freshly
// populated volume overwrites the good symlink while bash stays in the
// store, so /bin/sh still starts but PATH resolves nothing: the thin build
// dies on its first `mkdir`. The bootstrap must repair the profile link
// even when the profile's store directory is already on the volume.
func TestEnsureVolumeShell_RepairsDanglingDefaultProfile(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	vol := fmt.Sprintf("devcell-test-volshell-%d", os.Getpid())
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", "-f", vol).Run() })

	probe := func() (string, error) {
		return dockerRun(ctx, "-v", vol+":/nix", NixCoreImage, "/bin/sh", "-c", "command -v mkdir")
	}

	// First mount: docker copies the image's /nix into the empty volume.
	if out, err := probe(); err != nil {
		t.Fatalf("fresh volume should resolve mkdir: %v\n%s", err, out)
	}

	// Replace the profile link the way a hydrated cache does, with a target
	// that no longer exists. The image's own /nix stays intact at /nix here,
	// so its tools are available to perform the edit.
	if out, err := dockerRun(ctx, "-v", vol+":/mnt/nix", NixCoreImage, "/bin/sh", "-c",
		"ln -sfT /nix/store/00000000000000000000000000000000-gone-root-profile-env /mnt/nix/var/nix/profiles/default"); err != nil {
		t.Fatalf("could not dangle profile link: %v\n%s", err, out)
	}
	if out, err := probe(); err == nil {
		t.Fatalf("setup did not reproduce the broken state; mkdir still resolves:\n%s", out)
	}

	if err := EnsureVolumeShell(ctx, vol, NixCoreImage); err != nil {
		t.Fatalf("EnsureVolumeShell: %v", err)
	}

	if out, err := probe(); err != nil {
		t.Fatalf("after bootstrap mkdir still does not resolve: %v\n%s", err, out)
	}
}

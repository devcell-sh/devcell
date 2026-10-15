package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// volumeShellBootstrapScript runs inside a container where the nix volume
// is mounted at /mnt/nix (NOT /nix), so the image's own /nix is intact.
// It copies the default profile's full closure to /mnt/nix/store/, ensuring
// that /bin/sh (and coreutils, nix, git, etc.) resolve when a later
// container mounts the volume at /nix.
//
// Fast path: if the profile's store path already exists on the volume,
// nothing is copied.
const volumeShellBootstrapScript = `set -e
PROFILE_TARGET=$(readlink -f /nix/var/nix/profiles/default)
VOLUME_BASE=$(basename "$PROFILE_TARGET")
if [ -d "/mnt/nix/store/$VOLUME_BASE" ]; then
  exit 0
fi
DEPS=$(nix-store --query --requisites "$PROFILE_TARGET")
mkdir -p /mnt/nix/store
COPIED=0
for d in $DEPS; do
  BASE=$(basename "$d")
  if [ ! -d "/mnt/nix/store/$BASE" ]; then
    cp -a "$d" "/mnt/nix/store/$BASE"
    COPIED=$((COPIED+1))
  fi
done
mkdir -p /mnt/nix/var/nix/profiles
ln -sfT "$PROFILE_TARGET" /mnt/nix/var/nix/profiles/default
# Core image changed: wipe the devcell-tools profile so the thin build
# recreates it with store paths from the new closure.
rm -f /mnt/nix/var/nix/profiles/devcell-tools*
echo "bootstrap: synced $COPIED store paths, invalidated devcell-tools"
`

// EnsureVolumeShellArgv returns the docker argv that bootstraps the nix
// volume with the core image's shell closure. Pure for testing.
func EnsureVolumeShellArgv(volumeName, coreImage string) []string {
	return []string{
		"docker", "run", "--rm",
		"-v", volumeName + ":/mnt/nix",
		coreImage,
		"/bin/sh", "-c", volumeShellBootstrapScript,
	}
}

// EnsureVolumeShell ensures the nix volume contains the shell closure
// from the core image so /bin/sh resolves when the volume mounts at /nix.
//
// The nixos/nix image's /bin/sh is a symlink into /nix/store/<hash>/bin/bash.
// When the shared nix-store volume mounts over /nix, the symlink target
// must exist on the volume. A stale volume (from a prior image version)
// has different store hashes, making /bin/sh dangle and the thin build
// unbootable.
//
// The fix: mount the volume at /mnt/nix (leaving the image's own /nix
// intact) and copy the default profile's closure to the volume. This also
// covers coreutils, nix, git, and other tools the thin build script needs.
//
// No-op when the volume does not exist yet (Docker auto-populates on
// first mount) or the closure is already present.
func EnsureVolumeShell(ctx context.Context, volumeName, coreImage string) error {
	if !VolumeExists(ctx, volumeName) {
		return nil
	}
	argv := EnsureVolumeShellArgv(volumeName, coreImage)
	out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("volume shell bootstrap: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

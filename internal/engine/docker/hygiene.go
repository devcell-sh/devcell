package docker

import (
	"strconv"
	"strings"
)

// HygieneNixScript removes dangling GC root symlinks and orphaned metadata
// files from /nix/var/nix/gcroots/devcell/. This is the zero-risk subset of
// NixReapSweepScript: it never touches profile generations, never runs
// nix-store --gc, and never modifies the store itself. Only symlinks whose
// targets no longer exist are removed.
//
// Produces a machine-readable summary: stale=N orphaned_meta=N
//
// Exported for test assertions.
const HygieneNixScript = `set -e
STALE=0
ORPHANED=0
if [ -d /nix/var/nix/gcroots/devcell ]; then
  for root in /nix/var/nix/gcroots/devcell/*-profile /nix/var/nix/gcroots/devcell/*-generation; do
    [ -L "$root" ] || continue
    target=$(readlink "$root")
    if ! [ -e "$target" ]; then
      hash=$(basename "$root" | cut -d- -f1)
      rm -f "/nix/var/nix/gcroots/devcell/${hash}-profile"
      rm -f "/nix/var/nix/gcroots/devcell/${hash}-generation"
      rm -f "/nix/var/nix/gcroots/devcell/${hash}-meta"
      STALE=$((STALE + 1))
    fi
  done
  for meta in /nix/var/nix/gcroots/devcell/*-meta; do
    [ -f "$meta" ] || continue
    hash=$(basename "$meta" | cut -d- -f1)
    if [ ! -L "/nix/var/nix/gcroots/devcell/${hash}-profile" ]; then
      rm -f "$meta"
      ORPHANED=$((ORPHANED + 1))
    fi
  done
fi
echo "stale=$STALE orphaned_meta=$ORPHANED"`

// HygieneNixArgv wraps the hygiene script in a docker-run of the nix volume.
func HygieneNixArgv(volume string) []string {
	return []string{
		"docker", "run", "--rm",
		"-v", volume + ":/nix",
		NixCoreImage,
		"/bin/sh", "-c", HygieneNixScript,
	}
}

// ParseHygieneResult finds the summary line in the script output (docker may
// prepend image-pull noise) and extracts stale and orphaned_meta counts.
// Missing or unparseable output yields zeros.
func ParseHygieneResult(output string) (stale, orphanedMeta int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "stale=") {
			continue
		}
		for _, field := range strings.Fields(line) {
			k, v, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			n, _ := strconv.Atoi(v)
			switch k {
			case "stale":
				stale = n
			case "orphaned_meta":
				orphanedMeta = n
			}
		}
		return
	}
	return 0, 0
}

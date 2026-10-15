package cfg

import "strings"

// QualifyImage ensures an image reference is fully qualified for
// Podman compatibility. Docker implicitly prepends docker.io, but
// Podman requires explicit registry prefixes.
//
//   - "alpine"             → "docker.io/library/alpine"
//   - "alpine:3.20"        → "docker.io/library/alpine:3.20"
//   - "nixos/nix:2.34.7"   → "docker.io/nixos/nix:2.34.7"
//   - "ghcr.io/foo/bar:v1" → unchanged (already qualified)
func QualifyImage(ref string) string {
	if ref == "" {
		return ref
	}
	firstSlash := strings.IndexByte(ref, '/')
	if firstSlash == -1 {
		return "docker.io/library/" + ref
	}
	firstComponent := ref[:firstSlash]
	if strings.ContainsAny(firstComponent, ".:") {
		return ref
	}
	return "docker.io/" + ref
}

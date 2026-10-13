package docker

import (
	"context"
	"fmt"
	"strings"

	nixoci "github.com/devcell-sh/go-nixoci"

	"github.com/DimmKirr/devcell/internal/ux"
)

// NixCacheDockerArch normalizes a nix-style arch ("aarch64", "x86_64") to
// Docker convention ("arm64", "amd64").
func NixCacheDockerArch(arch string) string {
	switch arch {
	case "aarch64", "arm64":
		return "arm64"
	case "x86_64", "amd64":
		return "amd64"
	}
	return arch
}

// NixCacheImageRef returns the GHCR nix cache image reference for the given
// registry and Docker-convention arch (arm64 or amd64).
func NixCacheImageRef(registry, arch string) string {
	return fmt.Sprintf("%s:nix-cache-%s-latest", registry, arch)
}

// ShouldHydrateFromCache decides whether to pull the nix cache before building.
// Returns true when the volume is not yet hydrated AND the user hasn't disabled
// cache hydration via DEVCELL_NIX_CACHE=0.
func ShouldHydrateFromCache(hydrated bool, getenv func(string) string) bool {
	if hydrated {
		return false
	}
	v := strings.TrimSpace(getenv("DEVCELL_NIX_CACHE"))
	if v == "0" || v == "false" || v == "off" {
		return false
	}
	return true
}

// HydrateNixCache pulls the nix cache image from GHCR into the named Docker
// volume. Shows a progress spinner. Errors are non-fatal: a failed pull means
// the build runs from scratch (slower but correct).
func HydrateNixCache(ctx context.Context, volumeName, registry, arch string) error {
	dockerArch := NixCacheDockerArch(arch)
	ref := NixCacheImageRef(registry, dockerArch)

	label := fmt.Sprintf("Hydrating nix cache from %s", ref)
	sp := ux.NewProgressSpinner(label)

	resolved, err := nixoci.ResolveImage(ctx, ref)
	if err != nil || resolved == "" {
		sp.Warn(label + " — not available, building from scratch")
		ux.Debugf("nix cache resolve: ref=%s err=%v", ref, err)
		return nil
	}

	if err := nixoci.PullToDockerVolume(ctx, resolved, volumeName, 1); err != nil {
		sp.Warn(label + " — pull failed, building from scratch")
		ux.Debugf("nix cache pull: %v", err)
		return nil
	}

	sp.Success(label)
	return nil
}

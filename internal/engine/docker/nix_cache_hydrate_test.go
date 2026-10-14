package docker_test

import (
	"testing"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

func TestNixCacheImageRef_Arm64(t *testing.T) {
	got := docker.NixCacheImageRef("ghcr.io/devcell-sh/devcell", "arm64")
	want := "ghcr.io/devcell-sh/devcell:nix-cache-arm64-latest"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNixCacheImageRef_Amd64(t *testing.T) {
	got := docker.NixCacheImageRef("ghcr.io/devcell-sh/devcell", "amd64")
	want := "ghcr.io/devcell-sh/devcell:nix-cache-amd64-latest"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShouldHydrateFromCache_EmptyVolume(t *testing.T) {
	env := func(string) string { return "" }
	if !docker.ShouldHydrateFromCache(false, env) {
		t.Error("empty volume should trigger cache hydration")
	}
}

func TestShouldHydrateFromCache_HydratedVolumeSkips(t *testing.T) {
	env := func(string) string { return "" }
	if docker.ShouldHydrateFromCache(true, env) {
		t.Error("already-hydrated volume should skip cache hydration")
	}
}

func TestShouldHydrateFromCache_DisabledByEnv(t *testing.T) {
	env := func(k string) string {
		if k == "DEVCELL_NIX_CACHE" {
			return "0"
		}
		return ""
	}
	if docker.ShouldHydrateFromCache(false, env) {
		t.Error("DEVCELL_NIX_CACHE=0 should disable cache hydration")
	}
}

func TestShouldHydrateFromCache_ExplicitEnable(t *testing.T) {
	env := func(k string) string {
		if k == "DEVCELL_NIX_CACHE" {
			return "1"
		}
		return ""
	}
	if !docker.ShouldHydrateFromCache(false, env) {
		t.Error("DEVCELL_NIX_CACHE=1 with empty volume should hydrate")
	}
}

func TestNixCacheDockerArch_NormalizesArch(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"aarch64", "arm64"},
		{"arm64", "arm64"},
		{"x86_64", "amd64"},
		{"amd64", "amd64"},
	}
	for _, tt := range tests {
		got := docker.NixCacheDockerArch(tt.in)
		if got != tt.want {
			t.Errorf("NixCacheDockerArch(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

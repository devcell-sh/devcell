package docker_test

import (
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine/docker"
	"github.com/DimmKirr/devcell/internal/version"
)

func TestFlakeNixImage_DecoupledFromBinaryVersion(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })

	version.Version = "v0.7.0-50-g8e7d496-dirty"

	got := docker.FlakeNixImage()
	if got != cfg.DefaultNixImage {
		t.Errorf("FlakeNixImage() = %q, want %q (should not depend on version.Version)", got, cfg.DefaultNixImage)
	}
}

func TestFlakeNixImage_RespectsEnvOverride(t *testing.T) {
	t.Setenv("DEVCELL_NIX_IMAGE", "nixos/nix:2.99.0")

	got := docker.FlakeNixImage()
	if got != "nixos/nix:2.99.0" {
		t.Errorf("FlakeNixImage() = %q, want %q (DEVCELL_NIX_IMAGE override)", got, "nixos/nix:2.99.0")
	}
}

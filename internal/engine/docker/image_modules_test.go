package docker_test

import (
	"testing"

	docker "github.com/devcell-sh/devcell/internal/engine/docker"
)

func TestParseImageModules_Typical(t *testing.T) {
	got := docker.ParseImageModules("desktop: 69, base: 35, wireguard: 2")
	for _, want := range []string{"desktop", "base", "wireguard"} {
		if !got[want] {
			t.Errorf("ParseImageModules missing %q, got %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 modules, got %d: %v", len(got), got)
	}
}

func TestParseImageModules_Empty(t *testing.T) {
	got := docker.ParseImageModules("")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestMissingImpliedModules_AllPresent(t *testing.T) {
	missing := docker.MissingImpliedModules("desktop: 69, wireguard: 2", []string{"wireguard"})
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

func TestMissingImpliedModules_WireguardMissing(t *testing.T) {
	missing := docker.MissingImpliedModules("desktop: 69, base: 35", []string{"wireguard"})
	if len(missing) != 1 || missing[0] != "wireguard" {
		t.Errorf("expected [wireguard], got %v", missing)
	}
}

func TestMissingImpliedModules_EmptyLabel(t *testing.T) {
	missing := docker.MissingImpliedModules("", []string{"wireguard"})
	if len(missing) != 1 || missing[0] != "wireguard" {
		t.Errorf("expected [wireguard], got %v", missing)
	}
}

func TestMissingImpliedModules_NoImplied(t *testing.T) {
	missing := docker.MissingImpliedModules("desktop: 69", nil)
	if len(missing) != 0 {
		t.Errorf("expected nil, got %v", missing)
	}
}

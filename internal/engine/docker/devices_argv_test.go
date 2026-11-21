package docker_test

import (
	"testing"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/engine/docker"
)

func TestBuildArgv_UserDevicesAppearInArgv(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.CellCfg.Docker.Devices = []string{"/dev/dri", "virtio/gpu=all"}
	})
	if !hasArg(argv, "--device=/dev/dri") {
		t.Errorf("expected --device=/dev/dri in argv; got: %v", argv)
	}
	if !hasArg(argv, "--device=virtio/gpu=all") {
		t.Errorf("expected --device=virtio/gpu=all in argv; got: %v", argv)
	}
}

func TestBuildArgv_UserDevicesBeforeImage(t *testing.T) {
	spec := docker.RunSpec{
		Config:  baseConfig(),
		CellCfg: cfg.CellConfig{Docker: cfg.DockerSection{Devices: []string{"/dev/dri"}}},
		Binary:  "claude",
		Image:   "devcell-user:test",
	}
	argv := docker.BuildArgv(spec, noopFS(), noopLookPath)

	devIdx, imgIdx := -1, -1
	for i, a := range argv {
		if a == "--device=/dev/dri" {
			devIdx = i
		}
		if a == "devcell-user:test" {
			imgIdx = i
		}
	}
	if devIdx < 0 || imgIdx < 0 {
		t.Fatalf("expected both --device=/dev/dri and image in argv: %v", argv)
	}
	if devIdx > imgIdx {
		t.Errorf("--device=/dev/dri (idx %d) must come before image (idx %d)", devIdx, imgIdx)
	}
}

func TestBuildArgv_UserDevicesDoNotDuplicate(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.CellCfg.Docker.Devices = []string{"/dev/fuse"}
	})
	count := 0
	for _, a := range argv {
		if a == "--device=/dev/fuse" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 --device=/dev/fuse (dedup with hardcoded); got %d in %v", count, argv)
	}
}

func TestBuildArgv_HardcodedDevicesUnchangedWithoutUserDevices(t *testing.T) {
	argv := buildArgv(t)
	if !hasArg(argv, "--device=/dev/fuse") {
		t.Errorf("--device=/dev/fuse must always be present; argv: %v", argv)
	}
}

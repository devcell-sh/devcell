package docker_test

import (
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/engine/docker"
)

// KVM passthrough hands the daemon host's /dev/kvm to the container so QEMU
// can use hardware acceleration instead of TCG.
//
// It must be `--device`, not `-v`. A bind-mount creates the device node but
// the cgroup device controller still denies open(2): `-v /dev/kvm:/dev/kvm`
// yields EPERM, `--device=/dev/kvm` opens fine (verified against live Colima).
//
// When kvm is unset in config, the engine auto-detects by probing the daemon
// host with a throwaway busybox container. RunSpec.KVMAutoDetected carries the
// probe result into BuildArgv.

func TestBuildArgv_KVMAddsDeviceFlag(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.CellCfg.Cell.KVM = boolPtr(true)
	})
	if !hasArg(argv, "--device=/dev/kvm") {
		t.Errorf("kvm=true must emit --device=/dev/kvm; argv: %v", argv)
	}
}

func TestBuildArgv_KVMUsesDeviceNotVolume(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.CellCfg.Cell.KVM = boolPtr(true)
	})
	for i, a := range argv {
		if a == "-v" && i+1 < len(argv) && strings.Contains(argv[i+1], "/dev/kvm") {
			t.Errorf("/dev/kvm must be passed with --device, not -v (cgroup denies open); got -v %q", argv[i+1])
		}
	}
}

func TestBuildArgv_KVMOffByDefault(t *testing.T) {
	argv := buildArgv(t) // CellCfg zero value: KVM unset
	for _, a := range argv {
		if strings.Contains(a, "/dev/kvm") {
			t.Errorf("unset kvm must not emit any /dev/kvm flag; got %q", a)
		}
	}
}

func TestBuildArgv_KVMExplicitFalseOmitsDevice(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.CellCfg.Cell.KVM = boolPtr(false)
	})
	if hasArg(argv, "--device=/dev/kvm") {
		t.Errorf("kvm=false must not emit --device=/dev/kvm; argv: %v", argv)
	}
}

// The existing /dev/fuse device must survive — it is unconditional and
// unrelated to KVM.
func TestBuildArgv_KVMKeepsFuseDevice(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.CellCfg.Cell.KVM = boolPtr(true)
	})
	if !hasArg(argv, "--device=/dev/fuse") {
		t.Errorf("--device=/dev/fuse must still be present; argv: %v", argv)
	}
}

// ── Auto-detect ──────────────────────────────────────────────────────────

func TestBuildArgv_KVMAutoDetectedEmitsDevice(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.KVMAutoDetected = true
	})
	if !hasArg(argv, "--device=/dev/kvm") {
		t.Errorf("auto-detected KVM must emit --device=/dev/kvm; argv: %v", argv)
	}
}

func TestBuildArgv_KVMAutoDetectedRespectsExplicitFalse(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.KVMAutoDetected = true
		s.CellCfg.Cell.KVM = boolPtr(false)
	})
	if hasArg(argv, "--device=/dev/kvm") {
		t.Errorf("explicit kvm=false must override auto-detect; argv: %v", argv)
	}
}

func TestBuildArgv_KVMAutoDetectedDeniedBySolitary(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.KVMAutoDetected = true
		s.Policy = cfg.SolitaryPolicy()
		s.ProxyEnabled = true
	})
	if hasArg(argv, "--device=/dev/kvm") {
		t.Error("solitary policy must deny auto-detected KVM")
	}
}

func TestBuildArgv_KVMAutoDetectedSkippedWhenProxyEnabled(t *testing.T) {
	argv := buildArgv(t, func(s *docker.RunSpec) {
		s.KVMAutoDetected = true
		s.ProxyEnabled = true
	})
	if hasArg(argv, "--device=/dev/kvm") {
		t.Errorf("auto-detected KVM must be skipped when proxy is enabled; argv: %v", argv)
	}
}

// Guard against the flag drifting out of the docker-run flag block (it must
// precede the image name, like --device=/dev/fuse does).
func TestBuildArgv_KVMDeviceBeforeImage(t *testing.T) {
	spec := docker.RunSpec{
		Config:  baseConfig(),
		CellCfg: cfg.CellConfig{Cell: cfg.CellSection{KVM: boolPtr(true)}},
		Binary:  "claude",
		Image:   "devcell-user:test",
	}
	argv := docker.BuildArgv(spec, noopFS(), noopLookPath)

	devIdx, imgIdx := -1, -1
	for i, a := range argv {
		if a == "--device=/dev/kvm" {
			devIdx = i
		}
		if a == "devcell-user:test" {
			imgIdx = i
		}
	}
	if devIdx < 0 || imgIdx < 0 {
		t.Fatalf("expected both --device=/dev/kvm and the image in argv: %v", argv)
	}
	if devIdx > imgIdx {
		t.Errorf("--device=/dev/kvm (idx %d) must come before the image (idx %d)", devIdx, imgIdx)
	}
}

package docker_test

import (
	"testing"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

func TestKVMProbeArgv_UsesBusybox(t *testing.T) {
	argv := docker.KVMProbeArgv()
	if !hasArg(argv, "docker.io/library/busybox:1.36") {
		t.Errorf("probe must use docker.io/library/busybox:1.36; argv: %v", argv)
	}
}

func TestKVMProbeArgv_DeviceFlag(t *testing.T) {
	argv := docker.KVMProbeArgv()
	if !hasArg(argv, "--device=/dev/kvm") {
		t.Errorf("probe must request --device=/dev/kvm; argv: %v", argv)
	}
}

func TestKVMProbeArgv_NoNetwork(t *testing.T) {
	argv := docker.KVMProbeArgv()
	if !hasArg(argv, "--network=none") {
		t.Errorf("probe must use --network=none for isolation; argv: %v", argv)
	}
}

func TestKVMProbeArgv_Rm(t *testing.T) {
	argv := docker.KVMProbeArgv()
	if !hasArg(argv, "--rm") {
		t.Errorf("probe must include --rm; argv: %v", argv)
	}
}

func TestKVMProbeArgv_CommandIsTrue(t *testing.T) {
	argv := docker.KVMProbeArgv()
	last := argv[len(argv)-1]
	if last != "true" {
		t.Errorf("probe command must be 'true'; got %q", last)
	}
}

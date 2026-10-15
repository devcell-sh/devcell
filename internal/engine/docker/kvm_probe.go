package docker

import (
	"context"
	"os/exec"
	"time"

	"github.com/devcell-sh/devcell/internal/cache"
	"github.com/devcell-sh/devcell/internal/ux"
)

const kvmCacheKey = "kvm-probe"
const kvmCacheTTL = 4 * time.Hour

// KVMProbeArgv returns the argv for a throwaway container that tests whether
// the Docker daemon host exposes /dev/kvm. The container does nothing: if
// docker can attach the device node the exit code is 0; otherwise docker
// itself errors out before the container starts.
func KVMProbeArgv() []string {
	return []string{
		"docker", "run", "--rm",
		"--network=none",
		"--device=/dev/kvm",
		"docker.io/library/busybox:1.36",
		"true",
	}
}

// ProbeKVM runs a throwaway container to detect /dev/kvm on the daemon host.
// Results are cached for kvmCacheTTL so the ~100ms probe runs at most once
// per session.
func ProbeKVM(ctx context.Context) bool {
	if v, ok := cache.Load[bool](kvmCacheKey, kvmCacheTTL); ok {
		ux.Debugf("kvm probe: cached result=%v", v)
		return v
	}

	argv := KVMProbeArgv()
	err := exec.CommandContext(ctx, argv[0], argv[1:]...).Run()
	available := err == nil

	ux.Debugf("kvm probe: available=%v (err=%v)", available, err)
	cache.Save(kvmCacheKey, available)
	return available
}

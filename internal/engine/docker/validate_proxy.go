package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// dangerousCaps are capabilities that defeat network isolation.
var dangerousCaps = map[string]bool{
	"NET_ADMIN":     true,
	"SYS_ADMIN":     true,
	"SYS_PTRACE":    true,
	"SYS_MODULE":    true,
	"NET_BROADCAST": true,
	"BPF":           true,
	"PERFMON":       true,
	"SYS_RAWIO":     true,
}

// dangerousMountPrefixes are host paths that defeat network isolation when
// bind-mounted into the container.
var dangerousMountPrefixes = []string{
	"/var/run",
	"/run",
	"/proc",
	"/sys",
}

// ValidateProxyCompatibility checks that no docker/cell settings conflict with
// proxy-mode network isolation. Returns nil when everything is compatible.
// Each error names the offending TOML key and suggests --force-insecure-network.
func ValidateProxyCompatibility(c cfg.CellConfig) error {
	if c.Docker.Privileged {
		return fmt.Errorf("[docker] privileged = true defeats network isolation; "+
			"disable it or use --force-insecure-network to run without the proxy")
	}

	for _, raw := range c.Docker.CapAdd {
		name := strings.ToUpper(strings.TrimPrefix(strings.ToUpper(raw), "CAP_"))
		if dangerousCaps[name] {
			return fmt.Errorf("[docker] cap_add grants %s which defeats network isolation; "+
				"remove it or use --force-insecure-network", name)
		}
	}

	if cfg.WireguardEnabled(c) {
		return fmt.Errorf("[[wireguard]] is enabled: WireGuard needs an external route "+
			"that bypasses the proxy; disable it or use --force-insecure-network")
	}

	if c.Cell.ResolvedKVM() {
		return fmt.Errorf("[cell] kvm = true is unsupported in proxy mode; "+
			"disable it or use --force-insecure-network")
	}

	for _, vol := range c.Volumes {
		src := volumeSource(vol.Mount)
		if src == "" {
			continue
		}
		resolved := src
		if r, err := filepath.EvalSymlinks(src); err == nil {
			resolved = r
		} else if !os.IsNotExist(err) {
			resolved = src
		}
		if isDangerousMount(resolved) {
			return fmt.Errorf("[[volumes]] mount %q exposes %s which defeats network isolation; "+
				"remove it or use --force-insecure-network", vol.Mount, resolved)
		}
	}

	return nil
}

func volumeSource(mount string) string {
	parts := strings.SplitN(mount, ":", 3)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func isDangerousMount(resolved string) bool {
	clean := filepath.Clean(resolved)
	for _, prefix := range dangerousMountPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}
	return false
}

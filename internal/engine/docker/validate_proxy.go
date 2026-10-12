package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DimmKirr/devcell/internal/cfg"
)

// ValidateProxyCompatibility checks that no docker/cell settings conflict with
// proxy-mode network isolation. Returns nil when everything is compatible.
// Each error names the offending TOML key and suggests --force-insecure-network.
//
// The denied caps and mount paths come from SolitaryPolicy as the canonical
// source of truth, keeping validation and BuildArgv enforcement in sync.
func ValidateProxyCompatibility(c cfg.CellConfig) error {
	if c.Docker.Privileged {
		return fmt.Errorf("[docker] privileged = true defeats network isolation; "+
			"disable it or use --force-insecure-network to run without the proxy")
	}

	denied := cfg.SolitaryPolicy().(cfg.DockerPolicy).DeniedCaps()
	for _, raw := range c.Docker.CapAdd {
		name := strings.ToUpper(strings.TrimPrefix(strings.ToUpper(raw), "CAP_"))
		if denied[name] {
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

	deniedPaths := cfg.SolitaryPolicy().(cfg.DockerPolicy).DeniedMountPaths()
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
		if isDangerousMount(resolved, deniedPaths) {
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

func isDangerousMount(resolved string, deniedPaths []string) bool {
	clean := filepath.Clean(resolved)
	for _, prefix := range deniedPaths {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}
	return false
}

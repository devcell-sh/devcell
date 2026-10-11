package cfg

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// ConfigHash returns a short hex digest of the config fields that affect the
// running container. When the hash changes between launches, the container
// must be restarted to pick up the new config.
func ConfigHash(c CellConfig) string {
	h := sha256.New()

	// Stack + modules
	fmt.Fprintf(h, "stack=%s\n", c.Cell.ResolvedStack())
	mods := make([]string, len(c.Cell.Modules))
	copy(mods, c.Cell.Modules)
	sort.Strings(mods)
	fmt.Fprintf(h, "modules=%s\n", strings.Join(mods, ","))

	// Volumes (order matters for Docker, keep as-is)
	for _, v := range c.Volumes {
		fmt.Fprintf(h, "vol=%s\n", v.Resolved())
	}

	// Env (sorted keys for stability)
	envKeys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		fmt.Fprintf(h, "env=%s=%s\n", k, c.Env[k])
	}

	// Ports
	for _, p := range c.Ports.Forward {
		fmt.Fprintf(h, "port=%s\n", p)
	}
	fmt.Fprintf(h, "publish_ip=%s\n", c.Ports.PublishIP)

	// Docker runtime settings
	fmt.Fprintf(h, "privileged=%v\n", c.Docker.Privileged)
	for _, cap := range c.Docker.CapAdd {
		fmt.Fprintf(h, "cap=%s\n", cap)
	}
	fmt.Fprintf(h, "mem=%s\n", c.Docker.MemLimit)
	fmt.Fprintf(h, "cpu=%s\n", c.Docker.CPULimit)
	fmt.Fprintf(h, "shm=%s\n", c.Docker.ShmSize)

	// GUI
	fmt.Fprintf(h, "gui=%v\n", c.GUI.ResolvedEnabled())
	fmt.Fprintf(h, "wm=%s\n", c.GUI.WM)
	fmt.Fprintf(h, "resolution=%s\n", c.GUI.Resolution)
	fmt.Fprintf(h, "scale=%d\n", c.GUI.Scale)

	// MCP servers
	mcps := make([]string, len(c.Mcp.Enabled))
	copy(mcps, c.Mcp.Enabled)
	sort.Strings(mcps)
	fmt.Fprintf(h, "mcps=%s\n", strings.Join(mcps, ","))

	// Packages (nix stable from [cell])
	pkgs := make([]string, len(c.Cell.Packages))
	copy(pkgs, c.Cell.Packages)
	sort.Strings(pkgs)
	fmt.Fprintf(h, "packages=%s\n", strings.Join(pkgs, ","))

	// Network proxy
	fmt.Fprintf(h, "proxy.enabled=%v\n", NetworkProxyEnabled(c))
	fmt.Fprintf(h, "proxy.mode=%s\n", c.Network.Proxy.ResolvedMode())
	proxyAllow := make([]string, len(c.Network.Proxy.Allow))
	copy(proxyAllow, c.Network.Proxy.Allow)
	sort.Strings(proxyAllow)
	fmt.Fprintf(h, "proxy.allow=%s\n", strings.Join(proxyAllow, ","))
	proxyBlock := make([]string, len(c.Network.Proxy.Block))
	copy(proxyBlock, c.Network.Proxy.Block)
	sort.Strings(proxyBlock)
	fmt.Fprintf(h, "proxy.block=%s\n", strings.Join(proxyBlock, ","))
	proxyPresets := make([]string, len(c.Network.Proxy.Presets))
	copy(proxyPresets, c.Network.Proxy.Presets)
	sort.Strings(proxyPresets)
	fmt.Fprintf(h, "proxy.presets=%s\n", strings.Join(proxyPresets, ","))
	fmt.Fprintf(h, "proxy.allow_host_network=%v\n", c.Network.Proxy.ResolvedAllowHostNetwork())

	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/ux"
)

const DefaultSquidImage = "ubuntu/squid:latest@sha256:6a097f68bae708cedbabd6188d68c7e2e7a38cedd05a176e1cc0ba29e3bbe029"

// ProxySidecarOpts holds everything needed to start a proxy sidecar.
type ProxySidecarOpts struct {
	AppName     string
	CellName    string
	Bunk        string
	CellHome    string
	InternalNet string
	EgressNet   string // "devcell-network"
	Proxy       cfg.NetworkProxySection
	SquidImage  string // defaults to DefaultSquidImage if empty
}

// ProxySidecarName returns the gateway sidecar container name for a given app.
func ProxySidecarName(appName string) string {
	return "cell-" + appName + "-gw"
}

// InternalNetworkName returns the internal network name for a given app.
func InternalNetworkName(appName string) string {
	return "cell-" + appName + "-internal"
}

// ProxyConfDir returns the host-side directory for squid config files.
func ProxyConfDir(cellHome, cellName string) string {
	return filepath.Join(cellHome, "proxy")
}

// EnsureIsolatedNetwork creates an --internal docker network if it doesn't
// exist. If isolateGateway is true, the bridge gateway mode is set to
// "isolated" so the host is unreachable from containers on the network.
// Returns an error if the network exists but is not internal.
func EnsureIsolatedNetwork(ctx context.Context, name string, isolateGateway bool) error {
	out, err := exec.CommandContext(ctx, "docker", "network", "inspect", name).Output()
	if err == nil {
		var nets []struct {
			Internal bool `json:"Internal"`
		}
		if jsonErr := json.Unmarshal(out, &nets); jsonErr == nil && len(nets) > 0 {
			if !nets[0].Internal {
				return fmt.Errorf("network %q exists but is not internal; remove it with: docker network rm %s", name, name)
			}
			return nil
		}
		return nil
	}

	args := []string{"network", "create", "--internal"}
	if isolateGateway {
		args = append(args,
			"-o", "com.docker.network.bridge.gateway_mode_ipv4=isolated",
			"-o", "com.docker.network.bridge.gateway_mode_ipv6=isolated",
		)
	}
	args = append(args, name)
	cmd := exec.CommandContext(ctx, "docker", args...)
	if createOut, createErr := cmd.CombinedOutput(); createErr != nil {
		return fmt.Errorf("create internal network %q: %w: %s", name, createErr, strings.TrimSpace(string(createOut)))
	}
	return nil
}

// RemoveNetwork removes a docker network by name. Best-effort.
func RemoveNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", name)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
	return nil
}

// EnsureProxySidecar starts the squid proxy sidecar container. It generates
// the squid config, writes it to disk, starts the container on the internal
// network, connects it to the egress network, and waits for TCP readiness
// on port 3128.
func EnsureProxySidecar(ctx context.Context, opts ProxySidecarOpts) error {
	gwName := ProxySidecarName(opts.AppName)

	if ContainerRunning(ctx, gwName) {
		ux.Debugf("proxy: gateway %s already running", gwName)
		return nil
	}

	// Expand presets into the allow list.
	allow := append([]string{}, opts.Proxy.Allow...)
	if len(opts.Proxy.Presets) > 0 {
		presetDomains, err := cfg.ExpandPresets(opts.Proxy.Presets)
		if err != nil {
			return fmt.Errorf("expand presets: %w", err)
		}
		allow = append(allow, presetDomains...)
		ux.Debugf("proxy: expanded %d presets (%v) into %d domains", len(opts.Proxy.Presets), opts.Proxy.Presets, len(presetDomains))
	}
	ux.Debugf("proxy: allow=%d block=%d mode=%s", len(allow), len(opts.Proxy.Block), opts.Proxy.ResolvedMode())

	confDir := ProxyConfDir(opts.CellHome, opts.CellName)
	ux.Debugf("proxy: config dir=%s", confDir)
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		return fmt.Errorf("create proxy config dir: %w", err)
	}

	squidConf := GenerateSquidConf(opts.Proxy.ResolvedMode())
	if err := os.WriteFile(filepath.Join(confDir, "squid.conf"), []byte(squidConf), 0o644); err != nil {
		return fmt.Errorf("write squid.conf: %w", err)
	}

	aclFiles := GenerateACLFiles(allow, opts.Proxy.Block)
	for name, content := range aclFiles {
		if err := os.WriteFile(filepath.Join(confDir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	image := opts.SquidImage
	if image == "" {
		image = DefaultSquidImage
	}
	ux.Debugf("proxy: image=%s", image)

	// Remove any stopped leftover with the same name.
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", gwName).Run()

	args := []string{
		"run", "-d", "--rm",
		"--name", gwName,
		"--network", opts.InternalNet,
		"--cap-drop=ALL",
		"--add-host", "host.docker.internal:host-gateway",
		"--label", "devcell.cellid=" + opts.Bunk,
		"--label", "devcell.role=proxy",
		"--label", "devcell.cell=" + opts.CellName,
		"--label", "devcell.app=" + opts.AppName,
		"-v", confDir + ":/etc/squid:ro",
		image,
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start gateway %q: %w: %s", gwName, err, strings.TrimSpace(string(out)))
	}

	// Connect gateway to the egress network so it has outbound access.
	connectCmd := exec.CommandContext(ctx, "docker", "network", "connect", opts.EgressNet, gwName)
	if out, err := connectCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("connect gateway %q to %s: %w: %s", gwName, opts.EgressNet, err, strings.TrimSpace(string(out)))
	}

	// Wait for squid to accept connections on port 3128.
	ux.Debugf("proxy: waiting for squid readiness on %s:3128", gwName)
	deadline := time.Now().Add(30 * time.Second)
	for {
		check := exec.CommandContext(ctx, "docker", "exec", gwName,
			"sh", "-c", "cat < /dev/null > /dev/tcp/127.0.0.1/3128 2>/dev/null || nc -z 127.0.0.1 3128 2>/dev/null")
		if check.Run() == nil {
			ux.Debugf("proxy: gateway %s ready", gwName)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("gateway %q: squid never became ready on port 3128 within 30s", gwName)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// RemoveProxySidecar removes the proxy sidecar container. Best-effort.
func RemoveProxySidecar(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
	return nil
}

// CleanupOrphanedProxies finds proxy sidecars whose corresponding cell
// container is no longer running, and removes them along with their
// internal networks.
func CleanupOrphanedProxies(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "docker", "ps",
		"--filter", "label=devcell.role=proxy",
		"--format", "{{.Names}}").Output()
	if err != nil {
		return nil
	}
	names := strings.Fields(strings.TrimSpace(string(out)))
	for _, gwName := range names {
		// cell-<app>-gw -> cell-<app>-run
		cellName := strings.TrimSuffix(gwName, "-gw") + "-run"
		if ContainerRunning(ctx, cellName) {
			continue
		}
		_ = RemoveProxySidecar(ctx, gwName)
		internalNet := strings.TrimSuffix(gwName, "-gw") + "-internal"
		_ = RemoveNetwork(ctx, internalNet)
	}
	return nil
}

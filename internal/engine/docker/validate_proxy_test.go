package docker_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine/docker"
)

func cleanProxyConfig() cfg.CellConfig {
	return cfg.CellConfig{}
}

func TestValidateProxyCompatibility_Clean(t *testing.T) {
	if err := docker.ValidateProxyCompatibility(cleanProxyConfig()); err != nil {
		t.Fatalf("clean config should pass: %v", err)
	}
}

func TestValidateProxyCompatibility_Privileged(t *testing.T) {
	c := cleanProxyConfig()
	c.Docker.Privileged = true
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for privileged mode")
	}
	if !strings.Contains(err.Error(), "privileged") {
		t.Errorf("error should mention privileged: %v", err)
	}
	if !strings.Contains(err.Error(), "--force-insecure-network") {
		t.Errorf("error should mention --force-insecure-network: %v", err)
	}
}

func TestValidateProxyCompatibility_CapNetAdmin(t *testing.T) {
	c := cleanProxyConfig()
	c.Docker.CapAdd = []string{"NET_ADMIN"}
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for NET_ADMIN capability")
	}
	if !strings.Contains(err.Error(), "NET_ADMIN") {
		t.Errorf("error should mention NET_ADMIN: %v", err)
	}
}

func TestValidateProxyCompatibility_CapSysAdmin(t *testing.T) {
	c := cleanProxyConfig()
	c.Docker.CapAdd = []string{"SYS_ADMIN"}
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for SYS_ADMIN capability")
	}
	if !strings.Contains(err.Error(), "SYS_ADMIN") {
		t.Errorf("error should mention SYS_ADMIN: %v", err)
	}
}

func TestValidateProxyCompatibility_CapNormalized(t *testing.T) {
	c := cleanProxyConfig()
	c.Docker.CapAdd = []string{"cap_net_admin"}
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for cap_net_admin (normalized to NET_ADMIN)")
	}
	if !strings.Contains(err.Error(), "NET_ADMIN") {
		t.Errorf("error should mention NET_ADMIN after normalization: %v", err)
	}
}

func TestValidateProxyCompatibility_DockerSocket(t *testing.T) {
	c := cleanProxyConfig()
	c.Volumes = []cfg.VolumeMount{
		{Mount: "/var/run/docker.sock:/var/run/docker.sock"},
	}
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for docker socket mount")
	}
	if !strings.Contains(err.Error(), "docker") || !strings.Contains(err.Error(), "sock") {
		t.Errorf("error should mention docker socket: %v", err)
	}
}

func TestValidateProxyCompatibility_ProcMount(t *testing.T) {
	c := cleanProxyConfig()
	c.Volumes = []cfg.VolumeMount{
		{Mount: "/proc:/host-proc:ro"},
	}
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for /proc mount")
	}
	if !strings.Contains(err.Error(), "/proc") {
		t.Errorf("error should mention /proc: %v", err)
	}
}

func TestValidateProxyCompatibility_WireGuard(t *testing.T) {
	c := cleanProxyConfig()
	c.Wireguard = []cfg.WireguardEntry{
		{Name: "wg0", Enabled: true, Config: "[Interface]\nAddress = 10.0.0.1/32\n[Peer]\nPublicKey = abc=\n"},
	}
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for WireGuard enabled")
	}
	if !strings.Contains(err.Error(), "WireGuard") && !strings.Contains(err.Error(), "wireguard") {
		t.Errorf("error should mention WireGuard: %v", err)
	}
}

func TestValidateProxyCompatibility_KVM(t *testing.T) {
	c := cleanProxyConfig()
	b := true
	c.Cell.KVM = &b
	err := docker.ValidateProxyCompatibility(c)
	if err == nil {
		t.Fatal("expected error for KVM enabled")
	}
	if !strings.Contains(err.Error(), "kvm") && !strings.Contains(err.Error(), "KVM") {
		t.Errorf("error should mention KVM: %v", err)
	}
}

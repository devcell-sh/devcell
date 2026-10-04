package gui_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/gui"
)

func TestProtocol_Spec(t *testing.T) {
	for _, tc := range []struct {
		p                     gui.Protocol
		containerPort, env    string
		label, url, urlPrefix string
	}{
		{gui.RDP, "3389", "EXT_RDP_PORT", "RDP", gui.RDPUrl("389"), "rdp://"},
		{gui.VNC, "5900", "EXT_VNC_PORT", "VNC", gui.VNCUrl("389"), "vnc://"},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			if got := tc.p.ContainerPort(); got != tc.containerPort {
				t.Errorf("ContainerPort() = %q, want %q", got, tc.containerPort)
			}
			if got := tc.p.PortEnv(); got != tc.env {
				t.Errorf("PortEnv() = %q, want %q", got, tc.env)
			}
			if got := tc.p.Label(); got != tc.label {
				t.Errorf("Label() = %q, want %q", got, tc.label)
			}
			got := tc.p.URL("389")
			if got != tc.url {
				t.Errorf("URL(389) = %q, want %q", got, tc.url)
			}
			if !strings.HasPrefix(got, tc.urlPrefix) {
				t.Errorf("URL(389) = %q, want prefix %q", got, tc.urlPrefix)
			}
		})
	}
}

func TestParseDockerPS_Single(t *testing.T) {
	for _, tc := range []struct {
		p      gui.Protocol
		output string
		want   string
	}{
		{gui.RDP, "cell-myproject-3-run\t0.0.0.0:389->3389/tcp", "389"},
		{gui.VNC, "cell-myproject-3-run\t0.0.0.0:350->5900/tcp", "350"},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			m, err := tc.p.ParseDockerPS(tc.output)
			if err != nil {
				t.Fatal(err)
			}
			if m["myproject-3"] != tc.want {
				t.Errorf("want myproject-3→%s, got %v", tc.want, m)
			}
		})
	}
}

func TestParseDockerPS_Multi(t *testing.T) {
	for _, tc := range []struct {
		p                   gui.Protocol
		output              string
		wantProj, wantOther string
	}{
		{gui.RDP, "cell-proj-3-run\t0.0.0.0:389->3389/tcp\ncell-other-5-run\t0.0.0.0:589->3389/tcp", "389", "589"},
		{gui.VNC, "cell-proj-3-run\t0.0.0.0:350->5900/tcp\ncell-other-5-run\t0.0.0.0:550->5900/tcp", "350", "550"},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			m, err := tc.p.ParseDockerPS(tc.output)
			if err != nil {
				t.Fatal(err)
			}
			if m["proj-3"] != tc.wantProj {
				t.Errorf("want proj-3→%s, got %v", tc.wantProj, m)
			}
			if m["other-5"] != tc.wantOther {
				t.Errorf("want other-5→%s, got %v", tc.wantOther, m)
			}
		})
	}
}

func TestParseDockerPS_SkipsOtherPorts(t *testing.T) {
	for _, p := range []gui.Protocol{gui.RDP, gui.VNC} {
		t.Run(string(p), func(t *testing.T) {
			m, err := p.ParseDockerPS("cell-proj-3-run\t0.0.0.0:8080->80/tcp")
			if err != nil {
				t.Fatal(err)
			}
			if len(m) != 0 {
				t.Errorf("expected empty map for non-%s port, got %v", p.ContainerPort(), m)
			}
		})
	}
}

// TestParseDockerPS_OnlyOwnProtocol guards the shared parser against reading
// the other protocol's mapping: a cell publishing both ports yields each
// protocol its own host port, and a cell publishing only the other protocol's
// port is skipped.
func TestParseDockerPS_OnlyOwnProtocol(t *testing.T) {
	output := "cell-both-1-run\t0.0.0.0:389->3389/tcp, 0.0.0.0:350->5900/tcp\n" +
		"cell-rdponly-2-run\t0.0.0.0:489->3389/tcp\n" +
		"cell-vnconly-3-run\t0.0.0.0:450->5900/tcp"
	for _, tc := range []struct {
		p    gui.Protocol
		want map[string]string
	}{
		{gui.RDP, map[string]string{"both-1": "389", "rdponly-2": "489"}},
		{gui.VNC, map[string]string{"both-1": "350", "vnconly-3": "450"}},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			m, err := tc.p.ParseDockerPS(output)
			if err != nil {
				t.Fatal(err)
			}
			if len(m) != len(tc.want) {
				t.Errorf("want %v, got %v", tc.want, m)
			}
			for name, port := range tc.want {
				if m[name] != port {
					t.Errorf("want %s→%s, got %v", name, port, m)
				}
			}
		})
	}
}

func TestParseDockerPS_EmptyOutput(t *testing.T) {
	for _, p := range []gui.Protocol{gui.RDP, gui.VNC} {
		t.Run(string(p), func(t *testing.T) {
			m, err := p.ParseDockerPS("")
			if err != nil {
				t.Fatal(err)
			}
			if len(m) != 0 {
				t.Errorf("expected empty map, got %v", m)
			}
		})
	}
}

// TestParseDockerPS_DirPortMismatch covers the case where a container was
// started from pane %3 (port 7350) but cell vnc is run from pane %11 (port
// 1150). The lookup must return the actual docker port (7350), not the
// env-computed one (1150). The same holds for RDP (7389 vs 1189).
func TestParseDockerPS_DirPortMismatch(t *testing.T) {
	// Simulate: container started with bunk=3, SESSION_PORT_PREFIX=7
	// Container name: cell-devcell-73-3-run, VNC host port: 7350
	// Current pane: %11 → computed port would be 1150 (wrong)
	for _, tc := range []struct {
		p              gui.Protocol
		output         string
		want, computed string
	}{
		{gui.RDP, "cell-devcell-73-3-run\t0.0.0.0:7389->3389/tcp", "7389", "1189"},
		{gui.VNC, "cell-devcell-73-3-run\t0.0.0.0:7350->5900/tcp", "7350", "1150"},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			m, err := tc.p.ParseDockerPS(tc.output)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := m["devcell-73-3"]
			if !ok {
				t.Fatalf("expected entry for devcell-73-3, got %v", m)
			}
			if got != tc.want {
				t.Errorf("want actual port %s, got %q (env-computed would be %s)", tc.want, got, tc.computed)
			}
		})
	}
}

// TestParseDockerPS_MultiSession ensures that when multiple containers of the
// same directory exist, all ports are returned (caller picks one).
func TestParseDockerPS_MultiSession(t *testing.T) {
	for _, tc := range []struct {
		p            gui.Protocol
		output       string
		want3, want5 string
	}{
		{gui.RDP, "cell-devcell-73-3-run\t0.0.0.0:7389->3389/tcp\ncell-devcell-73-5-run\t0.0.0.0:7589->3389/tcp", "7389", "7589"},
		{gui.VNC, "cell-devcell-73-3-run\t0.0.0.0:7350->5900/tcp\ncell-devcell-73-5-run\t0.0.0.0:7550->5900/tcp", "7350", "7550"},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			m, err := tc.p.ParseDockerPS(tc.output)
			if err != nil {
				t.Fatal(err)
			}
			if len(m) != 2 {
				t.Errorf("expected 2 entries, got %d: %v", len(m), m)
			}
			if m["devcell-73-3"] != tc.want3 {
				t.Errorf("want devcell-73-3→%s, got %v", tc.want3, m)
			}
			if m["devcell-73-5"] != tc.want5 {
				t.Errorf("want devcell-73-5→%s, got %v", tc.want5, m)
			}
		})
	}
}

func TestParseInspectPort_Valid(t *testing.T) {
	for _, tc := range []struct {
		p           gui.Protocol
		inspectJSON string
		want        string
	}{
		{gui.RDP, `[{"NetworkSettings":{"Ports":{"3389/tcp":[{"HostIp":"0.0.0.0","HostPort":"389"}]}}}]`, "389"},
		// Minimal docker inspect JSON for port 5900 binding
		{gui.VNC, `[{"NetworkSettings":{"Ports":{"5900/tcp":[{"HostIp":"0.0.0.0","HostPort":"350"}]}}}]`, "350"},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			port, err := tc.p.ParseInspectPort(tc.inspectJSON)
			if err != nil {
				t.Fatal(err)
			}
			if port != tc.want {
				t.Errorf("want %s, got %q", tc.want, port)
			}
		})
	}
}

func TestParseInspectPort_Missing(t *testing.T) {
	for _, p := range []gui.Protocol{gui.RDP, gui.VNC} {
		t.Run(string(p), func(t *testing.T) {
			_, err := p.ParseInspectPort(`[{"NetworkSettings":{"Ports":{}}}]`)
			if err == nil {
				t.Errorf("expected error for missing %s port binding", p.ContainerPort())
			}
		})
	}
}

// TestParseInspectPort_OnlyOwnProtocol: a container publishing both ports
// yields each protocol its own binding; one publishing only the other
// protocol's port is an error.
func TestParseInspectPort_OnlyOwnProtocol(t *testing.T) {
	both := `[{"NetworkSettings":{"Ports":{` +
		`"3389/tcp":[{"HostIp":"0.0.0.0","HostPort":"389"}],` +
		`"5900/tcp":[{"HostIp":"0.0.0.0","HostPort":"350"}]}}}]`
	for _, tc := range []struct {
		p         gui.Protocol
		want      string
		otherOnly string
	}{
		{gui.RDP, "389", `[{"NetworkSettings":{"Ports":{"5900/tcp":[{"HostIp":"0.0.0.0","HostPort":"350"}]}}}]`},
		{gui.VNC, "350", `[{"NetworkSettings":{"Ports":{"3389/tcp":[{"HostIp":"0.0.0.0","HostPort":"389"}]}}}]`},
	} {
		t.Run(string(tc.p), func(t *testing.T) {
			port, err := tc.p.ParseInspectPort(both)
			if err != nil {
				t.Fatal(err)
			}
			if port != tc.want {
				t.Errorf("want %s, got %q", tc.want, port)
			}
			if _, err := tc.p.ParseInspectPort(tc.otherOnly); err == nil {
				t.Errorf("expected error when only the other protocol's port is published")
			}
		})
	}
}

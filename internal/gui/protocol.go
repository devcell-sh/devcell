// Package gui finds the remote-desktop port a running cell publishes and
// builds the URLs and client arguments `cell rdp` and `cell vnc` open it with.
//
// Discovery is the same for both protocols and is parameterized by Protocol.
// Viewer-specific helpers live in rdp.go (FreeRDP, Royal TSX, xrdp cert) and
// vnc.go (VNC password file, Screen Sharing and Royal TSX URLs).
package gui

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Protocol is a remote-desktop protocol a cell serves its desktop over.
type Protocol string

const (
	RDP Protocol = "rdp"
	VNC Protocol = "vnc"
)

// CellEndpoint is the address of a running cell's remote-desktop server.
type CellEndpoint struct {
	Host   string // "127.0.0.1" for Docker/WinKit, VM IP for Tart
	Port   string
	Engine string // "docker", "winkit", "tart"
	User   string // ARD auth user for tart (host $USER); empty for other engines
}

// Addr returns "host:port".
func (e CellEndpoint) Addr() string { return e.Host + ":" + e.Port }

// protocolSpec is everything that differs between the protocols at this layer.
type protocolSpec struct {
	containerPort string // port the server listens on inside the cell
	portEnv       string // env var holding the published host port
}

var specs = map[Protocol]protocolSpec{
	RDP: {containerPort: "3389", portEnv: "EXT_RDP_PORT"},
	VNC: {containerPort: "5900", portEnv: "EXT_VNC_PORT"},
}

// ContainerPort returns the port the protocol's server listens on inside a
// cell: "3389" for RDP, "5900" for VNC.
func (p Protocol) ContainerPort() string { return specs[p].containerPort }

// PortEnv returns the env var devcell sets inside a cell to the published
// host port: EXT_RDP_PORT or EXT_VNC_PORT.
func (p Protocol) PortEnv() string { return specs[p].portEnv }

// Label returns the protocol name as users see it: "RDP" or "VNC".
func (p Protocol) Label() string { return strings.ToUpper(string(p)) }

// TartVNCPassword is the macOS user password set for ARD Screen Sharing
// auth in Tart VMs. Matches the password set by GenerateCreateSessionUserScript
// (provision.go) and hosts/macos/default.nix.
const TartVNCPassword = "admin"

// URL returns the connection URL for a cell endpoint.
// Tart macOS VMs use ARD auth (ep.User:admin); other engines use legacy VNC.
func (p Protocol) URL(ep CellEndpoint) string {
	switch p {
	case RDP:
		return RDPUrl(ep.Host, ep.Port)
	case VNC:
		if ep.Engine == "tart" {
			user := ep.User
			if user == "" {
				user = "admin"
			}
			return VNCUrl(ep.Host, ep.Port, user, TartVNCPassword)
		}
		return VNCUrl(ep.Host, ep.Port, "", "vnc")
	}
	return ""
}

// ParseDockerPS parses the output of:
//
//	docker ps --filter "name=cell-" --format "{{.Names}}\t{{.Ports}}"
//
// and returns a map of cell name to the host port published for p.
// Containers that do not publish p's port are skipped.
func (p Protocol) ParseDockerPS(output string) (map[string]string, error) {
	result := map[string]string{}
	output = strings.TrimSpace(output)
	if output == "" {
		return result, nil
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]  // e.g. cell-myproject-3-run
		ports := parts[1] // e.g. 0.0.0.0:389->3389/tcp, 0.0.0.0:350->5900/tcp

		hostPort, ok := p.hostPort(ports)
		if !ok {
			continue
		}

		// Strip "cell-" prefix and "-run" suffix to get the cell name.
		cellName := strings.TrimPrefix(name, "cell-")
		cellName = strings.TrimSuffix(cellName, "-run")
		result[cellName] = hostPort
	}
	return result, nil
}

// hostPort finds the host port mapped to p's container port in a docker ps
// Ports string such as "0.0.0.0:389->3389/tcp, 0.0.0.0:8080->80/tcp".
func (p Protocol) hostPort(ports string) (string, bool) {
	target := "->" + p.ContainerPort() + "/"
	for _, segment := range strings.Split(ports, ",") {
		segment = strings.TrimSpace(segment)
		if !strings.Contains(segment, target) {
			continue
		}
		// "0.0.0.0:389->3389/tcp" -> hostPort "389"
		arrow := strings.Index(segment, "->")
		if arrow < 0 {
			continue
		}
		hostPart := segment[:arrow] // "0.0.0.0:389"
		colon := strings.LastIndex(hostPart, ":")
		if colon < 0 {
			continue
		}
		return hostPart[colon+1:], true
	}
	return "", false
}

// inspectResult is the part of docker inspect JSON ParseInspectPort reads.
type inspectResult struct {
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIp   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

// ParseInspectPort extracts the host port published for p's container port
// from docker inspect JSON output.
func (p Protocol) ParseInspectPort(inspectJSON string) (string, error) {
	var results []inspectResult
	if err := json.Unmarshal([]byte(inspectJSON), &results); err != nil {
		return "", fmt.Errorf("parse inspect JSON: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("empty inspect result")
	}
	key := p.ContainerPort() + "/tcp"
	bindings, ok := results[0].NetworkSettings.Ports[key]
	if !ok || len(bindings) == 0 {
		return "", fmt.Errorf("%s not published", key)
	}
	return bindings[0].HostPort, nil
}

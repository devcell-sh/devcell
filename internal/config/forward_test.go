package config

import (
	"strconv"
	"strings"
	"testing"
)

func TestResolveForwardEntry_PlainPort(t *testing.T) {
	// "3000" → "3000:3000" (no change, just expand)
	got := ResolveForwardEntry("3000", nil)
	if got != "3000:3000" {
		t.Errorf("want 3000:3000, got %q", got)
	}
}

func TestResolveForwardEntry_ExplicitHostAndContainer(t *testing.T) {
	// "8080:3000" → unchanged
	got := ResolveForwardEntry("8080:3000", nil)
	if got != "8080:3000" {
		t.Errorf("want 8080:3000, got %q", got)
	}
}

func TestResolveForwardEntry_TrailingColon_FreePicks(t *testing.T) {
	// "8080:" → container=8080, host=resolve(8080)
	// 8080 is almost certainly free in tests → "8080:8080"
	got := ResolveForwardEntry("8080:", nil)
	parts := strings.SplitN(got, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("want host:container, got %q", got)
	}
	if parts[1] != "8080" {
		t.Errorf("container port must be 8080, got %q", parts[1])
	}
	hostPort, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("host port not numeric: %q", parts[0])
	}
	if hostPort < 1024 || hostPort > 65535 {
		t.Errorf("host port out of range: %d", hostPort)
	}
}

func TestResolveForwardEntry_TrailingColon_BumpsOffTaken(t *testing.T) {
	// "4250:" with 4250 taken → host picks 4251+
	taken := map[int]struct{}{4250: {}}
	got := ResolveForwardEntry("4250:", taken)
	parts := strings.SplitN(got, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("want host:container, got %q", got)
	}
	if parts[1] != "4250" {
		t.Errorf("container port must be 4250, got %q", parts[1])
	}
	hostPort, _ := strconv.Atoi(parts[0])
	if hostPort == 4250 {
		t.Errorf("host port must not be 4250 (taken), got %d", hostPort)
	}
}

func TestResolveForwardEntry_TrailingColon_WithProtocol(t *testing.T) {
	// "8080:/udp" → container=8080/udp, host=resolve(8080)
	got := ResolveForwardEntry("8080:/udp", nil)
	parts := strings.SplitN(got, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("want host:container, got %q", got)
	}
	if parts[1] != "8080/udp" {
		t.Errorf("container port must preserve protocol: want 8080/udp, got %q", parts[1])
	}
}

package cfg

import "testing"

func TestQualifyImage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"alpine", "docker.io/library/alpine"},
		{"alpine:3.20", "docker.io/library/alpine:3.20"},
		{"busybox:1.36", "docker.io/library/busybox:1.36"},
		{"nixos/nix:2.34.7", "docker.io/nixos/nix:2.34.7"},
		{"ubuntu/squid:latest@sha256:abc123", "docker.io/ubuntu/squid:latest@sha256:abc123"},
		{"ghcr.io/devcell-sh/devcell:core", "ghcr.io/devcell-sh/devcell:core"},
		{"docker.io/nixos/nix:2.34.7", "docker.io/nixos/nix:2.34.7"},
		{"registry.example.com/img:v1", "registry.example.com/img:v1"},
		{"localhost:5000/myimg:latest", "localhost:5000/myimg:latest"},
	}
	for _, tt := range tests {
		if got := QualifyImage(tt.input); got != tt.want {
			t.Errorf("QualifyImage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

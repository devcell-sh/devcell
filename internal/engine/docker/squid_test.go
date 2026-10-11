package docker_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/docker"
)

func TestGenerateSquidConf_AllowMode(t *testing.T) {
	conf := docker.GenerateSquidConf("allow")

	required := []string{
		"http_port 3128",
		"cache deny all",
		`acl allowed_dst  dstdomain "/etc/squid/allow_domains.txt"`,
		`acl allowed_cidr dst       "/etc/squid/allow_cidrs.txt"`,
		`acl blocked_dst  dstdomain "/etc/squid/block_domains.txt"`,
		`acl blocked_cidr dst       "/etc/squid/block_cidrs.txt"`,
		"http_access deny blocked_dst",
		"http_access deny blocked_cidr",
		"http_access allow allowed_dst",
		"http_access allow allowed_cidr",
		"http_access deny all",
		"logformat devcell_json",
		"deny_info",
	}
	for _, want := range required {
		if !strings.Contains(conf, want) {
			t.Errorf("allow mode conf missing %q", want)
		}
	}

	if strings.Contains(conf, "http_access allow all\n") {
		t.Error("allow mode conf must not contain 'http_access allow all' as catch-all")
	}
}

func TestGenerateSquidConf_BlockMode(t *testing.T) {
	conf := docker.GenerateSquidConf("block")

	if !strings.Contains(conf, "http_access allow all") {
		t.Error("block mode conf must contain 'http_access allow all'")
	}
	if strings.Contains(conf, "http_access deny all") {
		t.Error("block mode conf must not contain 'http_access deny all' as the final catch-all")
	}
}

func TestGenerateACLFiles_DomainsAndCIDRs(t *testing.T) {
	allow := []string{"api.example.com", "*.github.com", "10.0.0.1", "172.16.0.0/12"}
	block := []string{"pastebin.com", "192.168.1.0/24"}

	files := docker.GenerateACLFiles(allow, block)

	// Check allow domains
	ad := files["allow_domains.txt"]
	if !strings.Contains(ad, "api.example.com") {
		t.Errorf("allow_domains.txt missing api.example.com, got: %q", ad)
	}
	if !strings.Contains(ad, ".github.com") {
		t.Errorf("allow_domains.txt missing .github.com (squid wildcard), got: %q", ad)
	}
	if strings.Contains(ad, "10.0.0.1") {
		t.Error("allow_domains.txt should not contain IP addresses")
	}

	// Check allow CIDRs
	ac := files["allow_cidrs.txt"]
	if !strings.Contains(ac, "10.0.0.1") {
		t.Errorf("allow_cidrs.txt missing 10.0.0.1, got: %q", ac)
	}
	if !strings.Contains(ac, "172.16.0.0/12") {
		t.Errorf("allow_cidrs.txt missing 172.16.0.0/12, got: %q", ac)
	}

	// Check block domains
	bd := files["block_domains.txt"]
	if !strings.Contains(bd, "pastebin.com") {
		t.Errorf("block_domains.txt missing pastebin.com, got: %q", bd)
	}

	// Check block CIDRs
	bc := files["block_cidrs.txt"]
	if !strings.Contains(bc, "192.168.1.0/24") {
		t.Errorf("block_cidrs.txt missing 192.168.1.0/24, got: %q", bc)
	}
}

func TestGenerateACLFiles_EmptyAllow(t *testing.T) {
	files := docker.GenerateACLFiles(nil, []string{"evil.com"})

	if ad := files["allow_domains.txt"]; ad != "" {
		t.Errorf("empty allow should produce empty allow_domains.txt, got: %q", ad)
	}
	if ac := files["allow_cidrs.txt"]; ac != "" {
		t.Errorf("empty allow should produce empty allow_cidrs.txt, got: %q", ac)
	}
	if bd := files["block_domains.txt"]; !strings.Contains(bd, "evil.com") {
		t.Errorf("block_domains.txt should contain evil.com, got: %q", bd)
	}
}

func TestGenerateACLFiles_WildcardExpansion(t *testing.T) {
	files := docker.GenerateACLFiles([]string{"*.example.com"}, nil)

	ad := files["allow_domains.txt"]
	if !strings.Contains(ad, ".example.com") {
		t.Errorf("wildcard *.example.com should produce .example.com, got: %q", ad)
	}
	if strings.Contains(ad, "*.example.com") {
		t.Errorf("wildcard *.example.com should be converted, not passed through literally, got: %q", ad)
	}
}

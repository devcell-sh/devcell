package docker

import (
	"net"
	"strings"
)

// GenerateSquidConf returns a complete squid.conf for the proxy sidecar.
// mode is "allow" (deny by default) or "block" (allow by default).
func GenerateSquidConf(mode string) string {
	defaultVerdict := "http_access deny all"
	if mode == "block" {
		defaultVerdict = "http_access allow all"
	}

	return strings.Join([]string{
		"http_port 3128",
		"pid_filename /tmp/squid.pid",
		"cache deny all",
		"cache_log /tmp/cache.log",
		`acl allowed_dst  dstdomain "/etc/squid/allow_domains.txt"`,
		`acl allowed_cidr dst       "/etc/squid/allow_cidrs.txt"`,
		`acl blocked_dst  dstdomain "/etc/squid/block_domains.txt"`,
		`acl blocked_cidr dst       "/etc/squid/block_cidrs.txt"`,
		"acl CONNECT method CONNECT",
		"http_access deny blocked_dst",
		"http_access deny blocked_cidr",
		"http_access allow allowed_dst",
		"http_access allow allowed_cidr",
		defaultVerdict,
		`logformat devcell_json {"ts":%ts,"client":"%>a","method":"%rm","target":"%ru","code":%>Hs}`,
		"access_log /tmp/access.log devcell_json",
		"deny_info ERR_ACCESS_DENIED blocked_dst",
		"deny_info ERR_ACCESS_DENIED blocked_cidr",
		"",
	}, "\n")
}

// GenerateACLFiles classifies allow and block entries into domain and CIDR
// files suitable for squid dstdomain and dst ACLs. Returns a map of filename
// to file content.
func GenerateACLFiles(allow, block []string) map[string]string {
	ad, ac := classifyEntries(allow)
	bd, bc := classifyEntries(block)
	return map[string]string{
		"allow_domains.txt": joinLines(ad),
		"allow_cidrs.txt":   joinLines(ac),
		"block_domains.txt": joinLines(bd),
		"block_cidrs.txt":   joinLines(bc),
	}
}

func classifyEntries(entries []string) (domains, cidrs []string) {
	for _, e := range entries {
		if isIPOrCIDR(e) {
			cidrs = append(cidrs, e)
		} else {
			domains = append(domains, toSquidDomain(e))
		}
	}
	return
}

func isIPOrCIDR(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// toSquidDomain converts wildcard notation to squid dstdomain format.
// *.example.com -> .example.com (matches subdomains in squid).
func toSquidDomain(d string) string {
	if strings.HasPrefix(d, "*.") {
		return d[1:] // "*.example.com" -> ".example.com"
	}
	return d
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

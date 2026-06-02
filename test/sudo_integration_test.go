//go:build integration

// sudo_integration_test.go — slow, network-dependent L2 tests that exercise
// the full `sudo <nix command>` round-trip. Gated behind `-tags=integration`
// because they fetch from cache.nixos.org (10–30s, depending on cache state).
//
// Run with:
//   go test -tags=integration ./test/ -run TestSudo_NixInstallHtop -count=1
//
// What the gated set adds vs the always-on L2 set:
//   TestSudo_PreservesNixEnv (always-on) proves the env vars survive sudo.
//   TestSudo_NixInstallHtop (gated)       proves they're SUFFICIENT for the
//                                         real-world "install a package" UX.

package container_test

import (
	"strings"
	"testing"
)

// TestSudo_NixInstallHtop pins the user-visible UX: `sudo nix profile add
// nixpkgs#htop` succeeds without --preserve-env= flags. Pre-fix this failed
// with "SSL peer certificate ... was not OK" because sudo's env_reset
// stripped NIX_SSL_CERT_FILE; nix then couldn't trust cache.nixos.org and
// gave up after retries.
func TestSudo_NixInstallHtop(t *testing.T) {
	c := startContainer(t, nil)

	// Install into root's nix profile. Exit 0 = success.
	out, code := exec(t, c, []string{"sudo", "nix", "profile", "add", "nixpkgs#htop"})
	if code != 0 {
		t.Fatalf("`sudo nix profile add nixpkgs#htop` failed (exit=%d) — env_keep insufficient or another regression\n--- output ---\n%s", code, out)
	}
	// SSL failures show up even with exit 0 sometimes (nix retries silently),
	// so explicitly assert no cert errors leaked into the log.
	for _, bad := range []string{
		"SSL peer certificate",
		"unable to get local issuer certificate",
		"unable to download",
	} {
		if strings.Contains(out, bad) {
			t.Errorf("sudo nix output contains cert/download error %q — env_keep entries may be incomplete\n--- output ---\n%s", bad, out)
		}
	}

	// Sanity: htop binary should be reachable in root's profile after install.
	out, code = exec(t, c, []string{"sudo", "/root/.nix-profile/bin/htop", "--version"})
	if code != 0 {
		// Fallback path some nix versions use.
		out, code = exec(t, c, []string{"sudo", "/nix/var/nix/profiles/per-user/root/profile/bin/htop", "--version"})
	}
	if code != 0 || !strings.Contains(strings.ToLower(out), "htop") {
		t.Fatalf("htop not runnable after install (exit=%d): %s", code, out)
	}
}

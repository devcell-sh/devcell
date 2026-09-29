// sudo_test.go — CELL-86: sudo works in pure (nix2container) images.
//
// Background: `pkgs.sudo.override { withPam = false; }` strips PAM linkage
// from the main sudo binary but does NOT propagate to the sudoers.so policy
// plugin, which is built in a separate derivation step. The plugin remains
// PAM-linked and calls pam_start("sudo", ...) at load time. Without a
// /etc/pam.d/sudo present in the image, plugin init aborts with
// "unable to initialize PAM: Critical error - immediate abort" and every
// `sudo` invocation dies before any policy check runs.
//
// Fix: pure-image builder (nixhome/image.nix) stages a permissive
// /etc/pam.d/sudo pointing at pam_permit.so so pam_start succeeds.
//
// L1: file-content wiring on image.nix.
// L2: container exec — `sudo whoami` returns "root" in a fresh pure cell.

package container_test

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// L1 — Wiring checks (file content)
// ---------------------------------------------------------------------------

// TestSudo_WorksInFreshCell pins the user-visible bug. Pre-CELL-86, this
// fails with "unable to initialize PAM: Critical error - immediate abort"
// before sudo even reads /etc/sudoers. With the PAM stub in place, the
// sudoers plugin proceeds to its policy check, sees NOPASSWD:ALL for the
// session user, and runs the command.
func TestSudo_WorksInFreshCell(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, nil)

	out, code := exec(t, c, []string{"sudo", "whoami"})
	if code != 0 {
		t.Fatalf("`sudo whoami` failed (exit=%d) — CELL-86 PAM stub not in image: %s", code, out)
	}
	if strings.TrimSpace(out) != "root" {
		t.Fatalf("`sudo whoami` returned %q, want \"root\"", strings.TrimSpace(out))
	}
}

// TestSudo_NoPamInitErrorOnPlainSudo specifically asserts that the
// pre-CELL-86 error message no longer appears anywhere in sudo's output.
// Catches regressions where the stub is present but malformed.
func TestSudo_NoPamInitErrorOnPlainSudo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, nil)

	out, _ := exec(t, c, []string{"sudo", "-n", "true"})
	pamErrors := []string{
		"unable to initialize PAM",
		"Critical error - immediate abort",
		"PAM account management error",
	}
	for _, msg := range pamErrors {
		if strings.Contains(out, msg) {
			t.Fatalf("sudo emitted pre-CELL-86 PAM error %q: %s", msg, out)
		}
	}
}

// TestSudo_PreservesNixEnv exercises the L2 effect of the env_keep entries:
// nix-related env vars set by the image (NIX_SSL_CERT_FILE, SSL_CERT_FILE,
// LOCALE_ARCHIVE) must survive a sudo invocation. Without env_keep,
// `sudo nix profile add nixpkgs#foo` fails on cert validation against
// cache.nixos.org — that's the user-visible bug this test pins.
//
// Cheap test (no network): just inspect sudo's env. The full
// "sudo nix profile add" round-trip is covered by TestSudo_NixInstallHtop
// behind the integration build tag.
func TestSudo_PreservesNixEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, nil)

	out, code := exec(t, c, []string{"sudo", "env"})
	if code != 0 {
		t.Fatalf("`sudo env` failed (exit=%d): %s", code, out)
	}
	for _, want := range []string{"NIX_SSL_CERT_FILE=", "SSL_CERT_FILE=", "LOCALE_ARCHIVE="} {
		if !strings.Contains(out, want) {
			t.Errorf("sudo env missing %q — env_keep not effective; sudo nix install paths will fail on cert/locale issues\n--- sudo env output ---\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// CELL-358 — setuid wrapper (thin images)
//
// CELL-86 above fixed the pure image only. Thin images resolve sudo from the
// devcell-tools profile on the SHARED /nix volume, where the store is 0555 and
// can never carry a setuid bit. Chmod-ing the store was the old fix and is
// wrong on a shared volume: a `nix profile upgrade` in any cell repoints the
// profile at a fresh 0555 path and breaks sudo in EVERY cell at once.
//
// The entrypoint now installs a setuid copy at /run/wrappers/bin/sudo (NixOS
// security-wrappers pattern), pins its closure with a GC root, and writes the
// PAM stub when missing.
// ---------------------------------------------------------------------------

// TestSudo_SessionUserCanEscalate is the user-visible bug: `sudo` from the
// session user's shell. The CELL-86 L2 tests above exec as uid 0, so they pass
// even when the setuid bit is missing entirely — only an unprivileged caller
// actually exercises the wrapper.
func TestSudo_SessionUserCanEscalate(t *testing.T) {
	if testing.Short() {
		t.Skip("long: starts a container (may build an image); run without -short or set DEVCELL_TEST_IMAGE")
	}
	c := startContainer(t, map[string]string{"HOST_USER": hostUser})

	t.Run("wrapper is setuid root", func(t *testing.T) {
		out, code := exec(t, c, []string{"stat", "-c", "%U %a", "/run/wrappers/bin/sudo"})
		if code != 0 {
			t.Fatalf("/run/wrappers/bin/sudo missing (exit %d) — entrypoint did not install the wrapper", code)
		}
		if !strings.HasPrefix(out, "root ") || !strings.Contains(out, "4755") {
			t.Fatalf("wrapper must be root-owned mode 4755, got %q", out)
		}
	})

	t.Run("wrapper shadows profile sudo on PATH", func(t *testing.T) {
		out, code := asUser(t, c, "command -v sudo")
		if code != 0 {
			t.Fatalf("sudo not on PATH (exit %d)", code)
		}
		if got := strings.TrimSpace(out); got != "/run/wrappers/bin/sudo" {
			t.Fatalf("PATH resolves sudo to %q, want /run/wrappers/bin/sudo — the non-setuid profile sudo is shadowing the wrapper", got)
		}
	})

	t.Run("session user can escalate", func(t *testing.T) {
		out, code := asUser(t, c, "sudo whoami")
		if code != 0 || strings.TrimSpace(out) != "root" {
			t.Fatalf("`sudo whoami` returned %q (exit %d), want \"root\"", strings.TrimSpace(out), code)
		}
	})

	// Scripts hardcode these paths; they must reach the wrapper in both
	// image variants.
	t.Run("fhs paths reach the wrapper", func(t *testing.T) {
		for _, p := range []string{"/bin/sudo", "/usr/bin/sudo"} {
			out, code := asUser(t, c, p+" whoami")
			if code != 0 || strings.TrimSpace(out) != "root" {
				t.Errorf("`%s whoami` returned %q (exit %d), want \"root\"", p, strings.TrimSpace(out), code)
			}
		}
	})
}

// image_nix_provenance_test.go — guard rail that pins the canonical image.nix
// path to the one the flake actually imports.
//
// Post-mortem (2026-05-20): for ~3 commits (e6acce0, 5cefc1f, 4e200c0) image-
// side fixes — sudo no-PAM, /etc/devcell/tool-versions staging, /etc/pam.d/sudo
// PAM stub — were edited into `nixhome/image.nix` and L1 tests read from that
// same path. The tests passed; the built image kept lacking all three
// behaviours. Root cause: the file at `nixhome/image.nix` was an orphan, never
// referenced by the flake. The flake imports `./packages/image.nix` at
// `nixhome/flake.nix:241`. The two files were created in the same commit
// (517b7c3) and quietly diverged.
//
// What the L1 tests were missing: an assertion that the FILE THEY READ is
// actually part of the build's import graph. They asserted "this string
// appears in this file" but not "this file is what the build uses". Below is
// that missing link.
//
// If image.nix ever moves again, this test fails fast and forces every
// content-checking test to be updated to the new canonical path.

package container_test

import (
	"strings"
	"testing"
)

// TestImageNix_CanonicalPathReferencedByFlake asserts that the path L1 tests
// read for image-build assertions is the same path `nixhome/flake.nix` imports.
// If you move image.nix and don't update this test, the build's source-of-truth
// diverges from what the tests check — exactly the bug this guard exists to
// catch.
func TestImageNix_CanonicalPathReferencedByFlake(t *testing.T) {
	const wantPath = "./packages/image.nix"

	flake := readNixhomeFile(t, "flake.nix")
	if !strings.Contains(flake, "import "+wantPath) {
		t.Fatalf(`nixhome/flake.nix does not contain "import %s".

Tests in test/mise_test.go and test/sudo_test.go read content from
"packages/image.nix". If the flake imports a different image.nix, those tests
are checking an orphan file and the assertions don't reflect the built image.

If you have intentionally moved image.nix, update both:
  - this test's wantPath constant
  - every readNixhomeFile(t, "packages/image.nix") call to the new path.`, wantPath)
	}
}

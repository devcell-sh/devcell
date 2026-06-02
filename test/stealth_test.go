package container_test

// stealth_test.go — L1 content assertions on the patchright stealth init.js
// embedded in nixhome/modules/scraping/default.nix. These pin defensive
// patterns added under DIMM-89 (arm64 stealth regressions on chrome.runtime
// + WebGL spoofs). The L2 detection suite (TestMcp_PatchrightDetectionSuite)
// verifies runtime behavior; these tests guard the spec from accidental
// reverts.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readScrapingNix returns the contents of nixhome/modules/scraping/default.nix.
func readScrapingNix(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "nixhome", "modules", "scraping", "default.nix"))
	if err != nil {
		t.Fatalf("read scraping/default.nix: %v", err)
	}
	return string(data)
}

// TestStealth_ChromeRuntime_DefensiveDefine asserts the stealth init.js
// installs window.chrome via Object.defineProperty (with a getter or
// non-writable value descriptor), not via plain `window.chrome = {...}`
// assignment.
//
// On arm64 the CI detection suite saw `window.chrome.runtime` missing AFTER
// our init ran, which is consistent with Chromium late-injecting its own
// window.chrome and overwriting the plain assignment. Object.defineProperty
// with a getter makes the slot non-reassignable.
func TestStealth_ChromeRuntime_DefensiveDefine(t *testing.T) {
	src := readScrapingNix(t)

	// The stealth init.js is a writeTextFile heredoc. Find the chrome mock region.
	if !strings.Contains(src, "Mock chrome.runtime") {
		t.Fatal("scraping/default.nix doesn't contain the stealth-init `Mock chrome.runtime` block — file shape changed")
	}

	// Must use Object.defineProperty on window for the `chrome` slot.
	// Either `Object.defineProperty(window, 'chrome', ...)` or `defineProperty(window, "chrome", ...)`.
	re := regexp.MustCompile(`Object\.defineProperty\s*\(\s*window\s*,\s*['"]chrome['"]`)
	if !re.MatchString(src) {
		t.Fatal("stealth init.js still uses plain `window.chrome = {...}` for the chrome mock — must use `Object.defineProperty(window, 'chrome', ...)` so late Chromium injection can't overwrite chrome.runtime (DIMM-89 arm64 regression)")
	}
}

// TestStealth_WebGL_InstanceLevelPatch asserts the stealth init.js patches
// HTMLCanvasElement.prototype.getContext to wrap the returned WebGL context
// instance's getParameter, in addition to patching
// WebGL[2]RenderingContext.prototype.getParameter.
//
// The prototype patch alone failed on arm64 Mesa/llvmpipe CI:
//
//	"webglVendor":"Google Inc. (Mesa)" (real value, not our 'Intel Inc.' spoof)
//
// Hypothesis: on arm64 Mesa, the WebGL context instance has getParameter as
// an own-property that shadows the prototype. Belt-and-suspenders fix:
// also patch the instance via a wrapped getContext.
func TestStealth_WebGL_InstanceLevelPatch(t *testing.T) {
	src := readScrapingNix(t)

	if !strings.Contains(src, "Spoof WebGL renderer") {
		t.Fatal("scraping/default.nix doesn't contain the WebGL spoof block — file shape changed")
	}

	// Must intercept getContext on HTMLCanvasElement.prototype to apply the
	// instance-level fallback for the prototype-shadowing case.
	if !strings.Contains(src, "HTMLCanvasElement.prototype.getContext") {
		t.Fatal("stealth init.js doesn't patch `HTMLCanvasElement.prototype.getContext` — needed as instance-level fallback when the prototype-only patch is shadowed by own-properties on the returned WebGL context (DIMM-89 arm64 Mesa regression)")
	}
}

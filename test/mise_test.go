// mise_test.go — TDD tests for CELL-85: bake mise installs + shims into image,
// two-level shim PATH for reliable runtime tooling.
//
// L1: file-content / wiring validation (no Docker, no nix runtime needed)
//     - Verifies the Dockerfile and nixhome modules contain the design hooks.
//     - Failing L1 = the wiring is missing; impl needs to land first.
//
// L2: container exec — declared mise tools are on PATH and runnable.
//     - Uses the existing testcontainers harness (image() + startContainer + exec).
//     - Run via `task test:integration -- -run TestMise`.

package container_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// L1 — Wiring checks (file content)
// ---------------------------------------------------------------------------

// TestMise_DeclaredToolsOnPATH asserts every declared mise tool resolves on
// PATH in the session user's login shell inside a fresh container. This is
// the user-visible bug being fixed — the session user is the product
// surface (root login shells go through the base image's /etc/profile and
// are not provisioned by the entrypoint).
func TestMise_DeclaredToolsOnPATH(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})
	declared := declaredMiseTools(t)

	for _, tool := range declared {
		bin := miseBinaryName(tool)
		t.Run(tool, func(t *testing.T) {
			out, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", "command -v " + bin})
			if code != 0 || strings.TrimSpace(out) == "" {
				t.Fatalf("declared mise tool %q (binary %q) not on PATH: out=%q exit=%d", tool, bin, out, code)
			}
			t.Logf("%s → %s", tool, strings.TrimSpace(out))
		})
	}
}

// TestMise_SharedInstalls_DeclaredToolsShared asserts mise resolves every
// declared tool from the read-only baked install dir via
// MISE_SHARED_INSTALL_DIRS (mise ≥2026.3.9 native shared installs, CELL-75)
// — `mise ls` must tag them "(shared)". This is the provenance check: a tool
// that was silently re-downloaded into the user data dir would resolve too,
// but without the tag.
func TestMise_SharedInstalls_DeclaredToolsShared(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})

	out, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", "mise ls 2>/dev/null"})
	if code != 0 {
		t.Fatalf("mise ls failed: exit=%d out=%q", code, out)
	}

	declared := declaredMiseTools(t)
	for _, tool := range declared {
		found := false
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), tool) && strings.Contains(line, "(shared)") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("declared tool %q not resolved from shared install dir (no \"(shared)\" tag in mise ls):\n%s", tool, out)
		}
	}
}

// TestMise_TerraformAndOpentofuOnPATH pins the specific bug (terraform and
// opentofu shims silently missing pre-fix). Named explicitly so future
// readers can grep for the regression.
func TestMise_TerraformAndOpentofuOnPATH(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})

	for _, bin := range []string{"terraform", "tofu"} {
		t.Run(bin, func(t *testing.T) {
			out, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", "command -v " + bin})
			if code != 0 || strings.TrimSpace(out) == "" {
				t.Fatalf("%s missing from PATH (this was the pre-CELL-85 bug): out=%q exit=%d", bin, out, code)
			}
		})
	}
}

// TestMise_Layering_LocalToolVersionsOverridesShared pins the full layering
// contract ("PATH, but for mise installs" — CELL-75):
//  1. no local pin → the tool resolves from the read-only shared (baked) layer
//  2. a project .tool-versions pinning a different version wins over shared
//  3. the missing version auto-installs (exec_auto_install defaults to true)
//     into the USER layer — never into the read-only shared dir
func TestMise_Layering_LocalToolVersionsOverridesShared(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})

	versionRe := regexp.MustCompile(`(?m)^v\d+\.\d+\.\d+$`)

	// 1. Baseline: resolves the shared baked version (no local pin).
	out, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", "cd && node --version"})
	if code != 0 {
		t.Fatalf("baseline node --version failed: exit=%d out=%q", code, out)
	}
	baseline := versionRe.FindString(out)
	if baseline == "" {
		t.Fatalf("no version in baseline output: %q", out)
	}
	// The pin below must differ from the baked version or the test proves nothing.
	const pin = "26.1.0"
	if baseline == "v"+pin {
		t.Fatalf("baked node is now %s — update the pinned test version to keep layers distinct", baseline)
	}

	// 2+3. Project dir pins a version absent from both layers → shim must
	// auto-install it into the user layer and run it.
	out, code = exec(t, c, []string{"gosu", hostUser, "bash", "-lc",
		`mkdir -p ~/app && echo "node ` + pin + `" > ~/app/.tool-versions && cd ~/app && node --version 2>/dev/null`})
	if code != 0 {
		t.Fatalf("pinned node --version failed (shim auto-install broken?): exit=%d out=%q", code, out)
	}
	if got := versionRe.FindString(out); got != "v"+pin {
		t.Fatalf("project .tool-versions must override shared layer: got %q want %q (baseline %s)", got, "v"+pin, baseline)
	}

	// Auto-install landed in the user layer; the shared dir stayed read-only.
	out, _ = exec(t, c, []string{"gosu", hostUser, "bash", "-lc",
		`test -d "$HOME/.local/share/mise/installs/node/` + pin + `" && echo USER-LAYER-OK; ` +
			`test ! -e "/opt/devcell/.local/share/mise/installs/node/` + pin + `" && echo SHARED-UNTOUCHED`})
	for _, want := range []string{"USER-LAYER-OK", "SHARED-UNTOUCHED"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s marker — wrong layer received the auto-install:\n%s", want, out)
		}
	}
}

// TestMise_SharedInstalls_NoUserCopies asserts the user data dir contains no
// copies or symlinks of the baked tools after boot — the old cross-bind
// design symlinked every baked version into $HOME (dangling-link hazard on
// image rebuilds, CELL-75); the shared-installs design must not.
func TestMise_SharedInstalls_NoUserCopies(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L2 in -short mode")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})

	// Only cross-tier links (absolute targets into /opt or elsewhere) are
	// forbidden — mise's own relative version aliases (24 -> ./24.16.0) are
	// legitimate and must survive boot.
	out, _ := exec(t, c, []string{"gosu", hostUser, "bash", "-lc",
		`find "$HOME/.local/share/mise/installs" -maxdepth 2 -type l -lname '/*' 2>/dev/null; true`})
	if links := strings.TrimSpace(out); links != "" {
		t.Errorf("user mise installs dir contains absolute-target symlinks (legacy cross-bind still active):\n%s", links)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// declaredMiseTools parses nixhome/modules/*.nix for `devcell.mise.tools.<name> = "..."`
// declarations and returns the list of tool names.
func declaredMiseTools(t *testing.T) []string {
	t.Helper()
	modulesDir := filepath.Join(nixhomeDir(), "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		t.Fatalf("read nixhome/modules: %v", err)
	}
	// Match `devcell.mise.tools.<name> = "..."` at line start (optional whitespace)
	// — commented-out lines (e.g. `# devcell.mise.tools.python = "3.13.2";` in
	// python.nix) must NOT match, otherwise tests assert against tools that are
	// not in home.packages mise.tools and have no shim.
	re := regexp.MustCompile(`(?m)^\s*devcell\.mise\.tools\.([a-z0-9_-]+)\s*=\s*"`)
	seen := map[string]bool{}
	var tools []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".nix") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(modulesDir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				tools = append(tools, name)
			}
		}
	}
	return tools
}

// TestMise_DeclaredMiseToolsParser_SkipsCommented exercises the parser regex
// in declaredMiseTools against synthetic input: line-anchored matches must
// skip commented declarations and accept whitespace-indented real ones.
// Locks in the fix for the original bug where the parser matched
// `# devcell.mise.tools.python = ...` and tests then demanded a python
// shim that didn't exist in the image's bake step.
func TestMise_DeclaredMiseToolsParser_SkipsCommented(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantMatch bool
	}{
		{"plain declaration", `devcell.mise.tools.go = "1.26.0";`, true},
		{"indented declaration", `  devcell.mise.tools.terraform = "1.14.3";`, true},
		{"tab-indented declaration", "\tdevcell.mise.tools.node = \"24.13.1\";", true},
		{"hash-commented", `# devcell.mise.tools.python = "3.13.2";`, false},
		{"hash-commented indented", `  # devcell.mise.tools.python = "3.13.2";`, false},
		{"hash with no space", `#devcell.mise.tools.python = "3.13.2";`, false},
		{"mid-line text not at start", `foo = devcell.mise.tools.bar = "1";`, false},
	}
	re := regexp.MustCompile(`(?m)^\s*devcell\.mise\.tools\.([a-z0-9_-]+)\s*=\s*"`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := re.MatchString(tc.input)
			if got != tc.wantMatch {
				t.Fatalf("regex match on %q: got %v, want %v", tc.input, got, tc.wantMatch)
			}
		})
	}
}

// miseBinaryName maps a mise tool key to the binary name when they differ.
func miseBinaryName(miseTool string) string {
	switch miseTool {
	case "opentofu":
		return "tofu"
	default:
		return miseTool
	}
}

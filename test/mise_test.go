// mise_test.go: mise runtimes inside the container. Baked installs and shims
// (CELL-85), shared-install layering (CELL-75), the host user's mise env
// vars, and thin-variant runtime checks.
//
// All container tests here are L2 (container exec against an existing
// image). Run via `task test:integration -- -run 'TestMise|TestThinRuntime'`.

package container_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestMise_DeclaredToolsOnPATH asserts every declared mise tool resolves on
// PATH in the host user's login shell inside a fresh container. This is
// the user-visible bug being fixed: the host user is the product
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
// Host-user mise environment and thin-variant runtime
// ---------------------------------------------------------------------------

// TestMise_DataDir -- the host user's MISE_DATA_DIR must be ~/.local/share/mise, not /opt/mise.
func TestMise_DataDir(t *testing.T) {
	c := startEnvContainer(t)

	out, code := asUser(t, c, "echo $MISE_DATA_DIR")
	if code != 0 {
		t.Fatalf("FAIL: could not read MISE_DATA_DIR (exit %d): %s", code, out)
	}

	expected := fmt.Sprintf("/home/%s/.local/share/mise", hostUser)
	if out != expected {
		t.Errorf("FAIL: MISE_DATA_DIR=%q, want %q", out, expected)
	} else {
		t.Logf("PASS: MISE_DATA_DIR=%q", out)
	}
}

// TestMise_NodeViaUserShims -- node must be reachable through ~/.local/share/mise/shims.
func TestMise_NodeViaUserShims(t *testing.T) {
	c := startEnvContainer(t)

	// Shim must live in ~/.local/share/mise/shims, not /opt/mise/shims.
	shimPath, code := asUser(t, c, "which node")
	if code != 0 {
		t.Fatalf("FAIL: node not found on PATH (exit %d): %s", code, shimPath)
	}

	expected := fmt.Sprintf("/home/%s/.local/share/mise/shims/node", hostUser)
	if shimPath != expected {
		t.Errorf("FAIL: node shim at %q, want %q", shimPath, expected)
	} else {
		t.Logf("PASS: node shim at %q", shimPath)
	}

	// Confirm it actually runs.
	out, code := asUser(t, c, "node --version")
	if code != 0 {
		t.Errorf("FAIL: node --version failed (exit %d): %s", code, out)
	} else {
		t.Logf("PASS: node --version: %s", out)
	}
}

// TestMise_UserInstallPreserved -- setup_mise_home must not overwrite a real dir
// with a symlink (user-installed version must be preserved).
func TestMise_UserInstallPreserved(t *testing.T) {
	c := startEnvContainer(t)

	// Create a fake "user-installed" real directory for a non-existent version.
	_, code := exec(t, c, []string{"bash", "-c",
		"mkdir -p /home/" + hostUser + "/.local/share/mise/installs/node/99.99.99/bin && " +
			"printf '#!/bin/sh\\necho v99.99.99\\n' > /home/" + hostUser + "/.local/share/mise/installs/node/99.99.99/bin/node && " +
			"chmod +x /home/" + hostUser + "/.local/share/mise/installs/node/99.99.99/bin/node",
	})
	if code != 0 {
		t.Fatalf("FAIL: could not create fake user install")
	}

	// Re-run the symlink setup logic (simulates what entrypoint does on restart).
	_, code = exec(t, c, []string{"bash", "-c", `
		baked="/opt/mise"
		user_mise="/home/` + hostUser + `/.local/share/mise"
		for tool_dir in "$baked/installs"/*/; do
			[ -d "$tool_dir" ] || continue
			tool_name=$(basename "$tool_dir")
			mkdir -p "$user_mise/installs/$tool_name"
			for ver_dir in "$tool_dir"*/; do
				[ -d "$ver_dir" ] || continue
				ver_name=$(basename "$ver_dir")
				dest="$user_mise/installs/$tool_name/$ver_name"
				[ -d "$dest" ] && [ ! -L "$dest" ] && continue
				ln -sfT "$ver_dir" "$dest"
			done
		done
	`})
	if code != 0 {
		t.Fatalf("FAIL: re-run of symlink setup failed")
	}

	// The real directory must NOT have been replaced by a symlink.
	out, code := exec(t, c, []string{"bash", "-c",
		"test -L /home/" + hostUser + "/.local/share/mise/installs/node/99.99.99 && echo SYMLINK || echo REAL"})
	if code != 0 || strings.TrimSpace(out) != "REAL" {
		t.Errorf("FAIL: user install was converted to symlink: %s", out)
	} else {
		t.Logf("PASS: user install preserved as real dir")
	}
}

// TestMise_DanglingSymlinkCleaned -- dangling symlinks in ~/.local/share/mise/installs/
// must be removed by setup_mise_home.
func TestMise_DanglingSymlinkCleaned(t *testing.T) {
	c := startEnvContainer(t)

	// Inject a dangling symlink pointing to a non-existent /opt/mise path.
	_, code := exec(t, c, []string{"bash", "-c",
		"mkdir -p /home/" + hostUser + "/.local/share/mise/installs/node && " +
			"ln -s /opt/mise/installs/node/0.0.0-nonexistent " +
			"/home/" + hostUser + "/.local/share/mise/installs/node/0.0.0-nonexistent",
	})
	if code != 0 {
		t.Fatalf("FAIL: could not inject dangling symlink")
	}

	// Re-run the dangling-symlink cleanup logic from setup_mise_home.
	_, code = exec(t, c, []string{"bash", "-c", `
		user_mise="/home/` + hostUser + `/.local/share/mise"
		for tool in "$user_mise/installs"/*/; do
			for link in "${tool%/}"/*; do
				if [ -L "$link" ] && [ ! -e "$link" ]; then rm -f "$link"; fi
			done
		done
	`})
	if code != 0 {
		t.Fatalf("FAIL: cleanup logic failed (exit %d)", code)
	}

	// The dangling symlink must be gone.
	out, _ := exec(t, c, []string{"bash", "-c",
		"test -L /home/" + hostUser + "/.local/share/mise/installs/node/0.0.0-nonexistent && echo EXISTS || echo CLEANED",
	})
	if strings.TrimSpace(out) != "CLEANED" {
		t.Errorf("FAIL: dangling symlink still present after cleanup")
	} else {
		t.Logf("PASS: dangling symlink cleaned up")
	}
}

// TestThinRuntime_NodeIsMiseShim verifies node resolves to a mise shim path,
// not a nix profile binary. Thin mode bakes shims at /opt/devcell/.local/share/mise/shims/.
func TestThinRuntime_NodeIsMiseShim(t *testing.T) {
	if !isThinVariant() {
		t.Skip("thin variant only")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})

	shimPath, code := exec(t, c, []string{"sh", "-c", "which node"})
	if code != 0 {
		t.Fatalf("node not found on PATH: %s", shimPath)
	}
	shimPath = strings.TrimSpace(shimPath)
	if !strings.Contains(shimPath, "mise/shims") {
		t.Errorf("node should be a mise shim, got: %s", shimPath)
	}
	t.Logf("node shim at: %s", shimPath)
}

// TestThinRuntime_NodeVersion verifies node --version matches the declared
// mise config version (24.13.1 from nixhome/modules/node.nix).
func TestThinRuntime_NodeVersion(t *testing.T) {
	if !isThinVariant() {
		t.Skip("thin variant only")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})

	out, code := exec(t, c, []string{"sh", "-c", "node --version"})
	if code != 0 {
		t.Fatalf("node --version failed (exit %d): %s", code, out)
	}
	version := strings.TrimSpace(out)
	if !strings.HasPrefix(version, "v24.") {
		t.Errorf("node version should be v24.x (from mise config), got: %s", version)
	}
	t.Logf("node version: %s", version)
}

// TestThinRuntime_AllDeclaredTools verifies all mise-declared tools are installed
// (no "(missing)" in mise ls output).
func TestThinRuntime_AllDeclaredTools(t *testing.T) {
	if !isThinVariant() {
		t.Skip("thin variant only")
	}
	c := startContainer(t, map[string]string{
		"APP_NAME":  "test",
		"HOST_USER": hostUser,
	})

	out, code := exec(t, c, []string{"sh", "-c", "mise ls 2>&1"})
	if code != 0 {
		t.Fatalf("mise ls failed (exit %d): %s", code, out)
	}
	if strings.Contains(out, "(missing)") {
		t.Errorf("some mise tools are missing:\n%s", out)
	} else {
		t.Logf("all mise tools installed:\n%s", out)
	}
}

// TestMise_NoAsdfEnvVarsLeaked -- no ASDF_* environment variables should be
// present in the container after migration to mise.
func TestMise_NoAsdfEnvVarsLeaked(t *testing.T) {
	c := startEnvContainer(t)

	out, code := asUser(t, c, "env | grep ^ASDF_ || true")
	if code != 0 {
		t.Fatalf("FAIL: env command failed (exit %d): %s", code, out)
	}

	if strings.TrimSpace(out) != "" {
		t.Errorf("FAIL: ASDF_* env vars leaked:\n%s", out)
	} else {
		t.Logf("PASS: no ASDF_* env vars")
	}
}

// TestMise_GlobalConfigEnvVar -- MISE_GLOBAL_CONFIG_FILE must point to a valid
// nix store path, not a file in $HOME.
func TestMise_GlobalConfigEnvVar(t *testing.T) {
	c := startEnvContainer(t)

	out, code := asUser(t, c, "echo $MISE_GLOBAL_CONFIG_FILE")
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("FAIL: MISE_GLOBAL_CONFIG_FILE not set (exit %d): %s", code, out)
	}
	if !strings.HasPrefix(out, "/nix/store/") {
		t.Errorf("FAIL: MISE_GLOBAL_CONFIG_FILE=%q, want /nix/store/... path", out)
	}

	// The file must actually exist and be readable.
	_, code = asUser(t, c, fmt.Sprintf("test -f %q", out))
	if code != 0 {
		t.Errorf("FAIL: MISE_GLOBAL_CONFIG_FILE=%q does not exist or is not readable", out)
	} else {
		t.Logf("PASS: MISE_GLOBAL_CONFIG_FILE=%q (valid nix store path)", out)
	}
}

// TestMise_DefaultNpmPackagesEnvVar -- MISE_NODE_DEFAULT_PACKAGES_FILE must point
// to a valid nix store path.
func TestMise_DefaultNpmPackagesEnvVar(t *testing.T) {
	c := startEnvContainer(t)

	out, code := asUser(t, c, "echo $MISE_NODE_DEFAULT_PACKAGES_FILE")
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("FAIL: MISE_NODE_DEFAULT_PACKAGES_FILE not set (exit %d): %s", code, out)
	}
	if !strings.HasPrefix(out, "/nix/store/") {
		t.Errorf("FAIL: MISE_NODE_DEFAULT_PACKAGES_FILE=%q, want /nix/store/... path", out)
	}

	// The file must actually exist and be readable.
	_, code = asUser(t, c, fmt.Sprintf("test -f %q", out))
	if code != 0 {
		t.Errorf("FAIL: MISE_NODE_DEFAULT_PACKAGES_FILE=%q does not exist or is not readable", out)
	} else {
		t.Logf("PASS: MISE_NODE_DEFAULT_PACKAGES_FILE=%q (valid nix store path)", out)
	}
}

// TestMise_NpmToolsAvailable -- npm-installed tools from /opt/npm-tools must work.
func TestMise_NpmToolsAvailable(t *testing.T) {
	c := startEnvContainer(t)

	// npm itself must be available
	out, code := asUser(t, c, "npm --version")
	if code != 0 {
		t.Fatalf("FAIL: npm not available (exit %d): %s", code, out)
	}
	t.Logf("PASS: npm version: %s", out)

	// A tool from /opt/npm-tools should be accessible
	out, code = asUser(t, c, "which mcp-server-patchright 2>/dev/null || which npx 2>/dev/null")
	if code != 0 {
		t.Errorf("FAIL: no npm tools found on PATH (exit %d): %s", code, out)
	} else {
		t.Logf("PASS: npm tool found at: %s", out)
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

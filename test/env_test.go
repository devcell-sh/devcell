// env_test.go: host-user identity, nix profile paths, write permissions, and image metadata inside a running container.

package container_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestEnv_NixLdLibs -- .nix-ld-libs/ directory must contain symlinks to GUI
// shared libraries from the full profile closure.
// Regression: generateNixLdPath activation ran after writeBoundary but before
// linkGeneration, scanning the old/empty profile generation instead of the current one.
func TestEnv_NixLdLibs(t *testing.T) {
	c := startEnvContainer(t)

	// The directory must exist and contain .so files.
	out, code := exec(t, c, []string{"ls", "/opt/devcell/.nix-ld-libs/"})
	if code != 0 || out == "" {
		t.Fatalf("FAIL: /opt/devcell/.nix-ld-libs/ missing or empty (exit %d)", code)
	}
	libs := strings.Fields(out)
	t.Logf("INFO: .nix-ld-libs has %d symlinks", len(libs))

	// Critical shared libraries that Electron/Chromium/non-nix binaries need.
	requiredLibs := []string{
		"libgtk-3.so",     // GTK 3
		"libcairo.so",     // 2D rendering
		"libpango",        // text layout (prefix — libpango-1.0.so etc.)
		"libnss3.so",      // network security
		"libnspr4.so",     // Netscape Portable Runtime
		"libasound.so",    // audio
		"libdbus-1.so",    // D-Bus IPC
		"libxkbcommon.so", // keyboard handling
	}

	for _, req := range requiredLibs {
		found := false
		for _, lib := range libs {
			if strings.Contains(lib, req) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("FAIL: .nix-ld-libs/ missing %q — activation script likely scanned stale profile", req)
		} else {
			t.Logf("PASS: found %q in .nix-ld-libs/", req)
		}
	}
}

// TestEnv_NixLdLibraryPathSession -- the host user's NIX_LD_LIBRARY_PATH must
// point at the merged .nix-ld-libs/ directory.
func TestEnv_NixLdLibraryPathSession(t *testing.T) {
	c := startEnvContainer(t)

	out, code := asUser(t, c, "echo $NIX_LD_LIBRARY_PATH")
	if code != 0 || out == "" {
		t.Fatalf("FAIL: NIX_LD_LIBRARY_PATH not set in the host user's login shell (exit %d)", code)
	}

	if !strings.Contains(out, "/opt/devcell/.nix-ld-libs") {
		t.Errorf("FAIL: NIX_LD_LIBRARY_PATH=%q does not contain /opt/devcell/.nix-ld-libs", out)
	} else {
		t.Logf("PASS: NIX_LD_LIBRARY_PATH contains .nix-ld-libs")
	}
}

// TestEnv_NixProfilePath -- /opt/devcell must exist and contain a .nix-profile.
func TestEnv_NixProfilePath(t *testing.T) {
	c := startEnvContainer(t)

	for _, path := range []string{
		"/opt/devcell",
		"/opt/devcell/.nix-profile",
		"/opt/devcell/.nix-profile/bin",
	} {
		_, code := exec(t, c, []string{"test", "-e", path})
		if code != 0 {
			t.Errorf("FAIL: %s does not exist", path)
		} else {
			t.Logf("PASS: %s exists", path)
		}
	}
}

// TestEnv_NixProfile -- home-manager native profile path must resolve.
func TestEnv_NixProfile(t *testing.T) {
	c := startEnvContainer(t)
	out, code := exec(t, c, []string{"readlink", "-f", "/opt/devcell/.local/state/nix/profiles/profile"})
	if code != 0 || !strings.Contains(out, "/nix/store/") {
		t.Fatalf("FAIL: /opt/devcell/.local/state/nix/profiles/profile doesn't resolve to /nix/store (exit %d): %q", code, out)
	}
	t.Logf("PASS: nix profile -> %s", out)
}

// TestEnv_SessionIdentity -- $HOME and $USER must match HOST_USER.
func TestEnv_SessionIdentity(t *testing.T) {
	c := startEnvContainer(t)

	expectedHome := "/home/" + hostUser

	cases := []struct {
		name     string
		cmd      string
		contains string
	}{
		{"whoami", "whoami", hostUser},
		{"HOME", "echo $HOME", expectedHome},
		{"USER", "echo $USER", hostUser},
	}

	for _, tc := range cases {
		out, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", tc.cmd})
		if code != 0 || !strings.Contains(out, tc.contains) {
			t.Errorf("FAIL %s: want %q, got %q (exit %d)", tc.name, tc.contains, out, code)
		} else {
			t.Logf("PASS %s: %q", tc.name, out)
		}
	}
}

// TestEnv_WritePaths -- GOPATH and home must be writable by the host user.
func TestEnv_WritePaths(t *testing.T) {
	c := startEnvContainer(t)

	// GOPATH must not be inside /opt/devcell.
	// Use $GOPATH env var (set by 05-shell-rc.sh) instead of `go env GOPATH`
	// to avoid requiring the go binary in stacks that don't include it.
	gopath, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", `echo "$GOPATH"`})
	if code != 0 || gopath == "" {
		// Fallback: try go env if available
		gopath, code = exec(t, c, []string{"gosu", hostUser, "bash", "-lc", "go env GOPATH 2>/dev/null"})
		if code != 0 {
			t.Fatalf("FAIL: GOPATH not set and go not available (exit %d): %s", code, gopath)
		}
	}
	if strings.HasPrefix(gopath, "/opt/devcell") {
		t.Errorf("FAIL: GOPATH=%q points into /opt/devcell -- host user can't write there", gopath)
	} else {
		t.Logf("PASS: GOPATH=%q (not in /opt/devcell)", gopath)
	}

	// GOPATH must be writable
	probe := gopath + "/.write-test-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	_, code = exec(t, c, []string{"gosu", hostUser, "bash", "-lc",
		fmt.Sprintf("mkdir -p %q && rmdir %q", probe, probe)})
	if code != 0 {
		t.Errorf("FAIL: GOPATH=%q is not writable by host user", gopath)
	} else {
		t.Logf("PASS: GOPATH=%q is writable", gopath)
	}

	// $HOME must be writable
	_, code = exec(t, c, []string{"gosu", hostUser, "bash", "-lc",
		"touch ~/.write-test && rm ~/.write-test"})
	if code != 0 {
		t.Errorf("FAIL: $HOME is not writable by host user")
	} else {
		t.Logf("PASS: $HOME is writable")
	}

	// /opt/devcell must NOT be writable by the host user (it's the nix env, owned by devcell)
	_, code = exec(t, c, []string{"gosu", hostUser, "bash", "-lc",
		"touch /opt/devcell/.write-test 2>/dev/null"})
	if code == 0 {
		t.Errorf("FAIL: host user can write to /opt/devcell -- should be read-only")
		// cleanup
		exec(t, c, []string{"rm", "-f", "/opt/devcell/.write-test"}) //nolint
	} else {
		t.Logf("PASS: /opt/devcell is read-only for host user")
	}
}

// TestEnv_BasePermissions -- /opt/devcell directories must be owned by devcell (uid 1000).
// /opt/npm-tools and /opt/python-tools were removed when patchright-mcp (CELL-140)
// and codex (CELL-136) moved to nix; neither variant creates them anymore, so they're
// no longer part of the contract.
func TestEnv_BasePermissions(t *testing.T) {
	c := startEnvContainer(t)

	dirs := []string{
		"/opt/devcell",
		"/opt/devcell/.config",
		"/opt/devcell/.config/nix",
		"/opt/devcell/.config/devcell",
		"/opt/devcell/.nix-profile",
		"/opt/mise",
	}

	for _, dir := range dirs {
		out, code := exec(t, c, []string{"stat", "-c", "%u:%g", dir})
		if code != 0 {
			t.Errorf("FAIL: %s does not exist", dir)
			continue
		}
		if out != "1000:1000" {
			t.Errorf("FAIL: %s owned by %s, want 1000:1000", dir, out)
		} else {
			t.Logf("PASS: %s owned by %s", dir, out)
		}
	}
}

// TestEnv_ImageVersionStamps -- build metadata must be discoverable from
// /etc/devcell/metadata.json (the canonical source per CELL-139).
// metadata.json is staged by the thin build (internal/scaffold).
func TestEnv_ImageVersionStamps(t *testing.T) {
	c := startEnvContainer(t)

	out, code := exec(t, c, []string{"cat", "/etc/devcell/metadata.json"})
	if code != 0 {
		t.Fatalf("FAIL: /etc/devcell/metadata.json not found (exit %d) — canonical build metadata file missing", code)
	}

	// Smallest sufficient shape check: it parses as JSON and has at least
	// build_date (the universally-required field). Other fields like
	// git_commit, stack, modules, packages are variant-specific.
	var meta struct {
		BuildDate string `json:"build_date"`
		GitCommit string `json:"git_commit"`
		Stack     string `json:"stack"`
	}
	if err := json.Unmarshal([]byte(out), &meta); err != nil {
		t.Fatalf("FAIL: /etc/devcell/metadata.json is not valid JSON: %v\nraw: %s", err, out)
	}
	if meta.BuildDate == "" {
		t.Fatalf("FAIL: /etc/devcell/metadata.json missing required `build_date` field. raw: %s", out)
	}
	t.Logf("PASS metadata.json: build_date=%q git_commit=%q stack=%q",
		meta.BuildDate, meta.GitCommit, meta.Stack)
}

// TestEnv_StartupTime -- container must reach ready state within budget.
func TestEnv_StartupTime(t *testing.T) {
	const budgetSeconds = 10 // generous -- tighten after refactor confirmed

	start := time.Now()
	_ = startContainer(t, map[string]string{
		"HOST_USER": hostUser,
		"APP_NAME":  "test",
	})
	elapsed := time.Since(start)

	t.Logf("Startup time: %.1fs (budget: %ds)", elapsed.Seconds(), budgetSeconds)
	if elapsed > time.Duration(budgetSeconds)*time.Second {
		t.Errorf("FAIL: startup took %.1fs, over %ds budget", elapsed.Seconds(), budgetSeconds)
	} else {
		t.Logf("PASS: within budget")
	}
}

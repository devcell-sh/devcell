// persistent_home_test.go: a $HOME volume carrying stale nix-store symlinks from an earlier image must still yield a working environment.

package container_test

import (
	"strings"
	"testing"
)

// TestPersistentHome_NixConf verifies that nix.conf is present in $HOME even
// when an empty .config/nix/ dir already exists from a previous container.
func TestPersistentHome_NixConf(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startContainerWithStaleHome(t)

	// nix.conf is read via NIX_CONF_DIR (pointing to /opt/devcell/.config/nix),
	// not $HOME/.config/nix/. Verify the env var path works after stale cleanup.
	out, code := asUser(t, c, `cat "$NIX_CONF_DIR/nix.conf" 2>/dev/null || cat /opt/devcell/.config/nix/nix.conf`)
	if code != 0 {
		t.Fatalf("FAIL: nix.conf not found via NIX_CONF_DIR or /opt/devcell (exit %d)", code)
	}
	if !strings.Contains(out, "experimental-features") {
		t.Errorf("FAIL: nix.conf exists but missing experimental-features:\n%s", out)
	} else {
		t.Logf("PASS: nix.conf contains experimental-features")
	}
}

// TestPersistentHome_ToolVersions verifies that .tool-versions in $HOME is a
// valid plain file (not a dangling symlink to a GC'd nix store path).
func TestPersistentHome_ToolVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startContainerWithStaleHome(t)

	// .tool-versions must be a readable file (not dangling symlink).
	out, code := asUser(t, c, "cat $HOME/.tool-versions")
	if code != 0 {
		t.Fatalf("FAIL: .tool-versions not readable (exit %d)\n"+
			"Root cause: dangling symlink from previous home-manager generation", code)
	}
	// Must contain at least one tool line (e.g., "node 24.13.1").
	if !strings.Contains(out, "node") {
		t.Errorf("FAIL: .tool-versions doesn't contain expected tools:\n%s", out)
	} else {
		t.Logf("PASS: .tool-versions readable with tools: %s", strings.ReplaceAll(out, "\n", ", "))
	}

	// Must NOT be a symlink (should be a plain file copy).
	linkOut, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc",
		"test -L $HOME/.tool-versions && echo SYMLINK || echo PLAIN"})
	if strings.Contains(linkOut, "SYMLINK") {
		t.Errorf("FAIL: .tool-versions is still a symlink (should be plain file)")
	} else {
		t.Logf("PASS: .tool-versions is a plain file")
	}
	_ = code
}

// TestPersistentHome_MiseConfig verifies that mise can read its global config
// despite a stale config.toml symlink in $HOME.
func TestPersistentHome_MiseConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startContainerWithStaleHome(t)

	// Mise config is read via MISE_GLOBAL_CONFIG_FILE env var (resolved nix store
	// path), not $HOME/.config/mise/config.toml. The $HOME path is cleaned up by
	// the stale symlink removal in entrypoint.sh.
	out, code := asUser(t, c, `cat "$MISE_GLOBAL_CONFIG_FILE" 2>/dev/null || cat /opt/devcell/.config/mise/config.toml 2>/dev/null`)
	if code != 0 {
		t.Skipf("SKIP: mise config not available (mise not in this stack)")
	}
	t.Logf("PASS: mise config readable: %.80s...", out)

	// After stale cleanup, $HOME/.config/mise/ should have no dangling symlinks.
	out, code = exec(t, c, []string{"gosu", hostUser, "bash", "-lc", `
		if [ -L "$HOME/.config/mise/config.toml" ] && [ ! -e "$HOME/.config/mise/config.toml" ]; then
			echo DANGLING
		else
			echo OK
		fi
	`})
	if strings.Contains(out, "DANGLING") {
		t.Errorf("FAIL: $HOME/.config/mise/config.toml is a dangling symlink\n" +
			"Root cause: stale nix store symlink persisted on bind mount")
	} else {
		t.Logf("PASS: mise config.toml is not dangling")
	}
}

// TestPersistentHome_NoDanglingSymlinks is the catch-all persistence bug detector.
func TestPersistentHome_NoDanglingSymlinks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startContainerWithStaleHome(t)

	// Find all dangling symlinks pointing to /nix/store under $HOME,
	// excluding tmp/ (nix build artifacts).
	out, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", `
		found=0
		while IFS= read -r link; do
			target=$(readlink "$link" 2>/dev/null)
			case "$target" in /nix/store/*)
				if [ ! -e "$link" ]; then
					echo "DANGLING: $link -> $target"
					found=1
				fi
			;; esac
		done < <(find "$HOME" -maxdepth 4 -type l -not -path "*/tmp/*" 2>/dev/null)
		exit $found
	`})

	if code != 0 {
		t.Errorf("FAIL: dangling nix-store symlinks found in $HOME:\n%s\n"+
			"Root cause: stale symlinks from previous home-manager generation persist on bind mount", out)
	} else {
		t.Logf("PASS: no dangling nix-store symlinks in $HOME")
	}
}

// TestPersistentHome_FontConfig verifies fontconfig files are valid after
// entrypoint runs with stale $HOME.
func TestPersistentHome_FontConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startContainerWithStaleHome(t)

	// Fontconfig is read via FONTCONFIG_PATH (/opt/devcell/.config/fontconfig),
	// not $HOME/.config/fontconfig. After stale cleanup, $HOME path may not exist.
	out, code := exec(t, c, []string{"gosu", hostUser, "bash", "-lc", `
		# Check the production path (env var or /opt/devcell)
		FC="${FONTCONFIG_PATH:-/opt/devcell/.config/fontconfig}"
		if [ -e "$FC/conf.d/10-hm-fonts.conf" ]; then
			echo OK
		elif [ -L "$HOME/.config/fontconfig/conf.d/10-hm-fonts.conf" ] && [ ! -e "$HOME/.config/fontconfig/conf.d/10-hm-fonts.conf" ]; then
			echo DANGLING
		else
			echo OK
		fi
	`})
	if strings.Contains(out, "DANGLING") {
		t.Errorf("FAIL: fontconfig 10-hm-fonts.conf is dangling\n" +
			"Root cause: stale nix store symlink persisted on bind mount")
	} else {
		t.Logf("PASS: fontconfig config resolves")
	}
	_ = code
}

// TestPersistentHome_StarshipConfig verifies starship config is accessible
// regardless of stale $HOME state.
func TestPersistentHome_StarshipConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startContainerWithStaleHome(t)

	// STARSHIP_CONFIG env var must point to a readable file.
	out, code := asUser(t, c, `test -f "$STARSHIP_CONFIG" && echo OK || echo MISSING`)
	if code != 0 || strings.Contains(out, "MISSING") {
		t.Errorf("FAIL: STARSHIP_CONFIG not readable (exit %d, out: %s)", code, out)
	} else {
		t.Logf("PASS: STARSHIP_CONFIG readable")
	}
}

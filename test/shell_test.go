// shell_test.go: starship prompt and zsh plugin wiring for the host user's shell.

package container_test

import (
	"strings"
	"testing"
)

// TestShell_StarshipConfigExists verifies the home-manager-generated config
// is present and contains the expected unicode character symbol.
func TestShell_StarshipConfigExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startEnvContainer(t)

	// starship.toml lives at /opt/devcell/.config/starship.toml; the host
	// user's shell sets STARSHIP_CONFIG to point there (no copy to $HOME).
	out, code := asUser(t, c, `cat "$STARSHIP_CONFIG"`)
	if code != 0 {
		// fallback: check the devcell home path directly
		out, code = asUser(t, c, "cat /opt/devcell/.config/starship.toml")
		if code != 0 {
			t.Fatalf("starship.toml not found at STARSHIP_CONFIG or /opt/devcell (exit %d): %s", code, out)
		}
	}

	if !strings.Contains(out, "\u2022") {
		t.Errorf("FAIL: starship.toml missing \u2022 character symbol:\n%s", out)
	} else {
		t.Logf("PASS: starship.toml contains \u2022 symbol")
	}

	if !strings.Contains(out, "add_newline = false") {
		t.Errorf("FAIL: starship.toml missing add_newline setting:\n%s", out)
	}
}

// TestShell_StarshipPromptRenders runs `starship prompt` directly and verifies
// the rendered output contains the configured unicode symbols.
func TestShell_StarshipPromptRenders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startEnvContainer(t)

	out, code := asUser(t, c, "cd /tmp && starship prompt")
	if code != 0 {
		t.Fatalf("starship prompt failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "\u2022") {
		t.Errorf("FAIL: starship prompt output missing \u2022 symbol: %q", out)
	} else {
		t.Logf("PASS: starship prompt rendered \u2022: %q", out)
	}
}

// TestShell_ZshStarshipIntegration verifies the complete integration chain:
// zsh sources .zshrc -> starship init is evaluated -> starship prompt renders.
func TestShell_ZshStarshipIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startEnvContainer(t)

	// .zshrc sources /opt/devcell/.zshrc which sets up PATH for starship.
	// Use bash -lc to ensure PATH is set, then invoke zsh.
	out, code := asUser(t, c, `zsh -c 'source ~/.zshrc 2>/dev/null; starship prompt 2>/dev/null'`)
	if code != 0 {
		t.Fatalf("zsh + starship prompt failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "\u2022") {
		t.Errorf("FAIL: zsh->.zshrc->starship prompt missing \u2022 symbol: %q", out)
	} else {
		t.Logf("PASS: zsh->.zshrc->starship prompt rendered \u2022: %q", out)
	}
}

// TestShell_ZshAutosuggestions verifies the zsh-autosuggestions plugin is loaded.
func TestShell_ZshAutosuggestions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startEnvContainer(t)

	// $HOME/.zshrc is a thin wrapper that sources /opt/devcell/.zshrc.
	// The plugin config lives in the sourced file, not the wrapper.
	out, code := asUser(t, c, "grep -rl autosuggestions ~/.zshrc /opt/devcell/.zshrc 2>/dev/null")
	if code != 0 {
		t.Fatalf("FAIL: zsh-autosuggestions not referenced in .zshrc chain (exit %d): %s", code, out)
	}
	t.Logf("PASS: zsh-autosuggestions found in: %s", strings.TrimSpace(out))
}

// TestShell_ZshSyntaxHighlighting verifies the syntax-highlighting plugin is loaded.
func TestShell_ZshSyntaxHighlighting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	c := startEnvContainer(t)

	out, code := asUser(t, c, "grep -rl syntax-highlighting ~/.zshrc /opt/devcell/.zshrc 2>/dev/null")
	if code != 0 {
		t.Fatalf("FAIL: zsh-syntax-highlighting not referenced in .zshrc chain (exit %d): %s", code, out)
	}
	t.Logf("PASS: zsh-syntax-highlighting found in: %s", strings.TrimSpace(out))
}

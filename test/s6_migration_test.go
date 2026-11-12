package container_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestS6Migration_NoFragmentReferences scans Go source and docs for stale
// references to the old entrypoint-fragment mechanism, which has been fully
// replaced by s6 oneshot services.
func TestS6Migration_NoFragmentReferences(t *testing.T) {
	banned := []string{
		"/etc/devcell/entrypoint.d",
		"modules/fragments/",
	}

	dirs := []string{
		"../internal/engine/docker",
		"../web/src/content/cell",
		"../.claude/commands",
		"../cmd",
	}

	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			t.Fatalf("resolve %s: %v", dir, err)
		}
		err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			ext := filepath.Ext(path)
			if ext != ".go" && ext != ".md" && ext != ".mdx" {
				return nil
			}
			if strings.Contains(path, "test/results/") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(data)
			rel, _ := filepath.Rel(absDir, path)

			for _, pattern := range banned {
				if strings.Contains(content, pattern) {
					t.Errorf("%s: stale fragment reference %q", rel, pattern)
				}
			}
			return nil
		})
		if err != nil {
			t.Errorf("walk %s: %v", dir, err)
		}
	}
}

// TestS6Migration_NixhomeHasS6Services verifies the nixhome checkout has the
// s6 service directory structure that tests depend on.
func TestS6Migration_NixhomeHasS6Services(t *testing.T) {
	nixhome := nixhomeDir()
	s6Dir := filepath.Join(nixhome, "modules", "s6")
	if _, err := os.Stat(s6Dir); err != nil {
		t.Skipf("nixhome s6 dir not found at %s (set DEVCELL_NIXHOME)", s6Dir)
	}

	services := []string{
		"wireguard",
		"gui-config",
		"mise-packages",
		"claude-config",
		"secrets",
		"chromium-cleanup",
	}
	for _, svc := range services {
		upPath := filepath.Join(s6Dir, svc, "up")
		if _, err := os.Stat(upPath); err != nil {
			t.Errorf("s6 service %s/up missing at %s", svc, upPath)
		}
	}

	ep := filepath.Join(nixhome, "entrypoint.sh")
	data, err := os.ReadFile(ep)
	if err != nil {
		t.Skipf("entrypoint.sh not found at %s", ep)
	}
	if strings.Contains(string(data), "entrypoint.d") {
		t.Error("entrypoint.sh still references entrypoint.d (should use s6)")
	}
}

// TestS6Migration_WgBootScriptNoFragments verifies wgBootScript does not
// write to entrypoint.d or exec the old entrypoint.
func TestS6Migration_WgBootScriptNoFragments(t *testing.T) {
	script := wgBootScript("[Interface]\nAddress = 10.0.0.1/32\n", "echo done")

	if strings.Contains(script, "/etc/devcell/entrypoint.d") {
		t.Error("wgBootScript still writes to /etc/devcell/entrypoint.d")
	}
	if strings.Contains(script, "entrypoint.sh") {
		t.Error("wgBootScript still execs the old entrypoint.sh")
	}
	if !strings.Contains(script, "wg-private-key") {
		t.Error("wgBootScript must write WG_PRIVATE_KEY to /run/secrets")
	}
	if !strings.Contains(script, "exec echo done") {
		t.Error("wgBootScript must exec the inner command")
	}
}

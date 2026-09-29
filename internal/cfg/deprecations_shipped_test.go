package cfg_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

var (
	tableHeaderRe = regexp.MustCompile(`^\[\[?\s*([A-Za-z0-9_.-]+)\s*\]\]?$`)
	keyLineRe     = regexp.MustCompile(`^([A-Za-z0-9_-]+)\s*=`)
)

// deprecatedKeysInText scans TOML text, including commented-out lines, and
// returns the names of deprecated keys it mentions.
func deprecatedKeysInText(text string) []string {
	var found []string
	seen := map[string]bool{}
	var table []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		var path []string
		if m := tableHeaderRe.FindStringSubmatch(line); m != nil {
			table = strings.Split(m[1], ".")
			path = table
		} else if m := keyLineRe.FindStringSubmatch(line); m != nil {
			path = append(append([]string(nil), table...), m[1])
		} else {
			continue
		}
		for _, d := range cfg.Deprecations {
			if len(path) >= len(d.Path) && strings.Join(path[:len(d.Path)], ".") == strings.Join(d.Path, ".") && !seen[d.Name] {
				seen[d.Name] = true
				found = append(found, d.Name)
			}
		}
	}
	return found
}

func TestDeprecatedKeysInText(t *testing.T) {
	text := "# [ports]\n# forward = [\"3000\"]\n[cell]\nports = [\"3000\"]\n# [[volumes]]\n# [op]\n"
	got := strings.Join(deprecatedKeysInText(text), ",")
	if got != "[ports] forward,[[volumes]],[op]" {
		t.Errorf("got %q", got)
	}
	if got := deprecatedKeysInText("[cell]\nvolumes = []\nmcps = []\n[ports]\npublish_ip = \"x\"\n"); len(got) != 0 {
		t.Errorf("shorthands flagged: %v", got)
	}
}

// Init templates and examples are what users copy; they must only show current keys.
func TestShippedConfigsUseNoDeprecatedKeys(t *testing.T) {
	var files []string
	for _, pattern := range []string{
		"../scaffold/templates/*.toml.tmpl",
		"../../examples/*/.devcell.toml",
	} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 3 {
		t.Fatalf("expected templates and examples, found %v", files)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if got := deprecatedKeysInText(string(data)); len(got) > 0 {
			t.Errorf("%s uses deprecated keys (even commented out): %v", f, got)
		}
	}
}

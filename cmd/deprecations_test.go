package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestWarnConfigDeprecations(t *testing.T) {
	c := cfg.CellConfig{DeprecatedUses: []cfg.DeprecatedUse{
		{Deprecation: cfg.Deprecation{Name: "[ports] forward", Replacement: "[cell] ports", Message: "use [cell] ports instead"}, File: "/g/devcell.toml"},
		{Deprecation: cfg.Deprecation{Name: "[mcp] enabled", Replacement: "[cell] mcps", Message: "use [cell] mcps instead"}, File: "/p/.devcell.toml"},
	}}
	var buf bytes.Buffer
	warnConfigDeprecations(&buf, c)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 warning lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "warning: /g/devcell.toml: [ports] forward is deprecated") {
		t.Errorf("line 0 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "use [cell] mcps instead") {
		t.Errorf("line 1 = %q", lines[1])
	}
}

func TestWarnConfigDeprecations_NoneIsSilent(t *testing.T) {
	var buf bytes.Buffer
	warnConfigDeprecations(&buf, cfg.CellConfig{})
	if buf.Len() != 0 {
		t.Errorf("want no output, got %q", buf.String())
	}
}

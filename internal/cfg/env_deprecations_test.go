package cfg_test

import (
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestApplyEnv_PerCellImage_CanonicalName(t *testing.T) {
	c := cfg.CellConfig{}
	cfg.ApplyEnv(&c, func(k string) string {
		if k == "DEVCELL_PER_CELL_IMAGE" {
			return "true"
		}
		return ""
	})
	if c.Cell.PerCellImage == nil || !*c.Cell.PerCellImage {
		t.Errorf("PerCellImage: want true, got %v", c.Cell.PerCellImage)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("canonical env var must not warn, got %v", c.DeprecatedUses)
	}
}

func TestApplyEnv_PerCellImage_LegacyNameFallsBackAndWarns(t *testing.T) {
	c := cfg.CellConfig{}
	cfg.ApplyEnv(&c, func(k string) string {
		if k == "DEVCELL_PER_SESSION_IMAGE" {
			return "true"
		}
		return ""
	})
	if c.Cell.PerCellImage == nil || !*c.Cell.PerCellImage {
		t.Errorf("PerCellImage: want true (from legacy env var), got %v", c.Cell.PerCellImage)
	}
	if len(c.DeprecatedUses) != 1 {
		t.Fatalf("want exactly 1 deprecation warning, got %v", c.DeprecatedUses)
	}
	want := "warning: DEVCELL_PER_SESSION_IMAGE is deprecated and will be removed in a future release: use DEVCELL_PER_CELL_IMAGE instead"
	if got := "warning: " + c.DeprecatedUses[0].Warning(); got != want {
		t.Errorf("Warning() =\n%q\nwant\n%q", got, want)
	}
}

func TestApplyEnv_PerCellImage_CanonicalTakesPrecedenceOverLegacy(t *testing.T) {
	c := cfg.CellConfig{}
	cfg.ApplyEnv(&c, func(k string) string {
		switch k {
		case "DEVCELL_PER_CELL_IMAGE":
			return "true"
		case "DEVCELL_PER_SESSION_IMAGE":
			return "true"
		}
		return ""
	})
	if c.Cell.PerCellImage == nil || !*c.Cell.PerCellImage {
		t.Errorf("PerCellImage: want true, got %v", c.Cell.PerCellImage)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("canonical env var set: must not warn even if legacy is also set, got %v", c.DeprecatedUses)
	}
}

func TestApplyEnv_PerCellImage_NeitherSet(t *testing.T) {
	c := cfg.CellConfig{}
	cfg.ApplyEnv(&c, func(string) string { return "" })
	if c.Cell.PerCellImage != nil {
		t.Errorf("PerCellImage: want nil, got %v", c.Cell.PerCellImage)
	}
	if len(c.DeprecatedUses) != 0 {
		t.Errorf("want no deprecations, got %v", c.DeprecatedUses)
	}
}

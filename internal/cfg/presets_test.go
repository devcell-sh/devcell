package cfg_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestExpandPresets_Known(t *testing.T) {
	domains, err := cfg.ExpandPresets([]string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) == 0 {
		t.Fatal("claude preset should expand to at least one domain")
	}
	want := "api.anthropic.com"
	found := false
	for _, d := range domains {
		if d == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("claude preset should contain %q, got %v", want, domains)
	}
	if !sort.StringsAreSorted(domains) {
		t.Errorf("expanded domains should be sorted, got %v", domains)
	}
}

func TestExpandPresets_Unknown(t *testing.T) {
	_, err := cfg.ExpandPresets([]string{"nonexistent"})
	if err == nil {
		t.Fatal("unknown preset should return an error")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the unknown preset name, got: %v", err)
	}
}

func TestExpandPresets_Includes(t *testing.T) {
	domains, err := cfg.ExpandPresets([]string{"nix"})
	if err != nil {
		t.Fatal(err)
	}
	hasNix := false
	hasGitHub := false
	for _, d := range domains {
		if d == "cache.nixos.org" {
			hasNix = true
		}
		if d == "github.com" {
			hasGitHub = true
		}
	}
	if !hasNix {
		t.Error("nix preset should contain cache.nixos.org")
	}
	if !hasGitHub {
		t.Error("nix preset should include github preset domains (github.com)")
	}
}

func TestExpandPresets_Dedup(t *testing.T) {
	domains, err := cfg.ExpandPresets([]string{"nix", "github", "nix"})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int)
	for _, d := range domains {
		seen[d]++
		if seen[d] > 1 {
			t.Errorf("domain %q appears %d times, should be deduplicated", d, seen[d])
		}
	}
}

func TestExpandPresets_Empty(t *testing.T) {
	domains, err := cfg.ExpandPresets(nil)
	if err != nil {
		t.Fatal(err)
	}
	if domains != nil {
		t.Errorf("nil input should return nil, got %v", domains)
	}

	domains, err = cfg.ExpandPresets([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if domains != nil {
		t.Errorf("empty input should return nil, got %v", domains)
	}
}

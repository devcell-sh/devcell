package cfg_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
)

func TestValidateNixPackageNames_Valid(t *testing.T) {
	np := cfg.NixPackages{
		Stable:   []string{"tmux", "htop", "python3Packages.requests"},
		Unstable: []string{"some-tool"},
		Edge:     []string{"my_pkg"},
	}
	if err := cfg.ValidateNixPackageNames(np); err != nil {
		t.Errorf("valid names should pass, got: %v", err)
	}
}

func TestValidateNixPackageNames_Empty(t *testing.T) {
	if err := cfg.ValidateNixPackageNames(cfg.NixPackages{}); err != nil {
		t.Errorf("empty should pass, got: %v", err)
	}
}

func TestValidateNixPackageNames_InvalidSpace(t *testing.T) {
	np := cfg.NixPackages{Stable: []string{"my package"}}
	err := cfg.ValidateNixPackageNames(np)
	if err == nil {
		t.Fatal("expected error for name with space")
	}
	if !strings.Contains(err.Error(), "my package") {
		t.Errorf("error should mention the bad name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "[cell] packages") {
		t.Errorf("error should name the key the package came from, got: %v", err)
	}
}

func TestValidateNixPackageNames_InvalidEmpty(t *testing.T) {
	np := cfg.NixPackages{Unstable: []string{""}}
	err := cfg.ValidateNixPackageNames(np)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestValidateNixPackageNames_InvalidSpecialChar(t *testing.T) {
	np := cfg.NixPackages{Edge: []string{"pkg;rm -rf"}}
	err := cfg.ValidateNixPackageNames(np)
	if err == nil {
		t.Fatal("expected error for name with semicolon")
	}
}

package tart

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cell"
)

func TestTemplateVMName_Default(t *testing.T) {
	got := TemplateVMName("ultimate", nil)
	want := "devcell-tart-ultimate"
	if got != want {
		t.Errorf("TemplateVMName(\"ultimate\", nil) = %q, want %q", got, want)
	}
}

func TestTemplateVMName_Base(t *testing.T) {
	got := TemplateVMName("base", nil)
	want := "devcell-tart-base"
	if got != want {
		t.Errorf("TemplateVMName(\"base\", nil) = %q, want %q", got, want)
	}
}

func TestTemplateVMName_WithModules(t *testing.T) {
	got := TemplateVMName("dev", []string{"plex", "linear"})
	if !strings.HasPrefix(got, "devcell-tart-dev-linear-plex-") {
		t.Errorf("expected prefix \"devcell-tart-dev-linear-plex-\", got %q", got)
	}
}

func TestTemplateVMName_ModulesSorted(t *testing.T) {
	got := TemplateVMName("dev", []string{"zed", "alpha"})
	alphaIdx := strings.Index(got, "alpha")
	zedIdx := strings.Index(got, "zed")
	if alphaIdx < 0 || zedIdx < 0 {
		t.Fatalf("expected both \"alpha\" and \"zed\" in %q", got)
	}
	if alphaIdx >= zedIdx {
		t.Errorf("expected \"alpha\" before \"zed\" in %q", got)
	}
}

func TestTemplateVMName_Deterministic(t *testing.T) {
	a := TemplateVMName("dev", []string{"plex", "linear"})
	b := TemplateVMName("dev", []string{"plex", "linear"})
	if a != b {
		t.Errorf("non-deterministic: %q != %q", a, b)
	}
}

func TestInstanceVMName(t *testing.T) {
	got := InstanceVMName("DIMM")
	want := "DIMM-tart"
	if got != want {
		t.Errorf("InstanceVMName(\"DIMM\") = %q, want %q", got, want)
	}
}

func TestStackTag_UsedByTemplateVMName(t *testing.T) {
	tag := cell.StackTag("dev", []string{"plex"})
	vmName := TemplateVMName("dev", []string{"plex"})
	if vmName != "devcell-tart-"+tag {
		t.Errorf("TemplateVMName should be devcell-tart-<tag>: got %q, tag=%q", vmName, tag)
	}
}

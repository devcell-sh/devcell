package cfg

import (
	"strings"
	"testing"
)

func TestConfigDrift_NoChange(t *testing.T) {
	c := CellConfig{Cell: CellSection{Stack: "base"}}
	if d := ConfigDrift(c, c); d != "" {
		t.Fatalf("expected empty drift, got %q", d)
	}
}

func TestConfigDrift_StackChange(t *testing.T) {
	old := CellConfig{Cell: CellSection{Stack: "base"}}
	new := CellConfig{Cell: CellSection{Stack: "ultimate"}}
	d := ConfigDrift(old, new)
	if !strings.Contains(d, "stack: base → ultimate") {
		t.Fatalf("expected stack drift, got %q", d)
	}
}

func TestConfigDrift_VolumeAdded(t *testing.T) {
	old := CellConfig{Volumes: []VolumeMount{{Mount: "/data"}}}
	new := CellConfig{Volumes: []VolumeMount{{Mount: "/data"}, {Mount: "/extra"}}}
	d := ConfigDrift(old, new)
	if !strings.Contains(d, "+1 volumes") {
		t.Fatalf("expected +1 volumes, got %q", d)
	}
}

func TestConfigDrift_EnvChange(t *testing.T) {
	old := CellConfig{Env: map[string]string{"A": "1"}}
	new := CellConfig{Env: map[string]string{"A": "2"}}
	d := ConfigDrift(old, new)
	if !strings.Contains(d, "env var") {
		t.Fatalf("expected env change, got %q", d)
	}
}

func TestConfigDrift_MultipleChanges(t *testing.T) {
	old := CellConfig{Cell: CellSection{Stack: "base"}}
	new := CellConfig{
		Cell:    CellSection{Stack: "ultimate"},
		Volumes: []VolumeMount{{Mount: "/data"}},
	}
	d := ConfigDrift(old, new)
	if !strings.Contains(d, "stack") || !strings.Contains(d, "volumes") {
		t.Fatalf("expected multiple drifts, got %q", d)
	}
}

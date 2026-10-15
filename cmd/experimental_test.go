package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/engine"
	"github.com/devcell-sh/devcell/internal/experiments"
)

// The Windows guest is experimental: every way of naming winkit fails without
// engine-windows and resolves with it. Docker and tart are not gated.
func TestResolveEngine_WindowsNeedsExperiment(t *testing.T) {
	t.Setenv(experiments.Env, "")
	gated := []struct {
		name string
		args []string
		cell cfg.CellSection
	}{
		{"--os windows", []string{"--os", "windows", "claude"}, cfg.CellSection{}},
		{"--os winpe", []string{"--os=winpe", "claude"}, cfg.CellSection{}},
		{"--engine winkit", []string{"--engine=winkit", "claude"}, cfg.CellSection{}},
		{"[cell] os", []string{"claude"}, cfg.CellSection{OS: "windows"}},
		{"[cell] engine", []string{"claude"}, cfg.CellSection{Engine: "winkit"}},
	}
	for _, tc := range gated {
		t.Run(tc.name, func(t *testing.T) {
			withOSArgs(t, tc.args...)
			var buf bytes.Buffer
			_, err := resolveEngine(&buf, cfg.CellConfig{Cell: tc.cell})
			var need *experiments.RequiredError
			if !errors.As(err, &need) || need.Name != experiments.Windows.Name {
				t.Fatalf("want *RequiredError for engine-windows, got %v", err)
			}
			if !strings.Contains(err.Error(), "--experimental=engine-windows") {
				t.Errorf("error should say how to enable: %v", err)
			}

			withOSArgs(t, append([]string{"--experimental=engine-windows"}, tc.args...)...)
			got, err := resolveEngine(&buf, cfg.CellConfig{Cell: tc.cell})
			if err != nil || got != engine.Winkit {
				t.Errorf("with the experiment: engine = %q, %v; want winkit", got, err)
			}
		})
	}
	withOSArgs(t, "claude")
	tomlOn := cfg.CellConfig{Cell: cfg.CellSection{OS: "windows", Experimental: []string{"engine-windows"}}}
	if got, err := resolveEngine(&bytes.Buffer{}, tomlOn); err != nil || got != engine.Winkit {
		t.Errorf("[cell] experimental: engine = %q, %v; want winkit", got, err)
	}
	for _, os := range []string{"linux", "macos"} {
		withOSArgs(t, "--os", os, "claude")
		if _, err := resolveEngine(&bytes.Buffer{}, cfg.CellConfig{}); err != nil {
			t.Errorf("--os %s is not gated, got %v", os, err)
		}
	}
}

// `devcell experimental` lists every registered feature with on/off.
func TestExperimentalCmd_ListsRegistryWithState(t *testing.T) {
	t.Setenv(experiments.Env, "ui-cage")
	t.Setenv("HOME", t.TempDir())
	withOSArgs(t, "experimental")
	var out bytes.Buffer
	experimentalCmd.SetOut(&out)
	if err := experimentalCmd.RunE(experimentalCmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, x := range experiments.List() {
		if !strings.Contains(out.String(), x.Name) {
			t.Errorf("listing lacks %s:\n%s", x.Name, out.String())
		}
	}
	for _, line := range strings.Split(out.String(), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		switch f[0] {
		case "ui-cage":
			if f[2] != "on" {
				t.Errorf("ui-cage should be on: %q", line)
			}
		case "engine-windows":
			if f[2] != "off" {
				t.Errorf("engine-windows should be off: %q", line)
			}
		}
	}
}

// A typo in --experimental fails on every engine command, not only when the
// engine is winkit.
func TestResolveEngine_RejectsUnknownExperimentalOnDocker(t *testing.T) {
	t.Setenv(experiments.Env, "")
	withOSArgs(t, "--experimental=nope", "build")
	_, err := resolveEngine(&bytes.Buffer{}, cfg.CellConfig{})
	var unknown *experiments.UnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("want *UnknownError, got %v", err)
	}
}

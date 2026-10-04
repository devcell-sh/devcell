package tart

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/cfg"
	"github.com/DimmKirr/devcell/internal/engine"
)

func TestEngine_RegisteredAsTart(t *testing.T) {
	e, err := engine.For(engine.Tart)
	if err != nil {
		t.Fatalf("engine.For(tart): %v", err)
	}
	if _, ok := e.(Engine); !ok {
		t.Fatalf("engine.For(tart) = %T, want tart.Engine", e)
	}
}

// The stage is checked before anything platform-specific, so a typo fails
// the same way on every host.
func TestBuild_RejectsUnknownStage(t *testing.T) {
	err := Engine{}.Build(context.Background(), engine.BuildOpts{Stage: "partial"})
	if err == nil || !strings.Contains(err.Error(), `--stage must be "base" or "full", got "partial"`) {
		t.Fatalf("Build(Stage: partial) = %v, want the --stage error", err)
	}
}

func TestNewBuildParams(t *testing.T) {
	t.Setenv("DEVCELL_TART_OCI_IMAGE", "")
	cell := engine.Cell{
		Name:     "dev",
		HostHome: "/home/u",
		BaseDir:  "/home/u/proj",
		Stack:    "go",
		Modules:  []string{"infra"},
		Config:   cfg.CellConfig{Cell: cfg.CellSection{TartOCIImage: "ghcr.io/example/macos:1"}},
	}
	base := buildParams{
		cellName:   "dev",
		hostHome:   "/home/u",
		projectDir: "/home/u/proj",
		stack:      "go",
		ociImage:   "ghcr.io/example/macos:1",
		stage:      "full",
	}

	tests := []struct {
		name string
		opts engine.BuildOpts
		want func(p *buildParams)
	}{
		{"empty stage builds the full template", engine.BuildOpts{Cell: cell}, func(p *buildParams) {}},
		{"base stage", engine.BuildOpts{Cell: cell, Stage: "base"}, func(p *buildParams) { p.stage = "base" }},
		{"--stack overrides the resolved stack", engine.BuildOpts{Cell: cell, Stack: "node"}, func(p *buildParams) { p.stack = "node" }},
		{"force", engine.BuildOpts{Cell: cell, Force: true}, func(p *buildParams) { p.force = true }},
		{"update implies force", engine.BuildOpts{Cell: cell, Update: true}, func(p *buildParams) { p.force = true }},
		{"no-cache", engine.BuildOpts{Cell: cell, NoCache: true}, func(p *buildParams) { p.noCache = true }},
		{"dry-run", engine.BuildOpts{Cell: cell, DryRun: true}, func(p *buildParams) { p.dryRun = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newBuildParams(tt.opts)
			if err != nil {
				t.Fatalf("newBuildParams: %v", err)
			}
			want := base
			tt.want(&want)
			if got != want {
				t.Errorf("newBuildParams =\n  %+v\nwant\n  %+v", got, want)
			}
		})
	}
}

func TestNewBuildParams_DefaultOCIImage(t *testing.T) {
	t.Setenv("DEVCELL_TART_OCI_IMAGE", "")
	got, err := newBuildParams(engine.BuildOpts{Cell: engine.Cell{HostHome: "/home/u"}})
	if err != nil {
		t.Fatalf("newBuildParams: %v", err)
	}
	if got.ociImage != cfg.DefaultTartOCIImage {
		t.Errorf("ociImage = %q, want %q", got.ociImage, cfg.DefaultTartOCIImage)
	}
}

// InitOpts.Force regenerates the cell's SSH keypair; without it an existing
// keypair is kept.
func TestPrepareHost_KeepsKeypairUnlessForced(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	key := NewCellSSHPaths(home, "dev").PrivateKey

	if err := prepareHost("dev", home, "base", false); err != nil {
		t.Fatalf("first prepareHost: %v", err)
	}
	first := readFile(t, key)

	if err := prepareHost("dev", home, "base", false); err != nil {
		t.Fatalf("second prepareHost: %v", err)
	}
	if !bytes.Equal(readFile(t, key), first) {
		t.Error("prepareHost without force replaced the existing keypair")
	}

	if err := prepareHost("dev", home, "base", true); err != nil {
		t.Fatalf("forced prepareHost: %v", err)
	}
	if bytes.Equal(readFile(t, key), first) {
		t.Error("prepareHost with force kept the old keypair")
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

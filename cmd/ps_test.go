package main

import (
	"testing"

	"github.com/devcell-sh/devcell/internal/engine/docker"
)

func TestPSCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"ps"})
	if err != nil {
		t.Fatalf("ps command not found: %v", err)
	}
	if cmd.Use != "ps" {
		t.Errorf("unexpected Use: %q", cmd.Use)
	}
}

func TestPSCmd_GlobalFlag(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"ps"})
	if err != nil {
		t.Fatal(err)
	}
	f := cmd.Flags().Lookup("global")
	if f == nil {
		t.Fatal("--global flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("--global default: %q", f.DefValue)
	}
}

func TestPSCmd_WorkspaceFilter(t *testing.T) {
	cells := []docker.CellInfo{
		{Name: "devcell-foo-1", Workspace: "main", Project: "/dev/foo"},
		{Name: "devcell-bar-2", Workspace: "work", Project: "/dev/bar"},
		{Name: "devcell-baz-3", Workspace: "", Project: "/dev/baz"},
	}

	got := docker.FilterByWorkspace(cells, "main", true)
	if len(got) != 2 {
		t.Fatalf("with legacy: want 2 cells, got %d", len(got))
	}
	if got[0].Name != "devcell-foo-1" {
		t.Errorf("first: %q", got[0].Name)
	}
	if got[1].Name != "devcell-baz-3" {
		t.Errorf("second (legacy): %q", got[1].Name)
	}
}

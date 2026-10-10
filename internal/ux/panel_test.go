package ux

import (
	"errors"
	"testing"

	streak "github.com/dimmkirr/go-streak-chart"
)

func init() {
	LogPlainText = true
}

func testGroups(steps ...string) []PanelGroup {
	return []PanelGroup{{Label: "Test", Steps: steps}}
}

func TestBootPanel_PhaseSuccess(t *testing.T) {
	panel := NewBootPanel(testGroups("Step A", "Step B"))
	err := panel.Phase("Step A", func() error { return nil })
	if err != nil {
		t.Fatalf("Phase returned unexpected error: %v", err)
	}
	panel.Finish("done")
}

func TestBootPanel_PhaseError(t *testing.T) {
	panel := NewBootPanel(testGroups("Step A"))
	want := errors.New("boom")
	err := panel.Phase("Step A", func() error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("Phase error = %v, want %v", err, want)
	}
	panel.FinishError("failed")
}

func TestBootPanel_PhaseDetailedSuccess(t *testing.T) {
	panel := NewBootPanel(testGroups("Image"))
	err := panel.PhaseDetailed("Image", func() (string, error) {
		return "sha256:abc", nil
	})
	if err != nil {
		t.Fatalf("PhaseDetailed returned unexpected error: %v", err)
	}
	panel.Finish("done")
}

func TestBootPanel_PhaseDetailedWarn(t *testing.T) {
	panel := NewBootPanel(testGroups("Nix store"))
	err := panel.PhaseDetailedWarn("Nix store", func() (string, bool, error) {
		return "stale", true, nil
	})
	if err != nil {
		t.Fatalf("PhaseDetailedWarn returned unexpected error: %v", err)
	}
	panel.Finish("done")
}

func TestBootPanel_UnknownPhasePassesThrough(t *testing.T) {
	panel := NewBootPanel(testGroups("Known"))
	called := false
	err := panel.Phase("Unknown", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("fn was not called for unknown phase")
	}
	panel.Finish("done")
}

func TestBootPanel_SealSetsDone(t *testing.T) {
	panel := NewBootPanel(testGroups("Cell ready"))
	panel.Seal("Cell ready")
	panel.Finish("done")
}

func TestBootPanel_MultiGroup(t *testing.T) {
	groups := []PanelGroup{
		{Label: "Prepare", Steps: []string{"Network", "Backup"}},
		{Label: "Boot", Steps: []string{"Container", "Services"}},
	}
	panel := NewBootPanel(groups)

	err := panel.Phase("Network", func() error { return nil })
	if err != nil {
		t.Fatalf("Network phase: %v", err)
	}

	err = panel.Phase("Backup", func() error { return nil })
	if err != nil {
		t.Fatalf("Backup phase: %v", err)
	}

	panel.Finish("done")
}

func TestBootPanel_WarnNoteUsesShortTag(t *testing.T) {
	panel := NewBootPanel(testGroups("Nix store"))
	_ = panel.PhaseDetailedWarn("Nix store", func() (string, bool, error) {
		return "drifted — 4 profile hashes (drift) — run: cell build prune --pure", true, nil
	})
	snap := panel.loader.Snapshot()
	note := snap.Note(0, 0)
	if note != "Nix store: drifted" {
		t.Errorf("note = %q, want %q", note, "Nix store: drifted")
	}
	panel.Finish("done")
}

func TestBootPanel_WarnNoteNoSeparatorUsesFullDetail(t *testing.T) {
	panel := NewBootPanel(testGroups("Nix store"))
	_ = panel.PhaseDetailedWarn("Nix store", func() (string, bool, error) {
		return "skipped", true, nil
	})
	snap := panel.loader.Snapshot()
	note := snap.Note(0, 0)
	if note != "Nix store: skipped" {
		t.Errorf("note = %q, want %q", note, "Nix store: skipped")
	}
	panel.Finish("done")
}

func TestBootPanel_VariableRowCols(t *testing.T) {
	groups := []PanelGroup{
		{Label: "Prepare", Steps: []string{"A", "B", "C"}},
		{Label: "Configure", Steps: []string{"X", "Y"}},
	}
	panel := NewBootPanel(groups)
	snap := panel.loader.Snapshot()
	if snap.RowCols(0) != 3 {
		t.Errorf("row 0 cols = %d, want 3", snap.RowCols(0))
	}
	if snap.RowCols(1) != 2 {
		t.Errorf("row 1 cols = %d, want 2", snap.RowCols(1))
	}
	panel.Finish("done")
}

func TestBootPanel_SetBootWithGroup(t *testing.T) {
	groups := []PanelGroup{
		{Label: "Boot", Steps: []string{"Container", "Entrypoint", "Services"}},
		{Label: "Environment", Steps: []string{"Shell", "Mise"}},
	}
	panel := NewBootPanel(groups)

	// Targeted by group + step.
	panel.SetBoot("Boot", "Entrypoint", streak.Done, "Entrypoint ready")
	panel.SetBoot("Environment", "Shell", streak.Done, "Shell ready")
	panel.Finish("done")
}

func TestBootPanel_PromotePending(t *testing.T) {
	groups := []PanelGroup{
		{Label: "Prepare", Steps: []string{"Docker", "Network"}},
		{Label: "Boot", Steps: []string{"Container", "Entrypoint"}},
		{Label: "Services", Steps: []string{"Display", "Audio"}},
	}
	panel := NewBootPanel(groups)

	// Simulate Prepare completing normally.
	_ = panel.Phase("Docker", func() error { return nil })
	_ = panel.Phase("Network", func() error { return nil })

	// Boot: only Container done (attach path).
	panel.SetBoot("Boot", "Container", streak.Done, "Container running")

	// Services: untouched (all Pending).
	// PromotePending should fill in the gaps.
	panel.PromotePending()

	snap := panel.Snapshot()

	// Prepare: already Done, should stay Done.
	for _, col := range []int{0, 1} {
		s, _ := snap.Get(0, col)
		if s != streak.Done {
			t.Errorf("Prepare col %d = %v, want Done", col, s)
		}
	}

	// Boot.Container: already Done.
	s, _ := snap.Get(1, 0)
	if s != streak.Done {
		t.Errorf("Boot.Container = %v, want Done", s)
	}
	// Boot.Entrypoint: was Pending, should now be Done.
	s, _ = snap.Get(1, 1)
	if s != streak.Done {
		t.Errorf("Boot.Entrypoint = %v, want Done (was Pending)", s)
	}

	// Services: both were Pending, should now be Done.
	for _, col := range []int{0, 1} {
		s, _ := snap.Get(2, col)
		if s != streak.Done {
			t.Errorf("Services col %d = %v, want Done", col, s)
		}
	}
	panel.Finish("done")
}

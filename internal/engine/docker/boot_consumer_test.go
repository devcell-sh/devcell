package docker_test

import (
	"strings"
	"testing"

	"github.com/DimmKirr/devcell/internal/engine/docker"
	"github.com/DimmKirr/devcell/internal/ux"
)

// CELL-264 Layer 3 — ConsumeBootEvents consumer goroutine tests.
//
// Mirrors the structure of the previous ConsumeNotifyEvents tests
// (CELL-263, now removed in Layer 8): under LogPlainText=true the spinner
// goroutine is disabled, so captured stdout deterministically reflects
// the permanent Success/Fail lines emitted by ProgressSpinner.

func TestConsumeBootEvents_StartingAndReadyMergeIntoOneRow(t *testing.T) {
	// A paired .starting + .ready for the same component must collapse to a
	// SINGLE permanent row whose title is the final ready-state label. The
	// in-flight title ("Loading mise tools") is shown while the spinner is
	// ticking, but the sealed row reflects only the completed state ("Mise
	// ready") — no redundant `✓ Mise ready 0s` follow-up. CELL-264 follow-up.
	defer withPlainText(t)()

	events := make(chan docker.BootEvent, 4)
	events <- docker.BootEvent{Component: "mise", State: "starting", Title: "Loading mise tools"}
	events <- docker.BootEvent{Component: "mise", State: "ready", Title: "Mise ready"}
	events <- docker.BootEvent{Component: "boot", State: "ready", Title: ""}
	close(events)

	out := captureStdout(t, func() {
		docker.ConsumeBootEvents(events)
	})

	if !strings.Contains(out, "Mise ready") {
		t.Errorf("sealed row should carry the ready-state title:\n%s", out)
	}
	if got := strings.Count(out, "✓"); got != 1 {
		t.Errorf("paired starting+ready must produce exactly 1 ✓ row, got %d:\n%s", got, out)
	}
}

func TestConsumeBootEvents_BootReadyStopsConsumer(t *testing.T) {
	defer withPlainText(t)()

	events := make(chan docker.BootEvent, 4)
	events <- docker.BootEvent{Component: "mise", State: "ready", Title: "Mise ready"}
	events <- docker.BootEvent{Component: "boot", State: "ready", Title: ""}
	// Anything after boot.ready must NOT render — consumer returns on the
	// seal event, even if more events are buffered behind it.
	events <- docker.BootEvent{Component: "gui", State: "ready", Title: "Should-not-appear"}
	close(events)

	out := captureStdout(t, func() {
		docker.ConsumeBootEvents(events)
	})

	if !strings.Contains(out, "Mise ready") {
		t.Errorf("missing pre-seal row:\n%s", out)
	}
	if strings.Contains(out, "Should-not-appear") {
		t.Errorf("post-boot.ready event must not render:\n%s", out)
	}
}

func TestConsumeBootEvents_FallsBackToComponentWhenNoTitle(t *testing.T) {
	// A fragment may emit a sentinel for a component not yet in the host's
	// title registry. Rather than drop the event, render the row using the
	// raw "<component> <state>" — the host should never silently miss
	// container progress just because the title map is out of sync.
	defer withPlainText(t)()

	events := make(chan docker.BootEvent, 2)
	events <- docker.BootEvent{Component: "newcomponent", State: "ready", Title: ""}
	events <- docker.BootEvent{Component: "boot", State: "ready", Title: ""}
	close(events)

	out := captureStdout(t, func() {
		docker.ConsumeBootEvents(events)
	})

	if !strings.Contains(out, "newcomponent") {
		t.Errorf("missing fallback row for untitled component:\n%s", out)
	}
}

// TestConsumeBootEvents_ChannelCloseDoesNotSealCurrent — when the events
// channel closes WITHOUT a preceding boot.ready, the agent's interactive
// process (zsh/claude) has typically taken over the TTY by then. Writing
// a ✓ row at this point would appear AFTER the shell prompt — visually
// confusing. CELL-264 contract: in-flight spinner is abandoned silently
// on channel close.
//
// The Loading mise tools spinner may still briefly write its frame to the
// captured buffer (the spinner goroutine ticks before the close arrives),
// but no permanent ✓ row should appear for it.
func TestConsumeBootEvents_ChannelCloseAbandonsCurrentSilently(t *testing.T) {
	defer withPlainText(t)()

	events := make(chan docker.BootEvent, 2)
	events <- docker.BootEvent{Component: "mise", State: "starting", Title: "Loading mise tools"}
	close(events) // simulates BootDirWatcher.Close() before boot.ready arrived

	out := captureStdout(t, func() {
		docker.ConsumeBootEvents(events)
	})

	// LogPlainText prints "→ Loading mise tools\n" on NewProgressSpinner;
	// it's the "spinner started" announcement. No ✓ for it because the
	// channel closed without boot.ready before the next event sealed it.
	if strings.Contains(out, "✓") {
		t.Errorf("channel close without boot.ready must NOT seal an in-flight row "+
			"(rows after this point would appear after the shell prompt); got:\n%s", out)
	}
}

func TestConsumeBootEvents_SoloReadyEventsRenderIndependently(t *testing.T) {
	// Some components (container, entrypoint) emit only a .ready sentinel
	// with no preceding .starting. Each must still produce its own ✓ row —
	// the merge-into-one-row optimization only applies to paired events.
	defer withPlainText(t)()

	events := make(chan docker.BootEvent, 4)
	events <- docker.BootEvent{Component: "container", State: "ready", Title: "Container started"}
	events <- docker.BootEvent{Component: "entrypoint", State: "ready", Title: "Entrypoint ready"}
	events <- docker.BootEvent{Component: "boot", State: "ready", Title: ""}
	close(events)

	out := captureStdout(t, func() {
		docker.ConsumeBootEvents(events)
	})

	if !strings.Contains(out, "Container started") {
		t.Errorf("missing solo container.ready row:\n%s", out)
	}
	if !strings.Contains(out, "Entrypoint ready") {
		t.Errorf("missing solo entrypoint.ready row:\n%s", out)
	}
	if c := strings.Count(out, "✓"); c != 2 {
		t.Errorf("two solo .ready events must produce 2 ✓ rows, got %d:\n%s", c, out)
	}
}

// Ensure withPlainText helper is reused from the existing test file
// (notify_consumer_test.go, removed in Layer 8 — we keep using the same
// helper pattern here). Same with captureStdout.
var _ = ux.LogPlainText // hint to keep ux import alive even if helper inlined

// The container shares the TTY with the consumer. Any animated row can be
// pushed off its saved cursor position by a foreign write that scrolls the
// screen, leaving a ghost `⠦ Starting GUI 1.3s` above the sealed ✓. So the
// consumer never animates: .starting emits nothing, .ready lands one row.
func TestConsumeBootEvents_StartingEmitsNoRow(t *testing.T) {
	defer withPlainText(t)()

	events := make(chan docker.BootEvent, 4)
	events <- docker.BootEvent{Component: "gui", State: "starting", Title: "Starting GUI"}
	events <- docker.BootEvent{Component: "gui", State: "ready", Title: "GUI ready"}
	events <- docker.BootEvent{Component: "boot", State: "ready"}
	close(events)

	out := captureStdout(t, func() { docker.ConsumeBootEvents(events) })

	if strings.Contains(out, "Starting GUI") {
		t.Errorf(".starting must not render a row of its own:\n%s", out)
	}
	if lines := strings.Count(strings.TrimSpace(out), "\n") + 1; lines != 1 {
		t.Errorf("want exactly one row, got %d:\n%s", lines, out)
	}
	if !strings.Contains(out, "✓") || !strings.Contains(out, "GUI ready") {
		t.Errorf("want a sealed ✓ GUI ready row:\n%s", out)
	}
}

// A `<component>.warn` sentinel carries a message in its file body; the
// consumer renders it as a ⚠ row without disturbing the phase in flight.
func TestConsumeBootEvents_WarnRendersWarningRow(t *testing.T) {
	defer withPlainText(t)()

	events := make(chan docker.BootEvent, 5)
	events <- docker.BootEvent{Component: "nix", State: "starting", Title: "Configuring nix"}
	events <- docker.BootEvent{Component: "gcroot", State: "warn", Detail: "Nix config drift: 3 variants"}
	events <- docker.BootEvent{Component: "nix", State: "ready", Title: "Nix ready"}
	events <- docker.BootEvent{Component: "boot", State: "ready"}
	close(events)

	out := captureStdout(t, func() { docker.ConsumeBootEvents(events) })

	if !strings.Contains(out, "⚠") || !strings.Contains(out, "Nix config drift: 3 variants") {
		t.Errorf("want a ⚠ row with the sentinel body:\n%s", out)
	}
	if got := strings.Count(out, "✓"); got != 1 {
		t.Errorf("warn must not seal or split the nix phase, want 1 ✓ row, got %d:\n%s", got, out)
	}
	if strings.Contains(out, "gcroot warn") {
		t.Errorf("warn must not fall back to the raw component row:\n%s", out)
	}
}

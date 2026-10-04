package docker

import (
	"time"

	"github.com/DimmKirr/devcell/internal/ux"
)

// ConsumeBootEvents reads BootDirWatcher events and renders each as a
// permanent ✓ row directly via ux.SuccessMsg. CELL-264.
//
// Wire format matches the host-side PhaseRunner (CELL-262) so the rendered
// checklist reads as one continuous boot story:
//
//	✓ Cell ready                           ← from PhaseRunner.Seal on the host
//	✓ Loading mise tools                   ← from sentinel mise.starting
//	✓ Mise ready                           ← from sentinel mise.ready
//	✓ GUI ready                            ← from sentinel gui.ready
//	                                        ← boot.ready stops the consumer; TTY handoff
//
// # Why no spinner
//
// The CELL-262 PhaseRunner uses ProgressSpinner for host-side phases
// because the host is actively *doing* work while the spinner ticks.
// Here, the events represent completed milestones from inside the
// container — by the time the host sees the sentinel file, the work is
// already done. A spinner adds no information AND introduces a race: the
// spinner's clear-on-stop can land on the wrong terminal row when the
// container's stdout writes concurrently to the same TTY (visible as
// a stuck `⠋ GUI ready 80ms` ghost row preceding the sealed ✓).
//
// Direct permanent-line emission avoids the race entirely. No spinner
// goroutine, no clear, no chance of fighting the container TTY.
//
// # Title lookup precedence
//
//  1. event.Title — set by BootDirWatcher from the host-side titles map
//  2. event.Component when Title is empty AND component != "boot" —
//     falls back so a fragment introducing a new component still produces
//     a visible row (just with the raw name) instead of being dropped.
//
// # boot.ready
//
// The terminal `boot.ready` event (sentinel emitted by entrypoint.sh
// after all fragments) carries Title="" by design — the consumer treats
// it as the seal trigger, NOT as a row to render. Returns immediately
// when seen.
func ConsumeBootEvents(events <-chan BootEvent) {
	// Per-row elapsed is measured here: a phase opens on its `.starting`
	// (or on a solo `.ready` such as container.ready) and seals on the
	// next event for the same component, or on whatever event comes next.
	//
	// No spinner, on purpose (see the header comment): the container
	// writes to the same TTY, and a foreign newline at the bottom of the
	// screen scrolls the spinner's saved cursor row away, so its clear
	// misses and a ghost `⠦ Starting GUI 1.3s` survives above the ✓.
	phaseStart := time.Now()
	inFlight := false
	var currentComponent string
	var currentTitle string

	sealCurrent := func(finalTitle string) {
		if !inFlight {
			return
		}
		ux.SuccessRow(finalTitle, time.Since(phaseStart))
		inFlight = false
		currentComponent = ""
		currentTitle = ""
	}

	for ev := range events {
		// Terminal seal — boot.ready means entrypoint finished sourcing
		// all fragments and is about to exec the child binary. Stop
		// rendering immediately so we don't fight claude/zsh for the TTY.
		if ev.Component == "boot" && ev.State == "ready" {
			sealCurrent(currentTitle)
			return
		}

		// `.warn` is a message, not a phase: render it in place and leave
		// whatever is in flight untouched.
		if ev.State == "warn" {
			msg := ev.Detail
			if msg == "" {
				msg = ev.Title
			}
			if msg == "" {
				msg = ev.Component + " warned"
			}
			ux.Warn(msg)
			continue
		}

		title := ev.Title
		if title == "" {
			// Unknown component: fall back to the raw name so the user
			// still sees that *something* happened in the container.
			title = ev.Component + " " + ev.State
		}

		// Paired .ready: a phase is open for THIS component (opened by its
		// .starting). Seal it with the ready-state title — one row.
		if ev.State == "ready" && inFlight && ev.Component == currentComponent {
			sealCurrent(title)
			continue
		}

		// New phase (either a .starting or a solo .ready like
		// container.ready / entrypoint.ready). Seal whatever was in flight
		// with its own current title, then open a fresh one. For solo
		// .ready events the next event seals it — preserving the visible
		// timing for the container/entrypoint boot rows.
		sealCurrent(currentTitle)
		currentComponent = ev.Component
		currentTitle = title
		phaseStart = time.Now()
		inFlight = true
	}

	// Channel closed without boot.ready (BootDirWatcher.Close, typically
	// because the container exited). The agent process has the TTY by
	// now; do NOT seal — writing more rows after the shell prompt would
	// be visually confusing.
}

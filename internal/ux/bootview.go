package ux

import (
	"fmt"
	"sync"

	streak "github.com/dimmkirr/go-streak-chart"
)

// BootView is everything the engines call on the boot panel. BootPanel
// draws it on the tty; EventPanel records it as BootEvents for a renderer
// in another process (CELL-657: the cage's tmux popup).
type BootView interface {
	Phaser
	Message(text string, s streak.Status)
	SetBoot(group, step string, s streak.Status, title string)
	WarnBoot(msg string)
	PromotePending()
	PromoteRunning()
	Finish(text string)
	FinishError(text string)
	ClearBelowGroups()
}

var (
	_ BootView = (*BootPanel)(nil)
	_ BootView = (*EventPanel)(nil)
)

// BootEvent is one panel mutation on the wire (JSON lines). Kind selects
// which fields matter; Status is a streak.Status name (see ParseStatus).
type BootEvent struct {
	Kind   string       `json:"kind"`
	Group  string       `json:"group,omitempty"`
	Step   string       `json:"step,omitempty"`
	Text   string       `json:"text,omitempty"`
	Detail string       `json:"detail,omitempty"`
	Status string       `json:"status,omitempty"`
	Groups []PanelGroup `json:"groups,omitempty"`
}

// Event kinds. EventGroups is always first: it carries the panel layout.
const (
	EventGroups         = "groups"
	EventRunning        = "running" // Step, Text = running label
	EventDone           = "done"    // Step, Detail
	EventFail           = "fail"    // Step, Text = error
	EventWarn           = "warn"    // Step, Detail
	EventSet            = "set"     // Group, Step, Status, Text = title
	EventMessage        = "message" // Text, Status (footer)
	EventPromotePending = "promote_pending"
	EventPromoteRunning = "promote_running"
	EventFinish         = "finish"       // Text
	EventFinishError    = "finish_error" // Text
	EventClearBelow     = "clear_below"
)

// ParseStatus maps a streak.Status name back to the value.
func ParseStatus(s string) (streak.Status, bool) {
	for _, st := range []streak.Status{streak.Pending, streak.Running, streak.Done, streak.Warning, streak.Error} {
		if st.String() == s {
			return st, true
		}
	}
	return 0, false
}

// Apply replays one event onto the panel. Unknown kinds are an error so a
// protocol mismatch between devcell versions shows up instead of leaving a
// dot grey.
func (p *BootPanel) Apply(ev BootEvent) error {
	switch ev.Kind {
	case EventGroups:
		// Layout is fixed at construction; the renderer built the panel
		// from this event already.
	case EventRunning:
		p.setRunning(ev.Step, ev.Text)
	case EventDone:
		p.setDone(ev.Step, ev.Detail)
	case EventFail:
		p.setFail(ev.Step, ev.Text)
	case EventWarn:
		p.setWarn(ev.Step, ev.Detail)
	case EventSet:
		s, ok := ParseStatus(ev.Status)
		if !ok {
			return fmt.Errorf("boot event %s: unknown status %q", ev.Kind, ev.Status)
		}
		p.SetBoot(ev.Group, ev.Step, s, ev.Text)
	case EventMessage:
		s, ok := ParseStatus(ev.Status)
		if !ok {
			return fmt.Errorf("boot event %s: unknown status %q", ev.Kind, ev.Status)
		}
		p.Message(ev.Text, s)
	case EventPromotePending:
		p.PromotePending()
	case EventPromoteRunning:
		p.PromoteRunning()
	case EventFinish:
		p.Finish(ev.Text)
	case EventFinishError:
		p.FinishError(ev.Text)
	case EventClearBelow:
		// Cursor gymnastics for the inline layout; a popup has nothing
		// below the panel to clear.
	default:
		return fmt.Errorf("boot event: unknown kind %q", ev.Kind)
	}
	return nil
}

// EventSink receives BootEvents in order. BootPanel is one (local replay);
// the cage adds a JSON-lines writer over a socket.
type EventSink interface {
	Apply(BootEvent) error
}

// EventPanel is a BootView that records every mutation and forwards it to
// at most one sink. Events emitted before Bind are buffered and replayed on
// Bind, so a renderer that attaches late sees the whole history. Phase*
// run their callbacks exactly like BootPanel does.
type EventPanel struct {
	mu     sync.Mutex
	events []BootEvent
	sink   EventSink
	err    error
}

// NewEventPanel starts the event log with the layout event.
func NewEventPanel(groups []PanelGroup) *EventPanel {
	p := &EventPanel{}
	p.emit(BootEvent{Kind: EventGroups, Groups: groups})
	return p
}

// Bind replays the buffered events into s, then streams new ones to it.
// Any previously bound sink stops receiving events.
func (p *EventPanel) Bind(s EventSink) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ev := range p.events {
		if err := s.Apply(ev); err != nil {
			return err
		}
	}
	p.sink = s
	return nil
}

// Unbind detaches the current sink; events keep buffering.
func (p *EventPanel) Unbind() {
	p.mu.Lock()
	p.sink = nil
	p.mu.Unlock()
}

// Events returns a copy of everything emitted so far.
func (p *EventPanel) Events() []BootEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]BootEvent, len(p.events))
	copy(out, p.events)
	return out
}

// Finished reports whether a finish or finish_error event was emitted.
func (p *EventPanel) Finished() (finished bool, failed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ev := range p.events {
		switch ev.Kind {
		case EventFinish:
			return true, false
		case EventFinishError:
			return true, true
		}
	}
	return false, false
}

// SinkErr returns the first error a sink reported, if any.
func (p *EventPanel) SinkErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *EventPanel) emit(ev BootEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
	if p.sink != nil {
		if err := p.sink.Apply(ev); err != nil && p.err == nil {
			p.err = err
		}
	}
}

func (p *EventPanel) Phase(name string, fn func() error) error {
	p.emit(BootEvent{Kind: EventRunning, Step: name, Text: name})
	if err := fn(); err != nil {
		p.emit(BootEvent{Kind: EventFail, Step: name, Text: err.Error()})
		return err
	}
	p.emit(BootEvent{Kind: EventDone, Step: name})
	return nil
}

func (p *EventPanel) PhaseDetailed(name string, fn func() (string, error)) error {
	return p.PhaseDetailedRunning(name, name, fn)
}

func (p *EventPanel) PhaseDetailedWarn(name string, fn func() (string, bool, error)) error {
	return p.PhaseDetailedRunningWarn(name, name, fn)
}

func (p *EventPanel) PhaseDetailedRunning(running, finalName string, fn func() (string, error)) error {
	p.emit(BootEvent{Kind: EventRunning, Step: finalName, Text: running})
	detail, err := fn()
	if err != nil {
		p.emit(BootEvent{Kind: EventFail, Step: finalName, Text: err.Error()})
		return err
	}
	p.emit(BootEvent{Kind: EventDone, Step: finalName, Detail: detail})
	return nil
}

func (p *EventPanel) PhaseDetailedRunningWarn(running, finalName string, fn func() (string, bool, error)) error {
	p.emit(BootEvent{Kind: EventRunning, Step: finalName, Text: running})
	detail, warn, err := fn()
	if err != nil {
		p.emit(BootEvent{Kind: EventFail, Step: finalName, Text: err.Error()})
		return err
	}
	if warn {
		p.emit(BootEvent{Kind: EventWarn, Step: finalName, Detail: detail})
	} else {
		p.emit(BootEvent{Kind: EventDone, Step: finalName, Detail: detail})
	}
	return nil
}

func (p *EventPanel) UpdateText(message string) {
	p.emit(BootEvent{Kind: EventMessage, Text: message, Status: streak.Running.String()})
}

func (p *EventPanel) Seal(name string) { p.emit(BootEvent{Kind: EventDone, Step: name}) }

func (p *EventPanel) Message(text string, s streak.Status) {
	p.emit(BootEvent{Kind: EventMessage, Text: text, Status: s.String()})
}

func (p *EventPanel) SetBoot(group, step string, s streak.Status, title string) {
	p.emit(BootEvent{Kind: EventSet, Group: group, Step: step, Status: s.String(), Text: title})
}

func (p *EventPanel) WarnBoot(msg string) {
	p.emit(BootEvent{Kind: EventMessage, Text: msg, Status: streak.Warning.String()})
}

func (p *EventPanel) PromotePending()         { p.emit(BootEvent{Kind: EventPromotePending}) }
func (p *EventPanel) PromoteRunning()         { p.emit(BootEvent{Kind: EventPromoteRunning}) }
func (p *EventPanel) Finish(text string)      { p.emit(BootEvent{Kind: EventFinish, Text: text}) }
func (p *EventPanel) FinishError(text string) { p.emit(BootEvent{Kind: EventFinishError, Text: text}) }
func (p *EventPanel) ClearBelowGroups()       { p.emit(BootEvent{Kind: EventClearBelow}) }

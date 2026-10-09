package docker

import (
	"strings"
	"testing"
	"time"
)

func TestWatchReader_ParsesJSONL(t *testing.T) {
	input := strings.NewReader(
		`{"ts":"2026-10-08T14:30:00Z","scope":"service","service":"shell-rc","status":"activating"}
{"ts":"2026-10-08T14:30:01Z","scope":"service","service":"shell-rc","status":"up","msg":"Shell ready"}
{"ts":"2026-10-08T14:30:02Z","scope":"system","service":"boot","status":"ready"}
`)
	out := make(chan ContainerEvent, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchReader(input, out)
		close(out)
	}()
	<-done

	events := drain(out)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Scope != "service" || events[0].Service != "shell-rc" || events[0].Status != "activating" {
		t.Errorf("event[0] = %+v", events[0])
	}
	if events[1].Status != "up" || events[1].Msg != "Shell ready" {
		t.Errorf("event[1] = %+v", events[1])
	}
	if events[2].Scope != "system" || events[2].Service != "boot" || events[2].Status != "ready" {
		t.Errorf("event[2] = %+v", events[2])
	}
}

func TestWatchReader_SkipsNonJSON(t *testing.T) {
	input := strings.NewReader(
		`some random log line
not json at all
{"ts":"2026-10-08T14:30:00Z","scope":"service","service":"mise","status":"up"}
another non-json line
`)
	out := make(chan ContainerEvent, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchReader(input, out)
		close(out)
	}()
	<-done

	events := drain(out)
	if len(events) != 1 {
		t.Fatalf("expected 1 event (non-JSON skipped), got %d", len(events))
	}
	if events[0].Service != "mise" {
		t.Errorf("event service = %q, want mise", events[0].Service)
	}
}

func TestWatchReader_SkipsEmptyScope(t *testing.T) {
	input := strings.NewReader(
		`{"ts":"2026-10-08T14:30:00Z","service":"test"}
{"ts":"2026-10-08T14:30:01Z","scope":"service","service":"test","status":"up"}
`)
	out := make(chan ContainerEvent, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchReader(input, out)
		close(out)
	}()
	<-done

	events := drain(out)
	if len(events) != 1 {
		t.Fatalf("expected 1 event (empty scope skipped), got %d", len(events))
	}
}

func TestWatchReader_ParsesTimestamp(t *testing.T) {
	input := strings.NewReader(
		`{"ts":"2026-10-08T14:30:00Z","scope":"service","service":"test","status":"up"}
`)
	out := make(chan ContainerEvent, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchReader(input, out)
		close(out)
	}()
	<-done

	events := drain(out)
	if len(events) != 1 {
		t.Fatal("expected 1 event")
	}
	want := time.Date(2026, 10, 8, 14, 30, 0, 0, time.UTC)
	if !events[0].TS.Equal(want) {
		t.Errorf("TS = %v, want %v", events[0].TS, want)
	}
}

func TestNewContainerLogWatcherFromReader(t *testing.T) {
	input := strings.NewReader(
		`{"ts":"2026-10-08T14:30:00Z","scope":"service","service":"mise","status":"up"}
`)
	w := newContainerLogWatcherFromReader(input)
	defer w.Close()

	select {
	case ev, ok := <-w.Events():
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if ev.Service != "mise" || ev.Status != "up" {
			t.Errorf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func drain(ch <-chan ContainerEvent) []ContainerEvent {
	var result []ContainerEvent
	for ev := range ch {
		result = append(result, ev)
	}
	return result
}

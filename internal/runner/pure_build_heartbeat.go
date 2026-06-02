package runner

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// heartbeatWriter wraps an io.Writer and emits a "still working" status line
// to that writer whenever no upstream writes have happened within `idle`.
//
// Rationale: a cold `cell build --pure --debug` spends 3-5 min in nix's
// eval + cache-substitutability phase with zero log output, even with -L -vv.
// The heartbeat makes that period observable so users can tell "still working"
// from "wedged" without having to ssh into the linux-builder.
type heartbeatWriter struct {
	inner    io.Writer
	idle     time.Duration
	now      func() time.Time
	start    time.Time
	stop     chan struct{}
	closed   bool
	mu       sync.Mutex
	lastSeen time.Time
}

func newHeartbeatWriter(inner io.Writer, idle time.Duration, now func() time.Time) *heartbeatWriter {
	if now == nil {
		now = time.Now
	}
	t := now()
	hb := &heartbeatWriter{
		inner:    inner,
		idle:     idle,
		now:      now,
		start:    t,
		lastSeen: t,
		stop:     make(chan struct{}),
	}
	go hb.tick()
	return hb
}

// Write forwards to inner and resets the silence timer.
func (h *heartbeatWriter) Write(p []byte) (int, error) {
	h.mu.Lock()
	h.lastSeen = h.now()
	h.mu.Unlock()
	return h.inner.Write(p)
}

// Close stops the heartbeat goroutine. Safe to call repeatedly.
func (h *heartbeatWriter) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	close(h.stop)
	return nil
}

func (h *heartbeatWriter) tick() {
	// Poll at idle/4 so a heartbeat fires within ~25% of `idle` after silence.
	poll := h.idle / 4
	if poll < 10*time.Millisecond {
		poll = 10 * time.Millisecond
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-t.C:
			h.mu.Lock()
			silence := h.now().Sub(h.lastSeen)
			if silence >= h.idle {
				elapsed := h.now().Sub(h.start).Truncate(time.Second)
				// Reset lastSeen so the next heartbeat fires after another
				// full `idle` of silence (not every tick).
				h.lastSeen = h.now()
				h.mu.Unlock()
				_, _ = fmt.Fprintf(h.inner,
					"     … still working (eval/cache-query phase is silent, elapsed %s)\n",
					elapsed)
				continue
			}
			h.mu.Unlock()
		}
	}
}

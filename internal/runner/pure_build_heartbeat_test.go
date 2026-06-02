package runner

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// Heartbeat writer wraps an io.Writer and emits a "still working" line
// to the underlying writer whenever no upstream writes have arrived within
// the configured idle window. Stops on Close.
//
// User-visible behavior:
//
//	→ Building devcell image (nix2container, stack=ultimate)
//	copying path '...' from 'https://cache.nixos.org'...
//	[1/0/1 copied (187.6/187.6 MiB), 32.3 MiB DL] fetching source from https://cache.nixos.org
//	     … still working (eval/cache-query phase is silent, elapsed 30s)
//	     … still working (eval/cache-query phase is silent, elapsed 60s)
//	these 12 derivations will be built:
//
// Spec:
//   - First heartbeat fires only after `idle` of silence from start (not at t=0).
//   - Subsequent heartbeats fire every `idle` while still silent.
//   - Any Write call resets the silence timer.
//   - Close() stops the goroutine and is safe to call multiple times.

func TestNewHeartbeatWriter_FiresWhenSilent(t *testing.T) {
	var buf safeBuffer
	hb := newHeartbeatWriter(&buf, 50*time.Millisecond, fakeClock())
	defer hb.Close()

	// Don't write anything. Wait long enough for ~3 heartbeats.
	time.Sleep(180 * time.Millisecond)

	got := buf.String()
	count := strings.Count(got, "still working")
	if count < 2 {
		t.Errorf("expected ≥2 heartbeats in 180ms with 50ms interval, got %d:\n%s", count, got)
	}
}

func TestNewHeartbeatWriter_ResetsOnWrite(t *testing.T) {
	var buf safeBuffer
	hb := newHeartbeatWriter(&buf, 80*time.Millisecond, fakeClock())
	defer hb.Close()

	// Keep writing every 30ms — never enough silence to fire a heartbeat.
	for range 6 {
		time.Sleep(30 * time.Millisecond)
		_, _ = hb.Write([]byte("nix output line\n"))
	}

	got := buf.String()
	if strings.Contains(got, "still working") {
		t.Errorf("heartbeat fired despite continuous writes:\n%s", got)
	}
	if !strings.Contains(got, "nix output line") {
		t.Errorf("upstream writes lost:\n%s", got)
	}
}

func TestNewHeartbeatWriter_StopsOnClose(t *testing.T) {
	var buf safeBuffer
	hb := newHeartbeatWriter(&buf, 30*time.Millisecond, fakeClock())

	// Let one heartbeat fire, then close.
	time.Sleep(50 * time.Millisecond)
	hb.Close()
	mark := buf.Len()
	time.Sleep(100 * time.Millisecond)
	if buf.Len() != mark {
		t.Errorf("heartbeat kept firing after Close():\nbefore=%d after=%d delta=%q",
			mark, buf.Len(), buf.String()[mark:])
	}
}

func TestNewHeartbeatWriter_DoubleCloseSafe(t *testing.T) {
	var buf safeBuffer
	hb := newHeartbeatWriter(&buf, 30*time.Millisecond, fakeClock())
	hb.Close()
	hb.Close() // must not panic or deadlock
}

// safeBuffer is bytes.Buffer + a mutex so reads/writes from the heartbeat
// goroutine and the test don't race under -race.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
func (b *safeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

// fakeClock returns time.Now — the heartbeat is real-clock based, but tests
// keep idle intervals tiny (30-80ms) so they finish fast. Wrapped in a helper
// in case we want to inject a controllable clock later.
func fakeClock() func() time.Time { return time.Now }

// Compile-time check: heartbeatWriter satisfies io.WriteCloser.
var _ io.WriteCloser = (*heartbeatWriter)(nil)

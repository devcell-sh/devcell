package serve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeStreamingStubAgent writes a shell script at <dir>/claude that emits
// the given JSONL lines (one per element) on stdout. Used to exercise
// streamClaude end-to-end without spawning the real claude binary.
func makeStreamingStubAgent(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	for _, ln := range lines {
		// Use single-quoted printf with explicit %s\n; JSON has no
		// embedded single quotes so this is safe.
		b.WriteString("printf '%s\\n' '")
		b.WriteString(ln)
		b.WriteString("'\n")
	}
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return dir
}

func TestStreamClaude_EndToEnd(t *testing.T) {
	lines := []string{
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"stream_event","event":{"type":"message_start","message":{"id":"msg_1","model":"claude-opus-4-7","role":"assistant"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" there"}}}`,
		`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"}}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"Hi there","usage":{"input_tokens":3,"output_tokens":2}}`,
	}
	dir := makeStreamingStubAgent(t, lines)
	withPath(t, dir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := streamClaude(ctx, ExecOpts{Agent: "claude", Prompt: "hi"})
	if err != nil {
		t.Fatalf("streamClaude: %v", err)
	}

	var (
		gotStart  bool
		gotDeltas []string
		gotResult bool
	)
	for ev := range ch {
		switch ev.Kind {
		case StreamEventMessageStart:
			gotStart = true
		case StreamEventTextDelta:
			gotDeltas = append(gotDeltas, ev.Delta)
		case StreamEventResult:
			gotResult = true
		case StreamEventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}
	if !gotStart {
		t.Error("MessageStart not observed")
	}
	if got := strings.Join(gotDeltas, ""); got != "Hi there" {
		t.Errorf("deltas concatenated = %q, want %q", got, "Hi there")
	}
	if !gotResult {
		t.Error("Result not observed")
	}
}

func TestStreamClaude_OpencodeRejected(t *testing.T) {
	_, err := streamClaude(context.Background(), ExecOpts{Agent: "opencode", Prompt: "hi"})
	if err == nil {
		t.Fatal("opencode should be rejected by streamClaude")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("error should mention claude-only support: %v", err)
	}
}

func TestStreamClaude_ContextCancelStopsClaude(t *testing.T) {
	// Stub that sleeps forever — cancelling ctx must kill it. Using
	// `sleep 60` keeps it simple; the test fails on the 5s timeout if
	// the process isn't killed promptly.
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 60\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	withPath(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := streamClaude(ctx, ExecOpts{Agent: "claude", Prompt: "hi"})
	if err != nil {
		t.Fatalf("streamClaude: %v", err)
	}
	// Cancel almost immediately and verify the channel closes promptly
	// (stub would otherwise sleep 60s).
	cancel()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case _, open := <-ch:
			if !open {
				return // channel closed → process was killed
			}
		case <-deadline.C:
			t.Fatal("channel still open 5s after cancel — claude not killed")
		}
	}
}

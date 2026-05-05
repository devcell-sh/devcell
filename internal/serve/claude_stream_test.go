package serve

import (
	"strings"
	"testing"
)

// realStreamCapture is a verbatim claude --output-format=stream-json
// --include-partial-messages capture (2026-04-28, claude-opus-4-7) for
// the prompt "Say 'one two three four five' on five separate lines".
// Used as the canonical fixture so the scanner is exercised against
// real wire bytes.
const realStreamCapture = `{"type":"system","subtype":"init","cwd":"/devcell-186","session_id":"ac12982f","tools":[]}
{"type":"stream_event","event":{"type":"message_start","message":{"model":"claude-opus-4-7","id":"msg_017bhZn5","type":"message","role":"assistant","content":[],"stop_reason":null,"usage":{"input_tokens":5,"cache_creation_input_tokens":30957,"cache_read_input_tokens":27019,"output_tokens":1}}},"session_id":"ac12982f"}
{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}},"session_id":"ac12982f"}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"one"}},"session_id":"ac12982f"}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"\ntwo\nthree\nfour\nfive"}},"session_id":"ac12982f"}
{"type":"assistant","message":{"model":"claude-opus-4-7","id":"msg_017bhZn5","content":[{"type":"text","text":"one\ntwo\nthree\nfour\nfive"}]}}
{"type":"stream_event","event":{"type":"content_block_stop","index":0},"session_id":"ac12982f"}
{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":13}},"session_id":"ac12982f"}
{"type":"stream_event","event":{"type":"message_stop"},"session_id":"ac12982f"}
{"type":"result","subtype":"success","is_error":false,"result":"one\ntwo\nthree\nfour\nfive","stop_reason":"end_turn","session_id":"ac12982f","total_cost_usd":0.001,"usage":{"input_tokens":5,"cache_creation_input_tokens":30957,"cache_read_input_tokens":27019,"output_tokens":13}}
`

func TestScanClaudeStream_RealCapture(t *testing.T) {
	ch := scanClaudeStream(strings.NewReader(realStreamCapture))

	var (
		gotStart   *StreamEvent
		gotDeltas  []string
		gotStop    *StreamEvent
		gotResult  *StreamEvent
		gotErr     error
		eventCount int
	)
	for ev := range ch {
		eventCount++
		switch ev.Kind {
		case StreamEventMessageStart:
			e := ev
			gotStart = &e
		case StreamEventTextDelta:
			gotDeltas = append(gotDeltas, ev.Delta)
		case StreamEventMessageStop:
			e := ev
			gotStop = &e
		case StreamEventResult:
			e := ev
			gotResult = &e
		case StreamEventError:
			gotErr = ev.Err
		}
	}

	if gotErr != nil {
		t.Fatalf("scanner emitted error: %v", gotErr)
	}
	if gotStart == nil || gotStart.MessageID != "msg_017bhZn5" || gotStart.Model != "claude-opus-4-7" {
		t.Errorf("MessageStart event missing or wrong: %+v", gotStart)
	}
	if got, want := strings.Join(gotDeltas, ""), "one\ntwo\nthree\nfour\nfive"; got != want {
		t.Errorf("text deltas concatenated = %q, want %q", got, want)
	}
	if gotStop == nil || gotStop.StopReason != "end_turn" {
		t.Errorf("MessageStop event missing or wrong: %+v", gotStop)
	}
	if gotResult == nil || gotResult.Final == nil {
		t.Fatal("Result event missing")
	}
	if gotResult.Final.Result != "one\ntwo\nthree\nfour\nfive" {
		t.Errorf("Result.Final.Result = %q", gotResult.Final.Result)
	}
	if gotResult.Final.Usage.OutputTokens != 13 {
		t.Errorf("Result.Final.Usage.OutputTokens = %d, want 13", gotResult.Final.Usage.OutputTokens)
	}

	// Skipped types (system, assistant full-turn, content_block_*,
	// message_stop) must not emit canonical events.
	wantEvents := 1 /*start*/ + len(gotDeltas) + 1 /*stop*/ + 1 /*result*/
	if eventCount != wantEvents {
		t.Errorf("emitted %d canonical events, want %d (skipped types leaked through?)", eventCount, wantEvents)
	}
}

func TestScanClaudeStream_IgnoresGarbageLines(t *testing.T) {
	// Random non-JSON noise (could happen if stderr leaks into stdout)
	// must be silently dropped, not turned into Error events.
	input := "not json\n" +
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}}` + "\n"
	var deltas []string
	for ev := range scanClaudeStream(strings.NewReader(input)) {
		if ev.Kind == StreamEventTextDelta {
			deltas = append(deltas, ev.Delta)
		}
		if ev.Kind == StreamEventError {
			t.Fatalf("garbage line produced an Error event: %v", ev.Err)
		}
	}
	if len(deltas) != 1 || deltas[0] != "hi" {
		t.Errorf("deltas = %v, want [hi]", deltas)
	}
}

func TestScanClaudeStream_OnlyTextDeltasForwarded(t *testing.T) {
	// tool_use deltas (out of scope for v1) must not leak as text.
	input := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"foo\":"}}}` + "\n"
	for ev := range scanClaudeStream(strings.NewReader(input)) {
		if ev.Kind == StreamEventTextDelta {
			t.Fatalf("non-text_delta leaked as TextDelta: %+v", ev)
		}
	}
}

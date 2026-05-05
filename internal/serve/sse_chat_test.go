package serve

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

// pumpChatSSEHelper feeds canonical events through pumpChatSSE and returns
// the rendered SSE body as a string. Heartbeats disabled so tests are
// deterministic.
func pumpChatSSEHelper(t *testing.T, events []StreamEvent, includeUsage bool) string {
	t.Helper()
	rec := httptest.NewRecorder()
	ch := make(chan StreamEvent, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	pumpChatSSE(context.Background(), rec, rec, ch, "chatcmpl-test", "anthropic/sonnet", includeUsage, 0)
	return rec.Body.String()
}

func TestPumpChatSSE_HappyPath(t *testing.T) {
	events := []StreamEvent{
		{Kind: StreamEventMessageStart, MessageID: "msg_1", Model: "claude-opus-4-7"},
		{Kind: StreamEventTextDelta, Delta: "Hi"},
		{Kind: StreamEventTextDelta, Delta: ", "},
		{Kind: StreamEventTextDelta, Delta: "world"},
		{Kind: StreamEventMessageStop, StopReason: "end_turn"},
		{Kind: StreamEventResult, Final: &claudeJSONResult{
			Type: "result", Subtype: "success", Result: "Hi, world",
			Usage: claudeJSONUsageObj{InputTokens: 5, OutputTokens: 3, CacheCreationInputTokens: 100},
		}},
	}
	body := pumpChatSSEHelper(t, events, false)

	// Frame ordering check.
	checks := []string{
		`"delta":{"role":"assistant"}`,
		`"delta":{"content":"Hi"}`,
		`"delta":{"content":", "}`,
		`"delta":{"content":"world"}`,
		`"finish_reason":"stop"`,
		"data: [DONE]",
	}
	prev := -1
	for _, c := range checks {
		idx := strings.Index(body, c)
		if idx < 0 {
			t.Errorf("missing frame fragment %q\n\nfull body:\n%s", c, body)
			continue
		}
		if idx <= prev {
			t.Errorf("fragment %q appeared out of order (idx=%d, prev=%d)\n\nfull body:\n%s", c, idx, prev, body)
		}
		prev = idx
	}
	// Without include_usage, the final chunk must NOT carry usage.
	if strings.Contains(body, `"usage":`) {
		t.Errorf("usage leaked when stream_options.include_usage=false:\n%s", body)
	}
}

func TestPumpChatSSE_IncludeUsage(t *testing.T) {
	events := []StreamEvent{
		{Kind: StreamEventMessageStart, MessageID: "msg_1", Model: "x"},
		{Kind: StreamEventTextDelta, Delta: "ok"},
		{Kind: StreamEventResult, Final: &claudeJSONResult{
			Type: "result", Result: "ok",
			Usage: claudeJSONUsageObj{InputTokens: 5, OutputTokens: 2, CacheReadInputTokens: 100},
		}},
	}
	body := pumpChatSSEHelper(t, events, true)

	if !strings.Contains(body, `"prompt_tokens":105`) {
		t.Errorf("prompt_tokens should be 5+0+100=105 (input+cache_creation+cache_read), got body:\n%s", body)
	}
	if !strings.Contains(body, `"completion_tokens":2`) {
		t.Errorf("completion_tokens should be 2:\n%s", body)
	}
	if !strings.Contains(body, `"total_tokens":107`) {
		t.Errorf("total_tokens should be 107:\n%s", body)
	}
}

func TestPumpChatSSE_DeltaSentBeforeRoleFallsBackGracefully(t *testing.T) {
	// Defensive path: streams that skip message_start should still get
	// a synthetic role chunk before the first content delta.
	events := []StreamEvent{
		{Kind: StreamEventTextDelta, Delta: "x"},
		{Kind: StreamEventResult, Final: &claudeJSONResult{Type: "result"}},
	}
	body := pumpChatSSEHelper(t, events, false)

	rolePos := strings.Index(body, `"role":"assistant"`)
	contentPos := strings.Index(body, `"content":"x"`)
	if rolePos < 0 || contentPos < 0 || rolePos > contentPos {
		t.Errorf("role chunk must precede first content delta\n%s", body)
	}
}

func TestPumpChatSSE_ChannelClosedWithoutResultStillTerminates(t *testing.T) {
	// Claude crash mid-stream — channel closes with no Result. Client
	// must still see a terminal chunk + [DONE] so it doesn't hang.
	events := []StreamEvent{
		{Kind: StreamEventMessageStart, MessageID: "x"},
		{Kind: StreamEventTextDelta, Delta: "partial"},
	}
	body := pumpChatSSEHelper(t, events, false)
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("missing terminator on premature channel close:\n%s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Errorf("missing finish_reason on premature close:\n%s", body)
	}
}

func TestPumpChatSSE_ResultWithIsErrorSendsFinishReasonError(t *testing.T) {
	events := []StreamEvent{
		{Kind: StreamEventMessageStart},
		{Kind: StreamEventResult, Final: &claudeJSONResult{Type: "result", IsError: true, Subtype: "error_during_execution"}},
	}
	body := pumpChatSSEHelper(t, events, false)
	if !strings.Contains(body, `"finish_reason":"error"`) {
		t.Errorf("missing finish_reason=error:\n%s", body)
	}
}

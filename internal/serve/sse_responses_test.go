package serve

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func pumpResponsesSSEHelper(t *testing.T, events []StreamEvent) string {
	t.Helper()
	rec := httptest.NewRecorder()
	ch := make(chan StreamEvent, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	pumpResponsesSSE(context.Background(), rec, rec, ch, "resp_test", "anthropic/sonnet", nil, 0)
	return rec.Body.String()
}

func TestPumpResponsesSSE_HappyPathFrameOrder(t *testing.T) {
	events := []StreamEvent{
		{Kind: StreamEventMessageStart, MessageID: "msg_1"},
		{Kind: StreamEventTextDelta, Delta: "Hi"},
		{Kind: StreamEventTextDelta, Delta: ", world"},
		{Kind: StreamEventResult, Final: &claudeJSONResult{
			Type: "result", Subtype: "success", Result: "Hi, world",
			Usage: claudeJSONUsageObj{InputTokens: 5, OutputTokens: 4, CacheReadInputTokens: 100},
		}},
	}
	body := pumpResponsesSSEHelper(t, events)

	expected := []string{
		"event: response.created",
		"event: response.in_progress",
		"event: response.output_item.added",
		"event: response.content_part.added",
		`event: response.output_text.delta`,
		`"delta":"Hi"`,
		`"delta":", world"`,
		"event: response.output_text.done",
		`"text":"Hi, world"`,
		"event: response.content_part.done",
		"event: response.output_item.done",
		"event: response.completed",
	}
	prev := -1
	for _, want := range expected {
		idx := strings.Index(body, want)
		if idx < 0 {
			t.Errorf("missing event %q\n\n%s", want, body)
			continue
		}
		if idx <= prev {
			t.Errorf("event %q out of order (idx=%d, prev=%d)\n\n%s", want, idx, prev, body)
		}
		prev = idx
	}

	// No `[DONE]` sentinel — Responses uses response.completed as terminator.
	if strings.Contains(body, "[DONE]") {
		t.Errorf("Responses SSE must NOT include [DONE] sentinel:\n%s", body)
	}
}

func TestPumpResponsesSSE_UsageOnCompletedFrame(t *testing.T) {
	events := []StreamEvent{
		{Kind: StreamEventMessageStart},
		{Kind: StreamEventTextDelta, Delta: "ok"},
		{Kind: StreamEventResult, Final: &claudeJSONResult{
			Type: "result", Result: "ok",
			Usage: claudeJSONUsageObj{InputTokens: 5, OutputTokens: 2, CacheCreationInputTokens: 50, CacheReadInputTokens: 100},
		}},
	}
	body := pumpResponsesSSEHelper(t, events)

	// input_tokens flattens (input + cache_creation + cache_read) = 155
	if !strings.Contains(body, `"input_tokens":155`) {
		t.Errorf("input_tokens should be 5+50+100=155 (flattened):\n%s", body)
	}
	if !strings.Contains(body, `"output_tokens":2`) {
		t.Errorf("output_tokens=2 missing:\n%s", body)
	}
	if !strings.Contains(body, `"cached_tokens":100`) {
		t.Errorf("cached_tokens detail (cache_read=100) missing:\n%s", body)
	}
	if !strings.Contains(body, `"total_tokens":157`) {
		t.Errorf("total_tokens should be 157:\n%s", body)
	}
}

func TestPumpResponsesSSE_ResultIsErrorEmitsFailed(t *testing.T) {
	events := []StreamEvent{
		{Kind: StreamEventMessageStart},
		{Kind: StreamEventResult, Final: &claudeJSONResult{
			Type: "result", IsError: true, Subtype: "error_during_execution",
		}},
	}
	body := pumpResponsesSSEHelper(t, events)
	if !strings.Contains(body, "event: response.failed") {
		t.Errorf("expected response.failed event, got:\n%s", body)
	}
	if strings.Contains(body, "event: response.completed") {
		t.Errorf("response.completed must not be emitted on error:\n%s", body)
	}
}

func TestPumpResponsesSSE_ChannelClosedWithoutResultStillCompletes(t *testing.T) {
	events := []StreamEvent{
		{Kind: StreamEventMessageStart},
		{Kind: StreamEventTextDelta, Delta: "partial"},
	}
	body := pumpResponsesSSEHelper(t, events)
	if !strings.Contains(body, "event: response.completed") {
		t.Errorf("missing response.completed on premature channel close:\n%s", body)
	}
}

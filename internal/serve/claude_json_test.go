package serve

import (
	"strings"
	"testing"
)

// realClaudeJSON is a real `claude --output-format=json` capture from
// claude-opus-4-7 (2026-04-28). Used as the canonical fixture so the
// parser is exercised against actual wire bytes, not a hand-written
// approximation.
const realClaudeJSON = `{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"duration_ms":2879,"duration_api_ms":2817,"num_turns":1,"result":"Hi.","stop_reason":"end_turn","session_id":"68876525-f79d-4369-945c-1acd2e7ba665","total_cost_usd":0.36413124999999996,"usage":{"input_tokens":5,"cache_creation_input_tokens":58225,"cache_read_input_tokens":0,"output_tokens":8,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard","cache_creation":{"ephemeral_1h_input_tokens":58225,"ephemeral_5m_input_tokens":0},"inference_geo":"","iterations":[{"input_tokens":5,"output_tokens":8,"cache_read_input_tokens":0,"cache_creation_input_tokens":58225,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":58225},"type":"message"}],"speed":"standard"},"modelUsage":{"claude-opus-4-7[1m]":{"inputTokens":5,"outputTokens":8,"cacheReadInputTokens":0,"cacheCreationInputTokens":58225,"webSearchRequests":0,"costUSD":0.36413124999999996,"contextWindow":1000000,"maxOutputTokens":64000}},"permission_denials":[],"terminal_reason":"completed","fast_mode_state":"off","uuid":"9dc17684-8c6f-4212-8c32-074c337bb1ec"}`

func TestParseClaudeJSON_RealCapture(t *testing.T) {
	text, usage, err := parseClaudeJSON([]byte(realClaudeJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hi." {
		t.Errorf("text = %q, want %q", text, "Hi.")
	}
	if usage.InputTokens != 5 {
		t.Errorf("InputTokens = %d, want 5", usage.InputTokens)
	}
	if usage.OutputTokens != 8 {
		t.Errorf("OutputTokens = %d, want 8", usage.OutputTokens)
	}
	if usage.CacheCreationInputTokens != 58225 {
		t.Errorf("CacheCreationInputTokens = %d, want 58225", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 0 {
		t.Errorf("CacheReadInputTokens = %d, want 0", usage.CacheReadInputTokens)
	}
	if usage.TotalCostUSD <= 0 {
		t.Errorf("TotalCostUSD should be > 0, got %v", usage.TotalCostUSD)
	}
}

func TestParseClaudeJSON_NotJSON(t *testing.T) {
	_, _, err := parseClaudeJSON([]byte("plain text from a non-json claude run"))
	if err == nil {
		t.Fatal("expected error decoding non-JSON")
	}
	if !strings.Contains(err.Error(), "decode claude json") {
		t.Errorf("error context lost: %v", err)
	}
}

func TestParseClaudeJSON_WrongEnvelope(t *testing.T) {
	_, _, err := parseClaudeJSON([]byte(`{"type":"system","subtype":"init"}`))
	if err == nil {
		t.Fatal("expected error for non-result envelope")
	}
	if !strings.Contains(err.Error(), `envelope type "system"`) {
		t.Errorf("error didn't name the unexpected type: %v", err)
	}
}

func TestParseClaudeJSON_MinimalShape(t *testing.T) {
	// Smallest valid envelope — proves we don't require optional fields.
	got, usage, err := parseClaudeJSON([]byte(`{"type":"result","result":"ok","usage":{"input_tokens":3,"output_tokens":2}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("text = %q, want ok", got)
	}
	if usage.InputTokens != 3 || usage.OutputTokens != 2 {
		t.Errorf("usage = %+v", usage)
	}
}

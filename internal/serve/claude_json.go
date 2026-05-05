package serve

import (
	"encoding/json"
	"fmt"
)

// claudeJSONResult mirrors `claude --output-format=json` output.
//
// Captured live (2026-04-28, claude-opus-4-7) — only the fields devcell
// actually consumes are typed; iterations, modelUsage, permission_denials,
// server_tool_use, cache_creation breakdowns, etc. are deliberately
// dropped to keep the surface tight. Unknown fields are ignored by
// json.Unmarshal — adding a new typed field later is non-breaking.
//
// Real example (truncated for clarity):
//
//	{
//	  "type": "result",
//	  "subtype": "success",
//	  "is_error": false,
//	  "api_error_status": null,
//	  "duration_ms": 2879,
//	  "duration_api_ms": 2817,
//	  "num_turns": 1,
//	  "result": "Hi.",
//	  "stop_reason": "end_turn",
//	  "session_id": "68876525-...",
//	  "total_cost_usd": 0.36413124999999996,
//	  "usage": {
//	    "input_tokens": 5,
//	    "cache_creation_input_tokens": 58225,
//	    "cache_read_input_tokens": 0,
//	    "output_tokens": 8,
//	    "service_tier": "standard"
//	  }
//	}
type claudeJSONResult struct {
	Type           string             `json:"type"`             // expected: "result"
	Subtype        string             `json:"subtype"`          // "success" / "error_during_execution" / ...
	IsError        bool               `json:"is_error"`         // true on API or tool error
	APIErrorStatus *string            `json:"api_error_status"` // populated when claude got an HTTP error from the upstream API
	Result         string             `json:"result"`           // the assistant's text reply
	StopReason     string             `json:"stop_reason"`      // "end_turn", "max_tokens", "tool_use", ...
	SessionID      string             `json:"session_id"`
	NumTurns       int                `json:"num_turns"`
	DurationMs     int                `json:"duration_ms"`
	DurationAPIMs  int                `json:"duration_api_ms"`
	TotalCostUSD   float64            `json:"total_cost_usd"`
	Usage          claudeJSONUsageObj `json:"usage"`
}

// claudeJSONUsageObj is claude's per-turn token accounting (the four fields
// matter for billing). The remaining sub-objects (server_tool_use,
// cache_creation, iterations, ...) are ignored.
type claudeJSONUsageObj struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// parseClaudeJSON decodes claude --output-format=json output into the
// internal Usage shape and returns the assistant's text. Returns an error
// only when the bytes don't decode as JSON or aren't a "result" envelope —
// the caller falls back to raw stdout in that case.
func parseClaudeJSON(stdout []byte) (text string, usage Usage, err error) {
	var r claudeJSONResult
	if err := json.Unmarshal(stdout, &r); err != nil {
		return "", Usage{}, fmt.Errorf("decode claude json: %w", err)
	}
	if r.Type != "result" {
		return "", Usage{}, fmt.Errorf("unexpected claude json envelope type %q", r.Type)
	}
	usage = Usage{
		InputTokens:              r.Usage.InputTokens,
		OutputTokens:             r.Usage.OutputTokens,
		CacheCreationInputTokens: r.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     r.Usage.CacheReadInputTokens,
		TotalCostUSD:             r.TotalCostUSD,
	}
	return r.Result, usage, nil
}

package peer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type stubReader struct {
	all    []MemoryEntry
	search map[string][]MemoryEntry
}

func (r *stubReader) ReadAll() []MemoryEntry { return r.all }
func (r *stubReader) Search(q string) []MemoryEntry {
	if r.search != nil {
		return r.search[q]
	}
	return nil
}

func TestServer_Initialize(t *testing.T) {
	resp := runMCP(t, &stubReader{}, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	result := resp.Result.(map[string]interface{})
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocol version: %v", result["protocolVersion"])
	}
	info := result["serverInfo"].(map[string]interface{})
	if info["name"] != "peer-memory" {
		t.Errorf("server name: %v", info["name"])
	}
}

func TestServer_ToolsList(t *testing.T) {
	resp := runMCP(t, &stubReader{}, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0].(map[string]interface{})
	if tool["name"] != "read_peer_memory" {
		t.Errorf("tool name: %v", tool["name"])
	}
}

func TestServer_ToolCall_ReadAll(t *testing.T) {
	reader := &stubReader{all: []MemoryEntry{
		{Agent: "codex", ProjectName: "myproject", ProjectDir: "/home/user/dev/myproject", MemoryDir: "/home/user/.codex/memories_v2_1.sqlite", Timestamp: 1700000000, Name: "short-responses", Body: "User prefers short responses."},
		{Agent: "claude", ProjectName: "myproject", ProjectDir: "/home/user/dev/myproject", MemoryDir: "/home/user/.claude/projects/-home-user-dev-myproject/memory", Timestamp: 1700000001, Name: "run-tests", Body: "Always run tests."},
	}}
	resp := runMCP(t, reader, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_peer_memory","arguments":{}}}`)
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	text := extractToolText(t, resp)
	if !strings.Contains(text, "short responses") {
		t.Errorf("missing expected content: %s", text)
	}
	if !strings.Contains(text, "Always run tests") {
		t.Errorf("missing expected content: %s", text)
	}
	if !strings.Contains(text, `"agent"`) {
		t.Errorf("response should be JSON with agent field: %s", text)
	}
	if !strings.Contains(text, `"timestamp"`) {
		t.Errorf("response should be JSON with timestamp field: %s", text)
	}
}

func TestServer_ToolCall_Search(t *testing.T) {
	reader := &stubReader{
		all: []MemoryEntry{{Agent: "codex", Body: "foo"}, {Agent: "codex", Body: "bar"}},
		search: map[string][]MemoryEntry{"test": {
			{Agent: "claude", ProjectName: "myproject", Timestamp: 1700000000, Name: "run-tests", Body: "Always run tests."},
		}},
	}
	resp := runMCP(t, reader, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_peer_memory","arguments":{"query":"test"}}}`)
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}
	text := extractToolText(t, resp)
	if !strings.Contains(text, "Always run tests") {
		t.Errorf("search result: %s", text)
	}
}

func TestServer_ToolCall_EmptyResult(t *testing.T) {
	reader := &stubReader{}
	resp := runMCP(t, reader, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_peer_memory","arguments":{}}}`)
	text := extractToolText(t, resp)
	if text != "No peer memories found." {
		t.Errorf("empty result: %q", text)
	}
}

func TestServer_ToolCall_UnknownTool(t *testing.T) {
	resp := runMCP(t, &stubReader{}, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"unknown_tool","arguments":{}}}`)
	text := extractToolText(t, resp)
	if !strings.Contains(text, "Unknown tool") {
		t.Errorf("unknown tool: %s", text)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	resp := runMCP(t, &stubReader{}, `{"jsonrpc":"2.0","id":7,"method":"bogus/method"}`)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code: %d", resp.Error.Code)
	}
}

func TestServer_Notification_NoResponse(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	var out bytes.Buffer
	srv := &Server{Reader: &stubReader{}, Source: SourceClaude, In: in, Out: &out}
	srv.Run()
	if out.Len() != 0 {
		t.Errorf("notification should produce no output, got: %s", out.String())
	}
}

func TestServer_MultipleRequests(t *testing.T) {
	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_peer_memory","arguments":{}}}`,
	}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	srv := &Server{Reader: &stubReader{all: []MemoryEntry{{Agent: "codex", Body: "some memory"}}}, Source: SourceClaude, In: in, Out: &out}
	srv.Run()

	responses := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses (notification produces none), got %d: %v", len(responses), responses)
	}
}

func runMCP(t *testing.T, reader MemoryReader, request string) jsonrpcResponse {
	t.Helper()
	in := strings.NewReader(request + "\n")
	var out bytes.Buffer
	srv := &Server{Reader: reader, Source: SourceClaude, In: in, Out: &out}
	if err := srv.Run(); err != nil {
		t.Fatal(err)
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nraw: %s", err, out.String())
	}
	return resp
}

func extractToolText(t *testing.T, resp jsonrpcResponse) string {
	t.Helper()
	result := resp.Result.(map[string]interface{})
	content := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatal("no content in tool result")
	}
	return content[0].(map[string]interface{})["text"].(string)
}

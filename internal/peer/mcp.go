package peer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

const (
	SourceClaude = "claude"
	SourceCodex  = "codex"
)

// Server is a minimal MCP stdio server that exposes peer agent memories.
type Server struct {
	Reader MemoryReader
	Source string // which agent's memories this server reads ("claude" or "codex")
	In     io.Reader
	Out    io.Writer
}

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run reads JSON-RPC requests from In and writes responses to Out until EOF.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(s.In)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, -32700, "parse error")
			continue
		}
		s.handle(req)
	}
	return scanner.Err()
}

func (s *Server) handle(req jsonrpcRequest) {
	switch req.Method {
	case "initialize":
		s.writeResult(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "peer-memory",
				"version": "0.1.0",
			},
		})
	case "notifications/initialized":
		// no response needed for notifications
	case "tools/list":
		s.writeResult(req.ID, map[string]interface{}{
			"tools": []interface{}{s.toolDef()},
		})
	case "tools/call":
		s.handleToolCall(req)
	default:
		if req.ID != nil {
			s.writeError(req.ID, -32601, "method not found: "+req.Method)
		}
	}
}

func (s *Server) toolDef() map[string]interface{} {
	desc := fmt.Sprintf("Read your peer agent's (%s) project memories: conventions, preferences, feedback rules, and architectural decisions they've learned about this project.", s.Source)
	if s.Source == "all" {
		desc = "Read all agents' project memories (yours and your peers'): conventions, preferences, feedback rules, and architectural decisions learned about this project."
	}
	return map[string]interface{}{
		"name":        "read_peer_memory",
		"description": desc,
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Optional keyword filter. If empty, returns all memories.",
				},
			},
		},
	}
}

func (s *Server) handleToolCall(req jsonrpcRequest) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(req.ID, -32602, "invalid params")
		return
	}
	if params.Name != "read_peer_memory" {
		s.writeResult(req.ID, toolResult("Unknown tool: "+params.Name, true))
		return
	}
	query, _ := params.Arguments["query"].(string)
	var entries []MemoryEntry
	if query == "" {
		entries = s.Reader.ReadAll()
	} else {
		entries = s.Reader.Search(query)
	}
	if len(entries) == 0 {
		s.writeResult(req.ID, toolResult("No peer memories found.", false))
		return
	}
	data, err := json.Marshal(entries)
	if err != nil {
		s.writeResult(req.ID, toolResult("error marshaling memories: "+err.Error(), true))
		return
	}
	s.writeResult(req.ID, toolResult(string(data), false))
}

func toolResult(text string, isError bool) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
		"isError": isError,
	}
}

func (s *Server) writeResult(id json.RawMessage, result interface{}) {
	s.write(jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) writeError(id json.RawMessage, code int, msg string) {
	s.write(jsonrpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *Server) write(resp jsonrpcResponse) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(s.Out, "%s\n", data)
}

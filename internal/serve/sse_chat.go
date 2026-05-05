package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ChatStreamChoice is one element of a chat.completion.chunk's choices
// array. The `delta` carries the incremental content; `finish_reason` is
// null on every chunk except the last.
type ChatStreamChoice struct {
	Index        int             `json:"index"`
	Delta        ChatStreamDelta `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

// ChatStreamDelta is the OpenAI per-chunk delta. Role is sent on the
// first chunk; content on each text-delta chunk; both empty on the
// terminal chunk.
type ChatStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChatStreamChunk is the JSON payload of one SSE `data:` frame.
type ChatStreamChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"` // always "chat.completion.chunk"
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []ChatStreamChoice `json:"choices"`
	Usage   *ChatUsage         `json:"usage,omitempty"`
}

// streamChatCompletions runs claude in stream mode and writes Chat
// Completions SSE to w. Closes when claude finishes or the HTTP client
// disconnects (r.Context() cancellation propagates to claude via
// streamClaude/exec.CommandContext).
//
// includeUsage corresponds to OpenAI's stream_options.include_usage —
// when true the final chunk carries a usage object; when false (default)
// it doesn't.
func streamChatCompletions(
	w http.ResponseWriter,
	r *http.Request,
	opts ExecOpts,
	chatID, model string,
	includeUsage bool,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming requires an http.Flusher", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx response buffering

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	events, err := streamClaude(ctx, opts)
	if err != nil {
		writeChatErrorChunk(w, flusher, chatID, model, err)
		return
	}
	pumpChatSSE(ctx, w, flusher, events, chatID, model, includeUsage, 15*time.Second)
}

// pumpChatSSE consumes a stream of canonical events and writes Chat
// Completions SSE frames. Pure formatter — used by streamChatCompletions
// in production and exercised directly by tests with a synthetic channel.
//
// heartbeatInterval = 0 disables heartbeats (tests use this so they
// don't have to manage timers).
func pumpChatSSE(
	ctx context.Context,
	w io.Writer,
	flusher http.Flusher,
	events <-chan StreamEvent,
	chatID, model string,
	includeUsage bool,
	heartbeatInterval time.Duration,
) {
	created := time.Now().Unix()
	roleSent := false

	var heartbeatC <-chan time.Time
	if heartbeatInterval > 0 {
		t := time.NewTicker(heartbeatInterval)
		defer t.Stop()
		heartbeatC = t.C
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-heartbeatC:
			fmt.Fprint(w, ":keepalive\n\n")
			flusher.Flush()

		case ev, open := <-events:
			if !open {
				// Channel closed without a Result — emit a terminal
				// chunk so the client doesn't hang.
				writeChatTerminalChunk(w, flusher, chatID, model, created, "stop", nil, includeUsage)
				return
			}

			switch ev.Kind {
			case StreamEventMessageStart:
				if !roleSent {
					writeChatChunk(w, flusher, ChatStreamChunk{
						ID:      chatID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []ChatStreamChoice{{
							Index: 0,
							Delta: ChatStreamDelta{Role: "assistant"},
						}},
					})
					roleSent = true
				}

			case StreamEventTextDelta:
				if !roleSent {
					// Defensive: some streams skip message_start. Emit
					// the role chunk on the first delta we see.
					writeChatChunk(w, flusher, ChatStreamChunk{
						ID:      chatID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []ChatStreamChoice{{
							Index: 0,
							Delta: ChatStreamDelta{Role: "assistant"},
						}},
					})
					roleSent = true
				}
				writeChatChunk(w, flusher, ChatStreamChunk{
					ID:      chatID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   model,
					Choices: []ChatStreamChoice{{
						Index: 0,
						Delta: ChatStreamDelta{Content: ev.Delta},
					}},
				})

			case StreamEventResult:
				finish := "stop"
				if ev.Final != nil && ev.Final.IsError {
					finish = "error"
				}
				var usage *ChatUsage
				if includeUsage && ev.Final != nil {
					u := chatUsageFromExec(Usage{
						InputTokens:              ev.Final.Usage.InputTokens,
						OutputTokens:             ev.Final.Usage.OutputTokens,
						CacheCreationInputTokens: ev.Final.Usage.CacheCreationInputTokens,
						CacheReadInputTokens:     ev.Final.Usage.CacheReadInputTokens,
					})
					usage = &u
				}
				writeChatTerminalChunk(w, flusher, chatID, model, created, finish, usage, includeUsage)
				return

			case StreamEventError:
				writeChatErrorChunk(w, flusher, chatID, model, ev.Err)
				return

			default:
				// MessageStop and friends — nothing to emit; Result
				// is the OpenAI terminator.
			}
		}
	}
}

func writeChatChunk(w io.Writer, flusher http.Flusher, c ChatStreamChunk) {
	b, _ := json.Marshal(c)
	fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
}

func writeChatTerminalChunk(
	w io.Writer, flusher http.Flusher,
	chatID, model string, created int64,
	finish string, usage *ChatUsage, includeUsage bool,
) {
	finishPtr := finish
	chunk := ChatStreamChunk{
		ID:      chatID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []ChatStreamChoice{{
			Index:        0,
			Delta:        ChatStreamDelta{},
			FinishReason: &finishPtr,
		}},
	}
	if includeUsage {
		chunk.Usage = usage
	}
	writeChatChunk(w, flusher, chunk)
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeChatErrorChunk(w io.Writer, flusher http.Flusher, chatID, model string, err error) {
	finish := "error"
	c := ChatStreamChunk{
		ID:      chatID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChatStreamChoice{{
			Index:        0,
			Delta:        ChatStreamDelta{Content: err.Error()},
			FinishReason: &finish,
		}},
	}
	writeChatChunk(w, flusher, c)
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

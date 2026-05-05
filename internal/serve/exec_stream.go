package serve

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/DimmKirr/devcell/internal/logger"
)

// streamClaude spawns claude in --output-format=stream-json mode and
// returns a channel of canonical StreamEvents (see claude_stream.go).
// The channel closes when claude exits or stdout reaches EOF.
//
// Cancellation: the caller controls lifetime via ctx. exec.CommandContext
// kills the process when ctx is cancelled — used by SSE handlers to stop
// claude when the HTTP client closes the connection.
//
// Opencode is rejected with an error: it has no stream-json equivalent.
// SSE handlers fall back to the buffered path for opencode.
func streamClaude(ctx context.Context, opts ExecOpts) (<-chan StreamEvent, error) {
	if opts.Agent != "claude" {
		return nil, fmt.Errorf("streaming is only supported for claude (got %q)", opts.Agent)
	}

	cmd := exec.CommandContext(ctx, "claude", claudeArgs(opts, "stream-json")...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claude stdout pipe: %w", err)
	}
	// Capture stderr — useful in WARN logs if claude exits non-zero
	// before emitting a `result` envelope.
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	scan := scanClaudeStream(stdout)

	// Wrap so we can join `cmd.Wait` after the channel closes — without
	// this, killed-by-ctx subprocesses would leave zombie entries until
	// the Go runtime reaps them on its own schedule.
	out := make(chan StreamEvent, 16)
	go func() {
		defer close(out)
		for ev := range scan {
			out <- ev
		}
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			// Don't surface cancellation as an error — that's the
			// expected path when the HTTP client disconnects.
			logger.Warn("claude stream exited non-zero", "err", err.Error())
			out <- StreamEvent{Kind: StreamEventError, Err: fmt.Errorf("claude exited: %w", err)}
		}
	}()
	return out, nil
}

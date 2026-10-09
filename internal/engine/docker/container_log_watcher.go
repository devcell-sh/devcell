package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"time"

	"github.com/DimmKirr/devcell/internal/ux"
)

// ContainerEvent is one parsed JSONL event from docker logs.
type ContainerEvent struct {
	TS      time.Time `json:"ts"`
	Scope   string    `json:"scope"`
	Service string    `json:"service,omitempty"`
	Status  string    `json:"status,omitempty"`
	Msg     string    `json:"msg,omitempty"`
}

// ContainerLogWatcher follows docker logs for a running container,
// parses JSONL lines into ContainerEvent structs, and emits them on
// a buffered channel. Non-JSONL lines are silently skipped.
type ContainerLogWatcher struct {
	out    chan ContainerEvent
	cancel context.CancelFunc
	done   chan struct{}
}

// NewContainerLogWatcher starts `docker logs --follow --since=<since> <container>`
// and parses JSONL into ContainerEvents on the returned watcher's channel.
func NewContainerLogWatcher(ctx context.Context, container string, since time.Time) *ContainerLogWatcher {
	ctx, cancel := context.WithCancel(ctx)
	w := &ContainerLogWatcher{
		out:    make(chan ContainerEvent, 64),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	sinceStr := since.UTC().Format(time.RFC3339)
	cmd := exec.CommandContext(ctx, "docker", "logs", "--follow", "--since="+sinceStr, container)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ux.Debugf("container log watcher: stdout pipe: %v", err)
		close(w.out)
		close(w.done)
		return w
	}

	if err := cmd.Start(); err != nil {
		ux.Debugf("container log watcher: start: %v", err)
		close(w.out)
		close(w.done)
		return w
	}

	go func() {
		defer close(w.done)
		defer close(w.out)
		watchReader(stdout, w.out)
		_ = cmd.Wait()
	}()

	return w
}

// newContainerLogWatcherFromReader creates a watcher from an arbitrary reader.
// Used in tests to inject fake JSONL without a real docker subprocess.
func newContainerLogWatcherFromReader(r io.Reader) *ContainerLogWatcher {
	w := &ContainerLogWatcher{
		out:    make(chan ContainerEvent, 64),
		cancel: func() {},
		done:   make(chan struct{}),
	}
	go func() {
		defer close(w.done)
		defer close(w.out)
		watchReader(r, w.out)
	}()
	return w
}

// watchReader reads lines from r, parses JSONL, and sends ContainerEvents
// to out. Non-JSON lines and events without a scope are silently skipped.
func watchReader(r io.Reader, out chan<- ContainerEvent) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev ContainerEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Scope == "" {
			continue
		}
		out <- ev
	}
}

// Events returns the channel of parsed container events.
func (w *ContainerLogWatcher) Events() <-chan ContainerEvent {
	return w.out
}

// Close stops the docker logs subprocess and waits for the reader goroutine.
func (w *ContainerLogWatcher) Close() {
	w.cancel()
	<-w.done
}

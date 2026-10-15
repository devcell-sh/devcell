package cage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/devcell-sh/devcell/internal/ux"
)

// BootUIOptions configures RunBootUI, the renderer that runs inside the
// tmux popup (`devcell boot-ui <socket>`).
type BootUIOptions struct {
	Socket         string        // unix socket the window process serves events on
	ForwardPID     int           // process to receive the first Ctrl+C (the booting devcell)
	Stdin          *os.File      // keyboard, for "press any key"; nil means no wait
	ConnectTimeout time.Duration // give up connecting after this; default 3s
	Dwell          time.Duration // keep the finished panel visible before closing; default 400ms
}

// Exit codes of RunBootUI.
const (
	BootUIOK          = 0   // boot finished, popup may close
	BootUIFailed      = 1   // boot failed or the writer died; user acknowledged
	BootUIUnreachable = 2   // could not connect to the event socket
	BootUIInterrupted = 130 // second Ctrl+C
)

// RunBootUI connects to the boot event socket, replays the events into a
// BootPanel on this process's terminal and exits by the rules below:
//
//   - finish: wait Dwell, close the socket (the ack), return BootUIOK
//   - finish_error: hold the panel until a key is pressed, return BootUIFailed
//   - socket EOF before finish: the writer died; show "boot aborted", hold
//     for a key, return BootUIFailed
//   - Ctrl+C: forwarded to ForwardPID, never acted on here; a second one
//     within 2s returns BootUIInterrupted
func RunBootUI(ctx context.Context, o BootUIOptions) int {
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = 3 * time.Second
	}
	if o.Dwell == 0 {
		o.Dwell = 400 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	force := make(chan struct{}, 1)
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	go func() {
		var last time.Time
		for s := range sigs {
			if s == syscall.SIGTERM {
				force <- struct{}{}
				return
			}
			if !last.IsZero() && time.Since(last) < 2*time.Second {
				force <- struct{}{}
				return
			}
			last = time.Now()
			if o.ForwardPID > 0 {
				_ = syscall.Kill(o.ForwardPID, syscall.SIGINT)
			}
		}
	}()

	conn, err := dialRetry(ctx, o.Socket, o.ConnectTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "boot-ui: %v\n", err)
		return BootUIUnreachable
	}
	defer conn.Close()

	type item struct {
		ev  ux.BootEvent
		err error
	}
	items := make(chan item, 64)
	go func() {
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			var ev ux.BootEvent
			if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
				items <- item{err: fmt.Errorf("boot-ui: bad event: %w", err)}
				return
			}
			items <- item{ev: ev}
		}
		err := sc.Err()
		if err == nil {
			err = io.EOF
		}
		items <- item{err: err}
	}()

	var panel *ux.BootPanel
	for {
		select {
		case <-ctx.Done():
			return BootUIInterrupted
		case <-force:
			return BootUIInterrupted
		case it := <-items:
			if it.err != nil {
				if panel == nil {
					fmt.Fprintf(os.Stderr, "boot-ui: %v\n", it.err)
					return BootUIFailed
				}
				panel.FinishError("boot aborted")
				holdLine("boot aborted: the booting devcell went away")
				waitKey(ctx, o.Stdin, force)
				return BootUIFailed
			}
			ev := it.ev
			if panel == nil {
				if ev.Kind != ux.EventGroups {
					fmt.Fprintf(os.Stderr, "boot-ui: first event must be %s, got %s\n", ux.EventGroups, ev.Kind)
					return BootUIFailed
				}
				panel = ux.NewBootPanel(ev.Groups)
				continue
			}
			switch ev.Kind {
			case ux.EventFinish:
				panel.Finish(ev.Text)
				select {
				case <-time.After(o.Dwell):
				case <-force:
				}
				return BootUIOK
			case ux.EventFinishError:
				text := ev.Text
				if text == "" {
					text = "boot failed"
				}
				panel.FinishError(text)
				holdLine(text)
				waitKey(ctx, o.Stdin, force)
				return BootUIFailed
			default:
				if err := panel.Apply(ev); err != nil {
					ux.Debugf("boot-ui: %v", err)
				}
			}
		}
	}
}

// holdLine is printed under the final panel while boot-ui waits for a key.
// The streak RowLayout shows row messages, not the finish footer, so the
// reason and the hint live here.
func holdLine(text string) {
	fmt.Fprintf(os.Stderr, "\r\n %s %s %s\r\n", ux.StyleError.Render("✗"), text, ux.StyleMuted.Render("— press any key"))
}

func dialRetry(ctx context.Context, path string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.Dial("unix", path)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("connect %s: %w", path, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// waitKey blocks until one byte arrives on in (raw mode), ctx ends or a
// force signal fires. A nil or non-terminal in returns at once.
func waitKey(ctx context.Context, in *os.File, force <-chan struct{}) {
	if in == nil || !term.IsTerminal(int(in.Fd())) {
		return
	}
	old, err := term.MakeRaw(int(in.Fd()))
	if err == nil {
		defer term.Restore(int(in.Fd()), old) //nolint:errcheck
	}
	got := make(chan struct{}, 1)
	go func() {
		buf := make([]byte, 1)
		_, _ = in.Read(buf)
		got <- struct{}{}
	}()
	select {
	case <-got:
	case <-ctx.Done():
	case <-force:
	}
}

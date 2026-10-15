package cage_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/cage"
	"github.com/devcell-sh/devcell/internal/ptyctl"
	"github.com/devcell-sh/devcell/internal/ux"
	streak "github.com/dimmkirr/go-streak-chart"
)

// CELL-657: boot panel in a tmux popup. Unit level: preflight, argv, the
// boot-ui renderer against a scripted socket server.

func fakeTmux(version, size string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		switch args[0] {
		case "-V":
			return version + "\n", nil
		case "display":
			return size + "\n", nil
		}
		return "", errors.New("unexpected tmux args")
	}
}

func TestPopupPreflight(t *testing.T) {
	cases := []struct {
		version, size string
		groups        int
		want          cage.Geometry
		wantErr       string
	}{
		// height = groups + 3 (border, rule) + 2 (hold line), clamped to rows-4
		{"tmux 3.6a", "200 60", 4, cage.Geometry{Width: 76, Height: 9}, ""},
		{"tmux 3.6a", "200 60", 5, cage.Geometry{Width: 76, Height: 10}, ""},
		{"tmux 3.3a", "80 24", 4, cage.Geometry{Width: 76, Height: 9}, ""},
		{"tmux 3.3", "60 16", 5, cage.Geometry{Width: 56, Height: 10}, ""},
		{"tmux 3.3", "60 13", 5, cage.Geometry{Width: 56, Height: 9}, ""},
		{"tmux 3.2a", "200 60", 4, cage.Geometry{}, "older than 3.3"},
		{"tmux 2.9", "200 60", 4, cage.Geometry{}, "older than 3.3"},
		{"tmux 3.6a", "40 10", 4, cage.Geometry{}, "too small"},
		{"garbage", "200 60", 4, cage.Geometry{}, "cannot parse"},
	}
	for _, tc := range cases {
		got, err := cage.PopupPreflight(fakeTmux(tc.version, tc.size), tc.groups)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s/%s/%d: err = %v, want %q", tc.version, tc.size, tc.groups, err, tc.wantErr)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s/%s/%d: = %+v, %v; want %+v", tc.version, tc.size, tc.groups, got, err, tc.want)
		}
	}
}

func TestPopupCommand(t *testing.T) {
	spec := cage.PopupSpec{
		SocketPath: "/ws/.local/state/devcell/boot-p-1.sock",
		Title:      " main  p-1 ", Accent: "#818cf8",
		Geometry: cage.Geometry{Width: 70, Height: 13},
		Self:     "/usr/local/bin/devcell",
	}
	tmux, args := cage.PopupCommand(spec, 4242)
	if tmux != "tmux" {
		t.Errorf("tmux = %q", tmux)
	}
	want := []string{"display-popup", "-E", "-w", "70", "-h", "13", "-x", "C", "-y", "C",
		"-T", " main  p-1 ", "-S", "fg=#818cf8",
		"DEVCELL_BOOT_PID=4242 /usr/local/bin/devcell boot-ui /ws/.local/state/devcell/boot-p-1.sock"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("args =\n  %q\nwant\n  %q", args, want)
	}
}

// --- boot-ui against a scripted server, run under a pty via the helper ---

var bootGroups = []ux.PanelGroup{
	{Label: "Prepare", Steps: []string{"Docker", "Network"}},
	{Label: "Boot", Steps: []string{"Container"}},
}

type bootServer struct {
	t    *testing.T
	ln   net.Listener
	conn net.Conn
	path string
}

func newBootServer(t *testing.T) *bootServer {
	t.Helper()
	// Unix socket paths are short-limited; keep them in /tmp.
	dir, err := os.MkdirTemp("", "bui")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "b.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return &bootServer{t: t, ln: ln, path: path}
}

func (s *bootServer) accept(ctx context.Context) {
	s.t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := s.ln.Accept()
		if err == nil {
			s.conn = c
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
		s.t.Fatal("boot-ui never connected")
	}
	if s.conn == nil {
		s.t.Fatal("accept failed")
	}
}

func (s *bootServer) send(ev ux.BootEvent) {
	s.t.Helper()
	b, _ := json.Marshal(ev)
	if _, err := s.conn.Write(append(b, '\n')); err != nil {
		s.t.Fatalf("send: %v", err)
	}
}

// waitEOF reports whether boot-ui closed its end (the ack) within d.
func (s *bootServer) waitEOF(d time.Duration) bool {
	_ = s.conn.SetReadDeadline(time.Now().Add(d))
	_, err := bufio.NewReader(s.conn).ReadByte()
	return err != nil && !errors.Is(err, os.ErrDeadlineExceeded)
}

// startBootUI runs this test binary in helper mode under a pty.
func startBootUI(t *testing.T, ctx context.Context, sock string) *ptyctl.Terminal {
	t.Helper()
	term, err := ptyctl.StartWithOptions(ctx,
		[]ptyctl.StartOption{ptyctl.WithSize(80, 16), ptyctl.WithEnv("DEVCELL_CAGE_HELPER=bootui")},
		os.Args[0], "-test.run=^TestHelperProcess$", "--", "boot-ui", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })
	return term
}

func exitCode(t *testing.T, term *ptyctl.Terminal, ctx context.Context) int {
	t.Helper()
	select {
	case err := <-term.Done():
		if err == nil {
			return 0
		}
		var ee interface{ ExitCode() int }
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		t.Fatalf("boot-ui ended with %v", err)
	case <-ctx.Done():
		t.Fatalf("boot-ui did not exit\n%s", term.Screen())
	}
	return -1
}

func TestBootUI_RendersAndAcksFinish(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv := newBootServer(t)
	term := startBootUI(t, ctx, srv.path)
	srv.accept(ctx)
	srv.send(ux.BootEvent{Kind: ux.EventGroups, Groups: bootGroups})
	srv.send(ux.BootEvent{Kind: ux.EventRunning, Step: "Docker", Text: "Docker"})
	srv.send(ux.BootEvent{Kind: ux.EventDone, Step: "Docker"})
	srv.send(ux.BootEvent{Kind: ux.EventSet, Group: "Boot", Step: "Container", Status: streak.Running.String(), Text: "Starting container"})
	if _, err := term.WaitFor(ctx, "Starting container"); err != nil {
		t.Fatal(err)
	}
	if s := term.Screen(); !strings.Contains(s, "Prepare") || !strings.Contains(s, "Boot") {
		t.Errorf("panel groups not rendered:\n%s", s)
	}
	started := time.Now()
	srv.send(ux.BootEvent{Kind: ux.EventFinish, Text: "Cell ready"})
	if !srv.waitEOF(5 * time.Second) {
		t.Fatal("boot-ui never acked finish by closing the socket")
	}
	if d := time.Since(started); d < 350*time.Millisecond {
		t.Errorf("ack after %v, want a dwell of at least ~400ms so the final panel is readable", d)
	}
	if code := exitCode(t, term, ctx); code != cage.BootUIOK {
		t.Errorf("exit = %d, want %d", code, cage.BootUIOK)
	}
}

func TestBootUI_FinishErrorHoldsForKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv := newBootServer(t)
	term := startBootUI(t, ctx, srv.path)
	srv.accept(ctx)
	srv.send(ux.BootEvent{Kind: ux.EventGroups, Groups: bootGroups})
	srv.send(ux.BootEvent{Kind: ux.EventFinishError, Text: "start failed"})
	if _, err := term.WaitFor(ctx, "press any key"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-term.Done():
		t.Fatal("boot-ui exited before a key was pressed")
	case <-time.After(700 * time.Millisecond):
	}
	_ = term.Send("x")
	if code := exitCode(t, term, ctx); code != cage.BootUIFailed {
		t.Errorf("exit = %d, want %d", code, cage.BootUIFailed)
	}
}

// The window process dying mid-boot must not leave the popup hanging.
func TestBootUI_WriterDeathShowsAbortedAndHoldsForKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv := newBootServer(t)
	term := startBootUI(t, ctx, srv.path)
	srv.accept(ctx)
	srv.send(ux.BootEvent{Kind: ux.EventGroups, Groups: bootGroups})
	srv.send(ux.BootEvent{Kind: ux.EventRunning, Step: "Network", Text: "Network"})
	if _, err := term.WaitFor(ctx, "Network"); err != nil {
		t.Fatal(err)
	}
	srv.conn.Close() // writer died
	if _, err := term.WaitFor(ctx, "boot aborted"); err != nil {
		t.Fatal(err)
	}
	_ = term.Send("\r")
	if code := exitCode(t, term, ctx); code != cage.BootUIFailed {
		t.Errorf("exit = %d, want %d", code, cage.BootUIFailed)
	}
}

func TestBootUI_UnreachableSocket(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	code := cage.RunBootUI(ctx, cage.BootUIOptions{Socket: filepath.Join(t.TempDir(), "none.sock"), ConnectTimeout: 300 * time.Millisecond})
	if code != cage.BootUIUnreachable {
		t.Errorf("exit = %d, want %d", code, cage.BootUIUnreachable)
	}
}

// TestHelperProcess is re-executed by the tests above as a separate
// process: DEVCELL_CAGE_HELPER selects the role, args after "--" the input.
func TestHelperProcess(t *testing.T) {
	role := os.Getenv("DEVCELL_CAGE_HELPER")
	if role == "" {
		t.Skip("helper process only")
	}
	args := flag.Args()
	switch role {
	case "bootui":
		if len(args) != 2 || args[0] != "boot-ui" {
			fmt.Fprintf(os.Stderr, "helper bootui: bad args %q\n", args)
			os.Exit(64)
		}
		pid, _ := strconv.Atoi(os.Getenv("DEVCELL_BOOT_PID"))
		os.Exit(cage.RunBootUI(context.Background(), cage.BootUIOptions{Socket: args[1], ForwardPID: pid, Stdin: os.Stdin}))
	case "window":
		os.Exit(helperWindow(args))
	}
	fmt.Fprintf(os.Stderr, "helper: unknown role %q\n", role)
	os.Exit(64)
}

package cage

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devcell-sh/devcell/internal/cell"
	"github.com/devcell-sh/devcell/internal/ux"
)

// The CELL-657 popup: while a cell boots inside the cage, the boot panel
// is rendered by `devcell boot-ui` in a centred tmux popup instead of
// scrolling through the agent window. Sequencing is enforced by
// construction, never by timing:
//
//   - the window process (this one) owns the boot and never draws on the
//     tty while the popup is up: stdin is /dev/null, stdout and stderr go
//     to a log file until the agent is attached;
//   - events reach boot-ui over a unix socket; EOF either way means the
//     peer is gone;
//   - the agent is attached only after boot succeeded, boot-ui acked the
//     finish by closing its socket, and the popup client has exited.

// Geometry is the popup size, from PopupPreflight.
type Geometry struct{ Width, Height int }

// PopupSpec is what StartPopup needs.
type PopupSpec struct {
	SocketPath string   // unix socket for boot events
	LogPath    string   // where the window process's stdout/stderr go during boot
	Title      string   // popup title
	Accent     string   // tmux colour for the border
	Geometry   Geometry // from PopupPreflight
	Tmux       string   // tmux binary; default "tmux"
	Self       string   // devcell binary that runs boot-ui; default os.Executable()
	SelfArgs   []string // arguments placed before "boot-ui" (tests inject a helper)
	Env        []string // KEY=VALUE pairs for boot-ui; the popup runs from the tmux server's environment, not ours
}

const (
	popupMinCols       = 50
	popupMinRows       = 12
	popupMaxWidth      = 76
	popupChromeRows    = 4 // top and bottom border, the rule under the groups, the footer line
	popupIssueRows     = 3 // room for warnings listed under the rule before the popup clips
	popupConnectWindow = 1500 * time.Millisecond
	popupAckTimeout    = 5 * time.Second
)

var tmuxVersionRe = regexp.MustCompile(`tmux (\d+)\.(\d+)`)

// PopupHeight is the popup height for a boot panel of groups rows: the
// group rows plus chrome and room for a few issue lines, never taller than
// the client minus a margin.
func PopupHeight(groups, clientRows int) int {
	return min(groups+popupChromeRows+popupIssueRows, clientRows-4)
}

// PopupPreflight decides whether a popup can be shown: tmux 3.3 or newer
// (title, border and style flags) and a client large enough. run executes
// tmux with the given arguments and returns its stdout. groups is how many
// group rows the boot panel will draw (see PopupHeight).
func PopupPreflight(run func(args ...string) (string, error), groups int) (Geometry, error) {
	v, err := run("-V")
	if err != nil {
		return Geometry{}, fmt.Errorf("popup: tmux -V: %w", err)
	}
	m := tmuxVersionRe.FindStringSubmatch(v)
	if m == nil {
		return Geometry{}, fmt.Errorf("popup: cannot parse tmux version %q", strings.TrimSpace(v))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 3 || (major == 3 && minor < 3) {
		return Geometry{}, fmt.Errorf("popup: tmux %d.%d is older than 3.3", major, minor)
	}
	size, err := run("display", "-p", "#{client_width} #{client_height}")
	if err != nil {
		return Geometry{}, fmt.Errorf("popup: client size: %w", err)
	}
	var cols, rows int
	if _, err := fmt.Sscanf(strings.TrimSpace(size), "%d %d", &cols, &rows); err != nil {
		return Geometry{}, fmt.Errorf("popup: client size %q: %w", strings.TrimSpace(size), err)
	}
	if cols < popupMinCols || rows < popupMinRows {
		return Geometry{}, fmt.Errorf("popup: terminal %dx%d too small", cols, rows)
	}
	return Geometry{Width: min(popupMaxWidth, cols-4), Height: PopupHeight(groups, rows)}, nil
}

// TmuxRunner runs the tmux binary for PopupPreflight.
func TmuxRunner(tmux string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		out, err := exec.Command(tmux, args...).Output()
		return string(out), err
	}
}

// PopupCommand builds the tmux argv that opens the popup running boot-ui.
// The popup command is shell-joined; DEVCELL_BOOT_PID tells boot-ui where
// to forward Ctrl+C.
func PopupCommand(spec PopupSpec, pid int) (string, []string) {
	tmux := spec.Tmux
	if tmux == "" {
		tmux = "tmux"
	}
	words := append([]string{spec.Self}, spec.SelfArgs...)
	words = append(words, "boot-ui", spec.SocketPath)
	env := append([]string{"DEVCELL_BOOT_PID=" + strconv.Itoa(pid)}, spec.Env...)
	cmd := strings.Join(env, " ") + " " + cell.ShellJoin(words)
	args := []string{
		"display-popup", "-E",
		"-w", strconv.Itoa(spec.Geometry.Width), "-h", strconv.Itoa(spec.Geometry.Height),
		"-x", "C", "-y", "C",
		"-T", spec.Title,
	}
	if spec.Accent != "" {
		args = append(args, "-S", "fg="+spec.Accent)
	}
	return tmux, append(args, cmd)
}

type popupMode int

const (
	modeUndecided popupMode = iota
	modePopup               // boot-ui connected; it renders
	modeInline              // no client in time; inline panel on the restored tty
	modeSkipped             // boot finished before any client; nothing to show
)

// Popup is one boot popup session owned by the window process.
type Popup struct {
	spec PopupSpec

	ln         net.Listener
	client     *exec.Cmd
	clientDone chan error
	connected  chan struct{} // closed when boot-ui connects
	conn       net.Conn
	ackDone    chan struct{} // closed when boot-ui closes its end

	mu   sync.Mutex
	mode popupMode
	ep   *ux.EventPanel

	saved    [3]*os.File
	logFile  *os.File
	redirect bool
}

// StartPopup listens for boot-ui, redirects this process's stdio away from
// the tty and opens the popup. On error nothing is left redirected and the
// caller runs with the inline panel.
func StartPopup(spec PopupSpec) (*Popup, error) {
	if spec.Self == "" {
		self, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("popup: %w", err)
		}
		spec.Self = self
	}
	if err := os.MkdirAll(filepath.Dir(spec.SocketPath), 0o755); err != nil {
		return nil, fmt.Errorf("popup: %w", err)
	}
	_ = os.Remove(spec.SocketPath)
	ln, err := net.Listen("unix", spec.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("popup: listen: %w", err)
	}
	p := &Popup{
		spec:       spec,
		ln:         ln,
		clientDone: make(chan error, 1),
		connected:  make(chan struct{}),
		ackDone:    make(chan struct{}),
	}
	if err := p.redirectStdio(); err != nil {
		ln.Close()
		_ = os.Remove(spec.SocketPath)
		return nil, err
	}
	tmux, args := PopupCommand(spec, os.Getpid())
	p.client = exec.Command(tmux, args...)
	devnull, _ := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	p.client.Stdin = devnull
	p.client.Stdout, p.client.Stderr = p.logFile, p.logFile
	if err := p.client.Start(); err != nil {
		p.restoreStdio()
		ln.Close()
		_ = os.Remove(spec.SocketPath)
		return nil, fmt.Errorf("popup: %w", err)
	}
	go func() { p.clientDone <- p.client.Wait() }()
	go p.accept()
	return p, nil
}

func (p *Popup) accept() {
	conn, err := p.ln.Accept()
	if err != nil {
		return
	}
	p.mu.Lock()
	p.conn = conn
	p.mu.Unlock()
	close(p.connected)
	// boot-ui never writes; its close is the ack.
	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := conn.Read(buf); err != nil {
				close(p.ackDone)
				return
			}
		}
	}()
}

// redirectStdio points stdin at /dev/null and stdout/stderr at the log so
// nothing from the boot reaches the tty behind the popup. A blocked prompt
// now fails fast on EOF instead of hanging.
func (p *Popup) redirectStdio() error {
	logFile, err := os.OpenFile(p.spec.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("popup: %w", err)
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		logFile.Close()
		return fmt.Errorf("popup: %w", err)
	}
	p.saved = [3]*os.File{os.Stdin, os.Stdout, os.Stderr}
	p.logFile = logFile
	os.Stdin, os.Stdout, os.Stderr = devnull, logFile, logFile
	p.redirect = true
	return nil
}

func (p *Popup) restoreStdio() {
	if !p.redirect {
		return
	}
	os.Stdin, os.Stdout, os.Stderr = p.saved[0], p.saved[1], p.saved[2]
	p.redirect = false
}

// NewBootView is the engine.RunOpts.NewBootView hook. Events buffer from
// the first call; boot-ui gets them when it connects. If no client
// connects within the window, the popup is abandoned and the buffered
// events replay into an inline panel on the restored tty.
func (p *Popup) NewBootView(groups []ux.PanelGroup) ux.BootView {
	ep := ux.NewEventPanel(groups)
	p.mu.Lock()
	p.ep = ep
	p.mu.Unlock()
	go func() {
		select {
		case <-p.connected:
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.mode != modeUndecided {
				return
			}
			p.mode = modePopup
			if err := ep.Bind(&jsonSink{conn: p.conn}); err != nil {
				ux.Debugf("popup: bind: %v", err)
			}
		case <-time.After(popupConnectWindow):
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.mode != modeUndecided {
				return
			}
			p.mode = modeInline
			p.abandonClient()
			p.restoreStdio()
			ux.SaveCursor()
			inline := ux.NewBootPanel(groups)
			inline.SetBannerRows(0)
			if err := ep.Bind(inline); err != nil {
				ux.Debugf("popup: inline bind: %v", err)
			}
		}
	}()
	return ep
}

// abandonClient kills the popup client and stops listening. Caller holds mu.
func (p *Popup) abandonClient() {
	p.ln.Close()
	if p.client.Process != nil {
		_ = p.client.Process.Kill()
	}
	_ = os.Remove(p.spec.SocketPath)
}

// BeforeAttach is the engine.RunOpts.BeforeAttach hook: the agent gate.
// It returns once the popup is gone and the tty is ours again.
func (p *Popup) BeforeAttach() error {
	p.mu.Lock()
	switch p.mode {
	case modeUndecided:
		// Boot finished before boot-ui connected: nothing worth showing.
		p.mode = modeSkipped
		p.abandonClient()
		p.restoreStdio()
		p.mu.Unlock()
		return nil
	case modeInline, modeSkipped:
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	// modePopup: boot-ui closes its socket after the dwell (the ack), then
	// the popup closes and the client exits.
	select {
	case <-p.ackDone:
	case <-time.After(popupAckTimeout):
		ux.Debugf("popup: no ack from boot-ui, closing it")
	}
	select {
	case <-p.clientDone:
	case <-time.After(popupAckTimeout):
		ux.Debugf("popup: client still running, killing it")
		_ = p.client.Process.Kill()
		<-p.clientDone
	}
	p.mu.Lock()
	p.restoreStdio()
	p.ln.Close()
	_ = os.Remove(p.spec.SocketPath)
	p.mu.Unlock()
	return nil
}

// Finish closes the session after the engine returned. On a boot error the
// popup stays up until the user acknowledges it; on success everything was
// already torn down by BeforeAttach.
func (p *Popup) Finish(bootErr error) error {
	p.mu.Lock()
	mode := p.mode
	switch mode {
	case modeUndecided:
		p.mode = modeSkipped
		p.abandonClient()
	case modePopup:
		if bootErr != nil && p.ep != nil {
			if finished, _ := p.ep.Finished(); !finished {
				p.ep.FinishError(bootErr.Error())
			}
		}
	}
	p.mu.Unlock()

	if mode == modePopup && bootErr != nil {
		// boot-ui holds the panel until a key is pressed, then exits.
		<-p.clientDone
	}
	p.mu.Lock()
	p.restoreStdio()
	p.ln.Close()
	_ = os.Remove(p.spec.SocketPath)
	if p.logFile != nil {
		p.logFile.Close()
		p.logFile = nil
	}
	p.mu.Unlock()
	return bootErr
}

// jsonSink writes one JSON line per event with a single Write, so lines
// never interleave.
type jsonSink struct {
	mu   sync.Mutex
	conn net.Conn
}

func (s *jsonSink) Apply(ev ux.BootEvent) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.conn.Write(append(b, '\n'))
	return err
}

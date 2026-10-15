// Package cage draws the CELL-655 bar: a one-line tmux status line pinned
// to the bottom of the terminal while devcell boots a cell and runs the
// agent. tmux runs on the host; devcell re-executes itself inside the tmux
// window, so the boot panel and the agent both render under the bar.
//
// Left: the workspace name, then the cell name (<project>-<cellID>), each
// its own coloured element. Right: the status panel with clocks, joined with
// the theme separator. The prefix key is None, so every keystroke reaches the agent;
// the bar is display-only. Scrolling goes through tmux copy-mode (mouse on).
//
// Opt in with --experimental=ui-cage, DEVCELL_EXPERIMENTAL=ui-cage or
// [cell] experimental = ["ui-cage"] (see internal/experiments).
package cage

import (
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/devcell-sh/devcell/internal/cell"
	"github.com/devcell-sh/devcell/internal/cfg"
)

// EnvActive marks the devcell process that already runs inside the cage.
const EnvActive = "DEVCELL_CAGE"

// Spec is what the cage needs to render its bar and find its config.
type Spec struct {
	WorkspaceName string
	CellID        string
	CellName      string
	WorkspaceHome string // ~/.devcell/<workspace>; the config is written below it
	Theme         Theme  // from ResolveTheme; zero value means the default preset
}

// confDir is relative to the workspace home.
const confDir = ".local/state/devcell"

// StatePath is a file under the workspace's devcell state dir.
func StatePath(workspaceHome, name string) string {
	return filepath.Join(workspaceHome, confDir, name)
}

// SocketPath is the boot event socket for a cell. Unix socket paths are
// capped at about 104 bytes, so it lives in the temp dir and falls back to
// a hashed name when the cell name is long.
func SocketPath(cellName string) string {
	p := filepath.Join(os.TempDir(), "devcell-boot-"+cellName+".sock")
	if len(p) > 90 {
		sum := sha1.Sum([]byte(cellName))
		p = filepath.Join("/tmp", "devcell-boot-"+hex.EncodeToString(sum[:4])+".sock")
	}
	return p
}

// ConfPath is where the tmux config for this cell is written.
func ConfPath(spec Spec) string {
	return filepath.Join(spec.WorkspaceHome, confDir, "cage-"+spec.CellName+".tmux.conf")
}

// WriteConf renders and writes the tmux config, returning its path.
func WriteConf(spec Spec) (string, error) {
	path := ConfPath(spec)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("cage: %w", err)
	}
	if spec.Theme == (Theme{}) {
		spec.Theme, _ = ResolveTheme(cfg.ThemeSection{})
	}
	if err := os.WriteFile(path, []byte(TmuxConf(spec.WorkspaceName, spec.CellName, spec.Theme)), 0o644); err != nil {
		return "", fmt.Errorf("cage: %w", err)
	}
	return path, nil
}

// clocks are the status panel blocks, joined with the theme separator. Each is
// a tmux format; "%%" survives tmux's strftime pass as a literal "%" for
// date(1). ET uses a POSIX TZ rule so it needs no zoneinfo.
var clocks = []string{
	"UTC #(TZ=UTC date +%%H:%%M)",
	"ET #(TZ='EST5EDT,M3.2.0,M11.1.0' date +%%H:%%M)",
}

//go:embed cage.tmux.conf.tmpl
var tmuxConfTmpl string

var tmpl = template.Must(template.New("cage.tmux.conf").Parse(tmuxConfTmpl))

// TmuxConf renders the tmux configuration for the cage.
func TmuxConf(workspaceName, cellName string, theme Theme) string {
	var b strings.Builder
	err := tmpl.Execute(&b, struct {
		ThemeName string
		Workspace string
		Cell      string
		Status    string
		Theme     Theme
	}{
		ThemeName: themeName(theme),
		Workspace: tmuxQuote(" " + workspaceName + " "),
		Cell:      tmuxQuote(" " + cellName + " "),
		Status:    " " + strings.Join(clocks, theme.Separator) + " ",
		Theme:     theme,
	})
	if err != nil {
		// The template is embedded and the data is plain strings: this
		// cannot fail at runtime, and a panic here is a build defect.
		panic("cage: render tmux conf: " + err.Error())
	}
	return b.String()
}

// themeName reports which preset the colours match, or "custom".
func themeName(th Theme) string {
	th.Separator = ""
	for name, preset := range presets {
		if preset == th {
			return name
		}
	}
	return "custom"
}

// tmuxQuote escapes s for a double-quoted tmux option value: # starts a
// format, $ expands an environment variable, " and \ delimit.
func tmuxQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `$`, `\$`)
	s = strings.ReplaceAll(s, `#`, `##`)
	return s
}

// Active reports whether this process already runs inside the cage.
func Active() bool { return os.Getenv(EnvActive) == "1" }

// Enabled reports whether the cage can wrap this run: tmux draws the bar
// on the tty, so both ends must be terminals.
func Enabled(stdinTTY, stdoutTTY bool) bool { return stdinTTY && stdoutTTY }

// Command builds the tmux argv that runs `self args...` inside the cage.
// The session gets its own socket per cell so it never collides with the
// user's tmux server. The inner process is pinned to the same identity
// (workspace, cell ID) because inside tmux TMUX_PANE would say otherwise.
// The command is shell-joined, the form every tmux version accepts.
func Command(confPath string, spec Spec, windowName, self string, args []string) (string, []string) {
	return "tmux", []string{
		"-u", "-f", confPath,
		"-L", "devcell-cage-" + spec.CellName,
		"new-session", "-n", windowName,
		"-e", EnvActive + "=1",
		"-e", "DEVCELL_WORKSPACE=" + spec.WorkspaceName,
		"-e", "DEVCELL_CELL_ID=" + spec.CellID,
		cell.ShellJoin(append([]string{self}, args...)),
	}
}

// Run writes the config, starts tmux on the host with this terminal attached
// and re-runs devcell (os.Args[1:] as args) inside it. It returns when the
// agent exits and tmux closes with it.
func Run(spec Spec, agent string, args []string) error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("cage: tmux not found on the host; install it or drop the cage experimental feature")
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cage: %w", err)
	}
	conf, err := WriteConf(spec)
	if err != nil {
		return err
	}
	_, argv := Command(conf, spec, filepath.Base(agent), self, args)
	cmd := exec.Command(tmuxPath, argv...)
	// tmux draws through the tty it finds on stdin. Its stdout only carries
	// the client's own chatter, above all the "[exited]" line printed when
	// the session ends, which tmux offers no option to silence. Sending
	// stdout to /dev/null drops that line and nothing else.
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("cage: %w", err)
	}
	defer devnull.Close()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, devnull, os.Stderr
	// A client started inside another tmux refuses to nest while TMUX is
	// set; this cage has its own socket, so nesting is intended.
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TMUX=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, EnvActive+"=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cage: tmux: %w", err)
	}
	return nil
}

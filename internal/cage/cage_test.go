package cage_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/cage"
	"github.com/devcell-sh/devcell/internal/cfg"
	"github.com/devcell-sh/devcell/internal/ptyctl"
)

func defaultTheme(t *testing.T) cage.Theme {
	t.Helper()
	th, err := cage.ResolveTheme(cfg.ThemeSection{})
	if err != nil {
		t.Fatal(err)
	}
	return th
}

func TestTmuxConf_StatusLine(t *testing.T) {
	conf := cage.TmuxConf("main", "devcell-437", defaultTheme(t))
	for _, want := range []string{
		`set -g status on`,
		`set -g status-position bottom`,
		`set -g status-left "#[bg=#38bdf8,fg=#0f172a,bold] main #[bg=#334155,fg=#e2e8f0] devcell-437 #[default]"`,
		`set -g window-status-format ""`,
		`set -g window-status-current-format ""`,
		`UTC #(TZ=UTC date +%%H:%%M)`,
		`ET #(TZ='EST5EDT,M3.2.0,M11.1.0' date +%%H:%%M)`,
		`set -g prefix None`,
		`set -g mouse off`,
		`set -g allow-passthrough on`,
		`set -g extended-keys on`,
		`set -g set-clipboard on`,
		`set -s escape-time 0`,
		`set -g focus-events on`,
		`set -g default-terminal "tmux-256color"`,
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing %q:\n%s", want, conf)
		}
	}
	if strings.Contains(conf, "status 2") {
		t.Error("the cage is one line, not two")
	}
}

// tmux treats #, $ and " specially inside double-quoted option values.
func TestTmuxConf_EscapesNames(t *testing.T) {
	conf := cage.TmuxConf(`a#b"c$d`, `p#1`, defaultTheme(t))
	if want := `bold] a##b\"c\$d #[bg=#334155,fg=#e2e8f0] p##1 #[default]"`; !strings.Contains(conf, want) {
		t.Errorf("conf should contain %s, got:\n%s", want, conf)
	}
}

// The inner devcell must land on the same cell: inside the cage TMUX_PANE
// belongs to the cage's tmux, so identity is pinned through the env.
func TestCommand_RerunsSelfInTmuxWithPinnedIdentity(t *testing.T) {
	spec := cage.Spec{WorkspaceName: "main", CellID: "437", CellName: "proj-437"}
	bin, args := cage.Command("/ws/.local/state/devcell/cage-proj-437.tmux.conf", spec,
		"claude", "/usr/local/bin/devcell", []string{"claude", "--cage-on", "-p", "hello world"})
	if bin != "tmux" {
		t.Errorf("binary = %q, want tmux", bin)
	}
	want := []string{
		"-u", "-f", "/ws/.local/state/devcell/cage-proj-437.tmux.conf",
		"-L", "devcell-cage-proj-437",
		"new-session", "-n", "claude",
		"-e", "DEVCELL_CAGE=1",
		"-e", "DEVCELL_WORKSPACE=main",
		"-e", "DEVCELL_CELL_ID=437",
		"/usr/local/bin/devcell claude --cage-on -p 'hello world'",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args =\n  %q\nwant\n  %q", args, want)
	}
}

func TestWriteConf_UnderWorkspaceHome(t *testing.T) {
	home := t.TempDir()
	spec := cage.Spec{WorkspaceName: "main", CellID: "7", CellName: "proj-7", WorkspaceHome: home}
	path, err := cage.WriteConf(spec)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "state", "devcell", "cage-proj-7.tmux.conf"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != cage.TmuxConf("main", "proj-7", defaultTheme(t)) {
		t.Error("written conf differs from TmuxConf output with the default theme")
	}
}

// Off unless both ends are terminals; the inner run recognises itself.
func TestEnabledAndActive(t *testing.T) {
	if cage.Enabled(false, true) || cage.Enabled(true, false) || !cage.Enabled(true, true) {
		t.Error("Enabled must require a tty on stdin and stdout")
	}
	t.Setenv(cage.EnvActive, "")
	if cage.Active() {
		t.Error("Active() without DEVCELL_CAGE should be false")
	}
	t.Setenv(cage.EnvActive, "1")
	if !cage.Active() {
		t.Error("Active() with DEVCELL_CAGE=1 should be true")
	}
}

// The generated conf really produces the bar: run tmux under a pty and
// read the bottom row of the emulated screen.
func TestCage_RendersBarInTmux(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	home := t.TempDir()
	spec := cage.Spec{WorkspaceName: "main", CellID: "437", CellName: "test-" + filepath.Base(home), WorkspaceHome: home}
	conf, err := cage.WriteConf(spec)
	if err != nil {
		t.Fatal(err)
	}
	_, args := cage.Command(conf, spec, "sleep", "sleep", []string{"30"})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	term, err := ptyctl.StartWithOptions(ctx, []ptyctl.StartOption{ptyctl.WithSize(100, 20)}, "tmux", args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = exec.Command("tmux", "-L", "devcell-cage-"+spec.CellName, "kill-server").Run()
		_ = term.Close()
	}()
	if _, err := term.WaitFor(ctx, "UTC "); err != nil {
		t.Fatalf("status line never rendered: %v\nscreen:\n%s", err, term.Screen())
	}
	// The clocks come from async #() jobs; give them one status-interval.
	time.Sleep(1500 * time.Millisecond)
	lines := strings.Split(term.Screen(), "\n")
	bottom := strings.TrimSpace(lines[len(lines)-1])
	if !strings.Contains(bottom, "main") || !strings.Contains(bottom, spec.CellName) || !strings.Contains(bottom, "UTC") || !strings.Contains(bottom, "ET") {
		t.Errorf("bottom row = %q, want workspace, cell name and clocks\nscreen:\n%s", bottom, term.Screen())
	}
	if !strings.Contains(bottom, ":") {
		t.Errorf("bottom row = %q, want HH:MM clocks rendered", bottom)
	}
}

// Run points the tmux client's stdout at /dev/null so the "[exited]" line
// tmux prints when the session ends never reaches the terminal, while the
// bar (drawn through the tty on stdin) still renders.
func TestCage_ClientStdoutToDevNullDropsExitMessage(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	home := t.TempDir()
	spec := cage.Spec{WorkspaceName: "main", CellID: "9", CellName: "exit-" + filepath.Base(home), WorkspaceHome: home}
	conf, err := cage.WriteConf(spec)
	if err != nil {
		t.Fatal(err)
	}
	socket := "devcell-cage-" + spec.CellName
	script := "echo BEFORE; tmux -u -f " + conf + " -L " + socket + " new-session 'sleep 0.8' >/dev/null; echo AFTER"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	term, err := ptyctl.StartWithOptions(ctx, []ptyctl.StartOption{ptyctl.WithSize(80, 10)}, "sh", "-c", script)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
		_ = term.Close()
	}()
	if _, err := term.WaitFor(ctx, spec.CellName); err != nil {
		t.Fatalf("bar never rendered with stdout redirected: %v\n%s", err, term.Screen())
	}
	if _, err := term.WaitFor(ctx, "AFTER"); err != nil {
		t.Fatalf("tmux never exited: %v\n%s", err, term.Screen())
	}
	time.Sleep(300 * time.Millisecond)
	if screen := term.Screen(); strings.Contains(screen, "[exited]") {
		t.Errorf("tmux exit message leaked to the terminal:\n%s", screen)
	}
}

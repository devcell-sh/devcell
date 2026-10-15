package cage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devcell-sh/devcell/internal/cage"
	"github.com/devcell-sh/devcell/internal/cell"
	"github.com/devcell-sh/devcell/internal/ptyctl"
	"github.com/devcell-sh/devcell/internal/ux"
	streak "github.com/dimmkirr/go-streak-chart"
)

// End to end under a real tmux: the window process (helperWindow) boots
// with a popup, then prints the agent marker. The screen must never show
// the popup border and the agent marker together.

const (
	agentMarker = "AGENT-STARTS-HERE"
	failMarker  = "BOOT-FAILED-NO-AGENT"
)

// helperWindow is the window process: args = [window, <mode>, <sock>, <log>].
func helperWindow(args []string) int {
	if len(args) != 4 {
		fmt.Fprintf(os.Stderr, "helper window: bad args %q\n", args)
		return 64
	}
	mode, sock, log := args[1], args[2], args[3]
	groups := []ux.PanelGroup{
		{Label: "Prepare", Steps: []string{"Docker", "Network"}},
		{Label: "Boot", Steps: []string{"Container"}},
	}
	geom := cage.Geometry{Width: 56, Height: 10}
	if strings.HasPrefix(mode, "size") { // docker-shaped panel, popup sized like cmd does
		groups = dockerShapedGroups(strings.HasPrefix(mode, "size5"))
		geom = cage.Geometry{Width: 76, Height: cage.PopupHeight(len(groups), 24)}
	}
	spec := cage.PopupSpec{
		SocketPath: sock, LogPath: log, Title: " main  p-1 ", Accent: "#818cf8",
		Geometry: geom,
		Self:     os.Args[0], SelfArgs: []string{"-test.run=^TestHelperProcess$", "--"},
		// The popup command runs this binary again as boot-ui; the role
		// must travel in the command, the popup does not inherit our env.
		Env: []string{"DEVCELL_CAGE_HELPER=bootui"},
	}
	p, err := cage.StartPopup(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "popup: %v\n", err)
		return 65
	}
	view := p.NewBootView(groups)
	if strings.HasPrefix(mode, "size") {
		// Light every group so the full panel is on screen, then hold it.
		for _, g := range groups {
			view.SetBoot(g.Label, g.Steps[0], 2, g.Label+" ok")
		}
		view.Message("holding for the screen capture", streak.Running)
		time.Sleep(2500 * time.Millisecond)
	}
	_ = view.Phase("Docker", func() error { time.Sleep(400 * time.Millisecond); return nil })
	if mode == "die" {
		os.Exit(1) // writer dies mid-boot; boot-ui must notice
	}
	bootErr := view.Phase("Network", func() error {
		time.Sleep(400 * time.Millisecond)
		if strings.HasSuffix(mode, "fail") { // "fail", "size4fail"
			return errors.New("no route")
		}
		return nil
	})
	if bootErr != nil {
		view.FinishError("boot failed")
		_ = p.Finish(bootErr)
		fmt.Println(failMarker)
		time.Sleep(1500 * time.Millisecond)
		return 1
	}
	view.SetBoot("Boot", "Container", 2, "Container running") // streak.Done
	view.Finish("Cell ready")
	view.ClearBelowGroups()
	if err := p.BeforeAttach(); err != nil {
		fmt.Fprintf(os.Stderr, "gate: %v\n", err)
		return 66
	}
	fmt.Println(agentMarker) // stands in for docker exec
	time.Sleep(1500 * time.Millisecond)
	_ = p.Finish(nil)
	return 0
}

// runWindowUnderTmux starts a cage-like tmux session whose window command
// is the helper window process, and samples the screen until done returns
// true or the deadline passes. It returns every sampled screen.
func runWindowUnderTmux(t *testing.T, mode string, done func(screen string) bool) (screens []string, term *ptyctl.Terminal) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	dir, err := os.MkdirTemp("", "e2e")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "b.sock")
	log := filepath.Join(dir, "boot.log")
	socket := "devcell-cage-e2e-" + filepath.Base(dir)
	window := cell.ShellJoin([]string{os.Args[0], "-test.run=^TestHelperProcess$", "--", "window", mode, sock, log})
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	term, err = ptyctl.StartWithOptions(ctx,
		[]ptyctl.StartOption{ptyctl.WithSize(90, 24), ptyctl.WithEnv("DEVCELL_CAGE_HELPER=window")},
		"tmux", "-u", "-f", "/dev/null", "-L", socket, "new-session", "-e", "DEVCELL_CAGE_HELPER=window", window)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
		_ = term.Close()
	})
	for {
		s := term.Screen()
		screens = append(screens, s)
		if done(s) {
			return screens, term
		}
		select {
		case <-ctx.Done():
			logData, _ := os.ReadFile(log)
			t.Fatalf("timed out in mode %s; last screen:\n%s\nboot log:\n%s", mode, s, logData)
		case <-time.After(40 * time.Millisecond):
		}
	}
}

func hasPopup(s string) bool { return strings.Contains(s, "┌") || strings.Contains(s, "╭") }

// dockerShapedGroups mirrors the docker engine's panel: Prepare, Configure,
// Boot, Services (GUI only), Environment.
func dockerShapedGroups(gui bool) []ux.PanelGroup {
	g := []ux.PanelGroup{
		{Label: "Prepare", Steps: []string{"Docker", "Image", "Network", "Volume"}},
		{Label: "Configure", Steps: []string{"Prompt", "Secrets"}},
		{Label: "Boot", Steps: []string{"Container", "Entrypoint", "Agent"}},
	}
	if gui {
		g = append(g, ux.PanelGroup{Label: "Services", Steps: []string{"Display", "WM", "VNC"}})
	}
	return append(g, ux.PanelGroup{Label: "Environment", Steps: []string{"Shell", "Mise", "Home"}})
}

// The popup is sized from the group count (cage.PopupHeight). With the
// docker-shaped panel the groups and the rule must land inside the border,
// with exactly the hold-line rows left under the rule.
func TestPopupE2E_PopupFitsDockerShapedPanel(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		groups int
	}{{"size4", 4}, {"size5", 5}} {
		t.Run(tc.mode, func(t *testing.T) {
			labels := dockerShapedGroups(tc.groups == 5)
			last := labels[len(labels)-1].Label
			seen := 0
			screens, _ := runWindowUnderTmux(t, tc.mode, func(s string) bool {
				if hasPopup(s) && strings.Contains(s, last) {
					seen++ // a few more samples so every replayed event has landed
				}
				return seen > 10
			})
			screen := screens[len(screens)-1]
			lines := strings.Split(screen, "\n")
			top, bottom := -1, -1
			for i, l := range lines {
				switch {
				case strings.ContainsAny(l, "┌╭"):
					top = i
				case strings.ContainsAny(l, "└╰"):
					bottom = i
				}
			}
			if top < 0 || bottom < 0 {
				t.Fatalf("no popup border:\n%s", screen)
			}
			wantHeight := cage.PopupHeight(tc.groups, 24)
			if got := bottom - top + 1; got != wantHeight {
				t.Errorf("popup is %d rows on screen, PopupHeight says %d:\n%s", got, wantHeight, screen)
			}
			row := func(text string) int {
				for i, l := range lines {
					if strings.Contains(l, text) {
						return i
					}
				}
				return -1
			}
			for _, g := range labels {
				r := row(g.Label)
				if r <= top || r >= bottom {
					t.Errorf("group %q at row %d is outside the popup (%d..%d):\n%s", g.Label, r, top, bottom, screen)
				}
			}
			// The streak layout draws messages on the group rows, not on a
			// footer line: under the groups come the rule, then only the rows
			// boot-ui's hold line needs on failure.
			rule := -1
			for i := row(last) + 1; i < bottom; i++ {
				if strings.Contains(lines[i], "────") && !strings.ContainsAny(lines[i], "┌╭└╰") {
					rule = i
					break
				}
			}
			if rule != row(last)+1 {
				t.Errorf("rule at row %d, want right under the last group (%d):\n%s", rule, row(last)+1, screen)
			}
			wantSpare := wantHeight - 2 - tc.groups - 1 // border, groups, rule
			if spare := bottom - rule - 1; spare != wantSpare {
				t.Errorf("%d spare rows under the rule, want %d (hold line):\n%s", spare, wantSpare, screen)
			}
			t.Logf("%s popup %d rows:\n%s", tc.mode, bottom-top+1, screen)
		})
	}
}

func TestPopupE2E_AgentStartsOnlyAfterPopupClosed(t *testing.T) {
	screens, _ := runWindowUnderTmux(t, "ok", func(s string) bool { return strings.Contains(s, agentMarker) })
	sawPopup := false
	for i, s := range screens {
		if hasPopup(s) {
			sawPopup = true
			if strings.Contains(s, agentMarker) {
				t.Fatalf("sample %d shows popup and agent together:\n%s", i, s)
			}
		}
	}
	if !sawPopup {
		t.Error("popup never appeared in any sample")
	}
	last := screens[len(screens)-1]
	if hasPopup(last) {
		t.Errorf("popup still on screen when the agent started:\n%s", last)
	}
}

func TestPopupE2E_FailedBootNeverStartsAgent(t *testing.T) {
	screens, term := runWindowUnderTmux(t, "fail", func(s string) bool { return strings.Contains(s, "press any key") })
	for i, s := range screens {
		if strings.Contains(s, agentMarker) {
			t.Fatalf("sample %d shows the agent after a failed boot:\n%s", i, s)
		}
	}
	_ = term.Send("x")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := term.WaitFor(ctx, failMarker); err != nil {
		t.Fatal(err)
	}
	if s := term.Screen(); hasPopup(s) || strings.Contains(s, agentMarker) {
		t.Errorf("after acknowledging the failure the popup must be gone and no agent started:\n%s", s)
	}
}

// If the window process dies, tmux closes its window and with it the whole
// cage session, popup included. Nothing may linger: no popup, no agent.
func TestPopupE2E_WindowDeathEndsCage(t *testing.T) {
	_, term := runWindowUnderTmux(t, "die", func(s string) bool { return !hasPopup(s) && strings.Contains(s, "exited") })
	select {
	case <-term.Done():
	case <-time.After(10 * time.Second):
		t.Fatalf("tmux client still running after the window process died:\n%s", term.Screen())
	}
	if s := term.Screen(); hasPopup(s) || strings.Contains(s, agentMarker) {
		t.Errorf("popup or agent survived the window's death:\n%s", s)
	}
}

// A failed boot prints the hold line under the rule. The popup has exactly
// those rows: the line must be inside the border and nothing may scroll off
// the top.
func TestPopupE2E_FailedBootHoldLineFitsTightPopup(t *testing.T) {
	screens, _ := runWindowUnderTmux(t, "size4fail", func(s string) bool { return strings.Contains(s, "press any key") })
	screen := screens[len(screens)-1]
	lines := strings.Split(screen, "\n")
	top, bottom, hold, first := -1, -1, -1, -1
	for i, l := range lines {
		switch {
		case strings.ContainsAny(l, "┌╭"):
			top = i
		case strings.ContainsAny(l, "└╰"):
			bottom = i
		case strings.Contains(l, "press any key"):
			hold = i
		case strings.Contains(l, "Prepare"):
			first = i
		}
	}
	if top < 0 || bottom < 0 || hold < 0 {
		t.Fatalf("popup or hold line missing:\n%s", screen)
	}
	if want := cage.PopupHeight(4, 24); bottom-top+1 != want {
		t.Errorf("popup is %d rows, want %d:\n%s", bottom-top+1, want, screen)
	}
	if first != top+1 {
		t.Errorf("first group row at %d, want %d: the hold line scrolled the panel:\n%s", first, top+1, screen)
	}
	if hold >= bottom || hold != bottom-1 {
		t.Errorf("hold line at row %d, want the last inner row %d:\n%s", hold, bottom-1, screen)
	}
	t.Logf("failed boot in a tight popup:\n%s", screen)
}

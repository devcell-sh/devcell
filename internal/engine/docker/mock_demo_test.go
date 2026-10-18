package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	streak "github.com/dimmkirr/go-streak-chart"
	"github.com/dimmkirr/termoscope"

	"github.com/devcell-sh/devcell/internal/ux"
)

// helperMockDemo renders a scripted boot sequence with realistic pacing
// for demo recordings. Nothing here talks to Docker or the network: every
// phase callback returns canned data immediately, and boot events are
// synthetic. The result looks like a real `devcell claude` boot but runs
// anywhere in < 10 s.
func helperMockDemo() {
	ux.LogPlainText = false

	ux.SaveCursor()
	fmt.Fprintln(os.Stderr, " "+ux.Banner("CELL", "my-project", "1"))

	groups := bootGroups("claude", groupOpts{
		hasPackages:  true,
		hasGitLookup: true,
		hasSecrets:   true,
		hasAPIKeys:   true,
		hasMCP:       true,
		hasGUI:       true,
	})
	panel := ux.NewBootPanel(groups)

	pace := 120 * time.Millisecond

	// ── Prepare (6 steps) ───────────────────────────────────────────
	_ = panel.Phase("Docker", func() error { return nil })
	time.Sleep(pace)
	_ = panel.Phase("Network", func() error { return nil })
	time.Sleep(pace)
	_ = panel.Phase("Orphan cleanup", func() error { return nil })
	time.Sleep(pace)
	_ = panel.PhaseDetailedWarn("Nix store", func() (string, bool, error) {
		return "drifted — 2 profile hashes (drift)", true, nil
	})
	time.Sleep(pace)
	_ = panel.Phase("Backup", func() error { return nil })
	time.Sleep(pace)
	_ = panel.PhaseDetailed("Image pin", func() (string, error) {
		return "sha256:e4f8a20b91c3", nil
	})
	time.Sleep(pace)

	// ── Configure (5 steps) ─────────────────────────────────────────
	_ = panel.PhaseDetailed("Packages", func() (string, error) {
		return "pre-commit, shellcheck", nil
	})
	time.Sleep(pace)
	_ = panel.PhaseDetailed("Git", func() (string, error) {
		return "user <dev@example.com>", nil
	})
	time.Sleep(pace)
	_ = panel.PhaseDetailed("Prompt", func() (string, error) {
		return "configured", nil
	})
	time.Sleep(pace)
	_ = panel.PhaseDetailedRunningWarn(
		"Loading secrets (please authorize 1Password)", "Secrets",
		func() (string, bool, error) {
			detail, warn := ux.FormatSecretsPhase(12, 0)
			return detail, warn, nil
		})
	time.Sleep(pace)
	_ = panel.Phase("API keys", func() error { return nil })
	time.Sleep(pace)

	// ── Boot (5 steps, via events) ──────────────────────────────────
	panel.SetBoot("Boot", "Container", streak.Done, "Container started")

	events := make(chan ContainerEvent, 20)
	go func() {
		send := func(e ContainerEvent) {
			events <- e
			time.Sleep(pace)
		}
		send(ContainerEvent{Scope: "system", Service: "entrypoint", Status: "ready", Msg: "Entrypoint ready"})
		send(ContainerEvent{Scope: "system", Service: "nix", Status: "starting", Msg: "Configuring nix"})
		send(ContainerEvent{Scope: "system", Service: "nix", Status: "ready", Msg: "Nix ready"})
		send(ContainerEvent{Scope: "system", Service: "s6", Status: "starting", Msg: "Activating session services"})
		send(ContainerEvent{Scope: "system", Service: "s6", Status: "ready", Msg: "Session services ready"})
		send(ContainerEvent{Scope: "system", Service: "secrets", Status: "ready", Msg: "Secrets ready"})

		// Services
		send(ContainerEvent{Scope: "service", Service: "xvfb", Status: "up", Msg: "Display server ready"})
		send(ContainerEvent{Scope: "service", Service: "dbus-session", Status: "up", Msg: "D-Bus session ready"})
		send(ContainerEvent{Scope: "service", Service: "window-manager", Status: "up", Msg: "Window manager ready"})
		send(ContainerEvent{Scope: "service", Service: "xrdp", Status: "up", Msg: "Remote desktop ready"})
		send(ContainerEvent{Scope: "service", Service: "pulseaudio", Status: "up", Msg: "Audio server ready"})

		// Environment
		send(ContainerEvent{Scope: "service", Service: "shell-rc", Status: "up", Msg: "Shell ready"})
		send(ContainerEvent{Scope: "service", Service: "mise", Status: "up", Msg: "Mise ready"})
		send(ContainerEvent{Scope: "service", Service: "homedir", Status: "up", Msg: "Home directory ready"})
		send(ContainerEvent{Scope: "service", Service: "claude-config", Status: "up", Msg: "Claude ready"})
		send(ContainerEvent{Scope: "service", Service: "mcp-toggle", Status: "up", Msg: "MCP ready"})
		send(ContainerEvent{Scope: "service", Service: "gui-config", Status: "up", Msg: "GUI ready"})

		events <- ContainerEvent{Scope: "system", Service: "boot", Status: "ready"}
		close(events)
	}()

	ConsumeContainerEventsPanel(events, panel)
	panel.ClearBelowGroups()
	printMockDemoTUI()

	// Brief pause so the viewer sees the finished boot before typing starts.
	time.Sleep(500 * time.Millisecond)

	// Move cursor back to the prompt line and type "Hello World" character
	// by character. Layout after printMockDemoTUI:
	//   line N-2: rule
	//   line N-1: ❯              ← target
	//   line N  : rule
	//   line N+1: status bar
	//   cursor here (N+2)
	// So 3 lines up, column 3 (after "❯ ").
	fmt.Print("\x1b[3A\x1b[3G")
	for _, ch := range "Hello World" {
		time.Sleep(60 * time.Millisecond)
		fmt.Print(string(ch))
	}
	// Hold the final frame so the recording ends on the typed prompt.
	time.Sleep(800 * time.Millisecond)
}

// printMockDemoTUI renders a Claude Code welcome screen for the demo.
func printMockDemoTUI() {
	const (
		pink   = "\x1b[38;5;174m"
		bgDark = "\x1b[48;5;16m"
		noBg   = "\x1b[49m"
		grey   = "\x1b[38;5;246m"
		ruleC  = "\x1b[38;5;244m"
		pinkLt = "\x1b[38;5;211m"
		bold   = "\x1b[1m"
		noBold = "\x1b[22m"
		reset  = "\x1b[0m"
	)
	rule := ruleC + strings.Repeat("─", 80) + reset

	fmt.Printf(" %s▐%s▛███▛█%s%s   %sClaude Code%s  %sv2.1.283%s\n",
		pink, bgDark, noBg, reset, bold, noBold, grey, reset)
	fmt.Printf("%s▝▜%s█████%s█▀%s   %sOpus 4.6 with high effort · Claude Max%s\n",
		pink, bgDark, noBg, reset, grey, reset)
	fmt.Printf(" %s▝▝   ▝▝%s   %s/my-project%s\n",
		pink, reset, grey, reset)
	fmt.Println()
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("❯ ")
	fmt.Println(rule)
	fmt.Printf("  %s▸▸%s bypass permissions on %s(shift+tab to cycle)%s\n",
		pinkLt, reset, grey, reset)
}

func TestMockDemo(t *testing.T) {
	if os.Getenv("TERMPROOF_HELPER") == "mock-demo" {
		helperMockDemo()
		os.Exit(0)
	}

	bin := buildDockerTestBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Setenv("TERMPROOF_HELPER", "mock-demo")
	pinTruecolorTerminal(t)
	tm, err := termoscope.Start(ctx, 120, 17, bin, "-test.run=^TestMockDemo$")
	if err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	termoscope.RecordWith(t, tm, termoscope.Options{
		MinCols: 120,
		MinRows: 17,
	})

	if _, err := tm.WaitFor(ctx, "Hello World"); err != nil {
		t.Fatal(err)
	}
	_ = tm.Wait()

	termoscope.SavePNG(t, tm, "mock-demo-final")
	termoscope.SaveSVG(t, tm, "mock-demo-final")

	// ── Sanity checks: the recording produced a plausible screen ────
	line0 := tm.Line(0)
	if !strings.Contains(line0, "CELL") || !strings.Contains(line0, "my-project") {
		t.Errorf("line 0 (banner): expected 'CELL · my-project', got %q", strings.TrimRight(line0, " "))
	}

	groupLabels := []string{"Prepare", "Configure", "Boot", "Services", "Environment"}
	for i, label := range groupLabels {
		line := tm.Line(i + 1)
		if !strings.Contains(line, label) {
			t.Errorf("line %d: expected group %q, got %q", i+1, label, strings.TrimRight(line, " "))
		}
	}

	if !strings.Contains(tm.Line(7), "Claude Code") {
		t.Errorf("line 7: expected 'Claude Code', got %q", strings.TrimRight(tm.Line(7), " "))
	}

	// Prompt line should show the typed input.
	if !strings.Contains(tm.Line(13), "Hello World") {
		t.Errorf("line 13 (prompt): expected 'Hello World', got %q", strings.TrimRight(tm.Line(13), " "))
	}

	green, yellow, red, _ := countDots(tm)
	if green == 0 {
		t.Error("expected at least some green dots in mock demo")
	}
	if red != 0 {
		t.Errorf("mock demo should have no red dots, got %d", red)
	}
	t.Logf("dots: green=%d yellow=%d red=%d", green, yellow, red)
}

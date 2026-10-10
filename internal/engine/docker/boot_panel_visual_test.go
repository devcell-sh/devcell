package docker

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	streak "github.com/dimmkirr/go-streak-chart"
	"github.com/dimmkirr/termoscope"

	"github.com/DimmKirr/devcell/internal/ux"
)

// helperRealBoot replicates a real `cell claude` boot with 5 groups (27 dots),
// followed by a Claude Code TUI welcome screen:
//
//	cell ▸ CELL · devcell #3           ← banner (line 0)
//	Prepare      ■ ■ ■ ■ ■ ■          ← group 0
//	Configure    ■ ■ ■ ■ ■            ← group 1
//	Boot         ■ ■ ■ ■ ■            ← group 2
//	Services     ■ ■ ■ ■ ■            ← group 3
//	Environment  ■ ■ ■ ■ ■ ■          ← group 4
//	───────────────────────────────     ← rule (finish line)
//	 ▐▛███▛█   Claude Code  v2.1.283   ← logo + title
//	▝▜██████▀  Opus 4.6 with ...       ← model info
//	 ▝▝   ▝▝   /devcell-2             ← directory
//	                                    ← empty
//	                                    ← empty
//	───────────────────────────────     ← rule
//	❯                                  ← prompt
//	───────────────────────────────     ← rule
//	  ⏵⏵ bypass permissions on ...     ← status bar
//
// Expected: 25 green, 2 yellow (Nix store + Secrets), 0 red.
func helperRealBoot() {
	ux.LogPlainText = false

	ux.SaveCursor()
	fmt.Fprintln(os.Stderr, " "+ux.Banner("CELL", "devcell", "3"))

	groups := bootGroups("claude", groupOpts{
		hasPackages:  true,
		hasGitLookup: true,
		hasSecrets:   true,
		hasAPIKeys:   true,
		hasMCP:       true,
		hasGUI:       true,
	})
	panel := ux.NewBootPanel(groups)

	pace := 60 * time.Millisecond

	// Prepare (6 steps): Phase calls in the runner
	_ = panel.Phase("Docker", func() error { return nil })
	_ = panel.Phase("Network", func() error { return nil })
	_ = panel.Phase("Orphan cleanup", func() error { return nil })
	_ = panel.PhaseDetailedWarn("Nix store", func() (string, bool, error) {
		return "drifted — 4 profile hashes (drift)", true, nil
	})
	_ = panel.Phase("Backup", func() error { return nil })
	_ = panel.PhaseDetailed("Image pin", func() (string, error) {
		return "sha256:abc1234567", nil
	})
	time.Sleep(pace)

	// Configure (5 steps): Phase calls in the runner
	_ = panel.PhaseDetailed("Packages", func() (string, error) {
		return "pre-commit", nil
	})
	_ = panel.PhaseDetailed("Git", func() (string, error) {
		return "user <dev@hazelops.com>", nil
	})
	_ = panel.PhaseDetailed("Prompt", func() (string, error) {
		return "configured", nil
	})
	_ = panel.PhaseDetailedRunningWarn(
		"Loading secrets (please authorize 1Password)", "Secrets",
		func() (string, bool, error) {
			detail, warn := ux.FormatSecretsPhase(18, 1)
			return detail, warn, nil
		})
	_ = panel.Phase("API keys", func() error { return nil })
	time.Sleep(pace)

	// Boot: Container set by runner, rest via boot events
	panel.SetBoot("Boot", "Container", streak.Done, "Container started")

	events := make(chan BootEvent, 4)
	go func() {
		send := func(e BootEvent) {
			events <- e
			time.Sleep(pace)
		}
		send(BootEvent{Component: "entrypoint", State: "ready", Title: "Entrypoint ready"})
		send(BootEvent{Component: "nix", State: "starting", Title: "Configuring nix"})
		send(BootEvent{Component: "nix", State: "ready", Title: "Nix ready"})
		send(BootEvent{Component: "s6", State: "starting", Title: "Activating session services"})
		send(BootEvent{Component: "s6", State: "ready", Title: "Session services ready"})
		send(BootEvent{Component: "secrets", State: "ready", Title: "Secrets ready"})
		send(BootEvent{Component: "xvfb", State: "ready", Title: "Display server ready"})
		send(BootEvent{Component: "dbus-session", State: "ready", Title: "D-Bus session ready"})
		send(BootEvent{Component: "window-manager", State: "ready", Title: "Window manager ready"})
		send(BootEvent{Component: "xrdp", State: "ready", Title: "Remote desktop ready"})
		send(BootEvent{Component: "pulseaudio", State: "ready", Title: "Audio server ready"})
		send(BootEvent{Component: "shell", State: "ready", Title: "Shell ready"})
		send(BootEvent{Component: "mise", State: "ready", Title: "Mise ready"})
		send(BootEvent{Component: "home", State: "ready", Title: "Home directory ready"})
		send(BootEvent{Component: "claude", State: "ready", Title: "Claude ready"})
		send(BootEvent{Component: "mcp-toggle", State: "ready", Title: "MCP ready"})
		send(BootEvent{Component: "gui", State: "ready", Title: "GUI ready"})
		events <- BootEvent{Component: "boot", State: "ready"}
		close(events)
	}()

	ConsumeBootEventsPanel(events, panel)
	panel.ClearBelowGroups()
	printClaudeCodeTUI()
	fmt.Println("DONE")
}

// printClaudeCodeTUI renders a Claude Code welcome screen matching the
// real TUI: block-art logo, model/plan info, rules, ❯ prompt, status bar.
func printClaudeCodeTUI() {
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
	fmt.Printf(" %s▝▝   ▝▝%s   %s/devcell-2%s\n",
		pink, reset, grey, reset)
	fmt.Println()
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("❯ ")
	fmt.Println(rule)
	fmt.Printf("  %s⏵⏵%s bypass permissions on %s(shift+tab to cycle)%s\n",
		pinkLt, reset, grey, reset)
}

func TestBootPanelVisual(t *testing.T) {
	if os.Getenv("TERMPROOF_HELPER") == "real" {
		helperRealBoot()
		os.Exit(0)
	}

	bin := filepath.Join(t.TempDir(), "docker.test")
	build := exec.Command("go", "test", "-c", "-o", bin, "./internal/engine/docker/")
	build.Dir = "/devcell-2"
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Setenv("TERMPROOF_HELPER", "real")
	tm, err := termoscope.Start(ctx, 120, 24, bin, "-test.run=^TestBootPanelVisual$")
	if err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	termoscope.Record(t, tm)

	if _, err := tm.WaitFor(ctx, "DONE"); err != nil {
		t.Fatal(err)
	}
	_ = tm.Wait()

	termoscope.SavePNG(t, tm, "final")
	termoscope.SaveSVG(t, tm, "final")

	// ── Dot color assertions ────────────────────────────────────────────
	green, yellow, red, grey := countDots(tm)
	t.Logf("dots: green=%d yellow=%d red=%d grey=%d (total=%d)",
		green, yellow, red, grey, green+yellow+red+grey)

	// Prepare: 6 (5 green + 1 yellow Nix store)
	// Configure: 5 (4 green + 1 yellow Secrets)
	// Boot: 5 (5 green)
	// Services: 5 (5 green — Display, Desktop, Remote, Audio, D-Bus)
	// Environment: 6 (6 green)
	// Total: 27 dots, 25 green, 2 yellow
	if green != 25 {
		t.Errorf("green dots = %d, want 25", green)
	}
	if yellow != 2 {
		t.Errorf("yellow dots = %d, want 2 (Nix store + Secrets)", yellow)
	}
	if red != 0 {
		t.Errorf("red dots = %d, want 0", red)
	}
	if total := green + yellow + red + grey; total != 27 {
		t.Errorf("total dots = %d, want 27", total)
	}

	// ── Line layout assertions ──────────────────────────────────────────
	// The final screen must have this vertical structure:
	//   line 0:  banner       "cell ▸ CELL · devcell #3"
	//   line 1:  group 0      "Prepare      ■ ■ ■ ..."
	//   line 2:  group 1      "Configure    ■ ■ ■ ..."
	//   line 3:  group 2      "Boot         ■ ■ ■ ..."
	//   line 4:  group 3      "Services     ■ ■ ■ ..."
	//   line 5:  group 4      "Environment  ■ ■ ■ ..."
	//   line 6:  rule         "───────────────────────"
	//   line 7:  logo row 1   "▐▛███▛█   Claude Code  v2.1.283"
	//   line 8:  logo row 2   "▝▜██████▀  Opus 4.6 ..."
	//   line 9:  logo row 3   "▝▝   ▝▝   /devcell-2"
	//   line 10: empty
	//   line 11: empty
	//   line 12: rule         "───────────────────────"
	//   line 13: prompt       "❯ "
	//   line 14: rule         "───────────────────────"
	//   line 15: status bar   "⏵⏵ bypass permissions ..."
	//   line 16: sentinel     "DONE"

	// Dump all lines for diagnostics
	for y := 0; y < tm.Height(); y++ {
		line := strings.TrimRight(tm.Line(y), " ")
		if line != "" {
			t.Logf("line %2d: %q", y, line)
		}
	}

	// Line 0: banner
	line0 := tm.Line(0)
	if !strings.Contains(line0, "cell") || !strings.Contains(line0, "CELL") || !strings.Contains(line0, "devcell") {
		t.Errorf("line 0 (banner): expected 'cell ▸ CELL · devcell', got %q", strings.TrimRight(line0, " "))
	}

	// Lines 1-5: group labels
	groupLabels := []string{"Prepare", "Configure", "Boot", "Services", "Environment"}
	for i, label := range groupLabels {
		line := tm.Line(i + 1)
		if !strings.Contains(line, label) {
			t.Errorf("line %d: expected group %q, got %q", i+1, label, strings.TrimRight(line, " "))
		}
	}

	// Line 6: rule (panel finish)
	if !strings.Contains(tm.Line(6), "───") {
		t.Errorf("line 6 (rule): expected ─── separator, got %q", strings.TrimRight(tm.Line(6), " "))
	}

	// Line 7: Claude Code logo + title
	if !strings.Contains(tm.Line(7), "Claude Code") {
		t.Errorf("line 7 (title): expected 'Claude Code', got %q", strings.TrimRight(tm.Line(7), " "))
	}

	// Line 8: model info
	if !strings.Contains(tm.Line(8), "Opus") {
		t.Errorf("line 8 (model): expected 'Opus', got %q", strings.TrimRight(tm.Line(8), " "))
	}

	// Line 9: project path
	if !strings.Contains(tm.Line(9), "/devcell-2") {
		t.Errorf("line 9 (project): expected '/devcell-2', got %q", strings.TrimRight(tm.Line(9), " "))
	}

	// Line 13: prompt
	if !strings.Contains(tm.Line(13), "❯") {
		t.Errorf("line 13 (prompt): expected '❯', got %q", strings.TrimRight(tm.Line(13), " "))
	}

	// Line 15: status bar
	if !strings.Contains(tm.Line(15), "bypass") {
		t.Errorf("line 15 (status): expected 'bypass', got %q", strings.TrimRight(tm.Line(15), " "))
	}

	// Logo starts right after the rule (line 7), verifying
	// ClearBelowGroups erased the right number of lines.
	logoLine := -1
	for y := 0; y < tm.Height(); y++ {
		if strings.Contains(tm.Line(y), "Claude Code") {
			logoLine = y
			break
		}
	}
	if logoLine != 7 {
		t.Errorf("Claude Code logo on line %d, want line 7 (banner + 5 groups + rule)", logoLine)
	}
}

// helperRealBootMidScreen prints 10 lines of prior terminal output, then
// runs the same boot as helperRealBoot. This simulates starting `cell claude`
// from a terminal that already has content above.
func helperRealBootMidScreen() {
	for i := 0; i < 10; i++ {
		fmt.Fprintf(os.Stderr, "prior-output-%d\n", i)
	}
	helperRealBoot()
}

func TestBootPanelVisual_MidScreen(t *testing.T) {
	if os.Getenv("TERMPROOF_HELPER") == "midscreen" {
		helperRealBootMidScreen()
		os.Exit(0)
	}

	bin := filepath.Join(t.TempDir(), "docker.test")
	build := exec.Command("go", "test", "-c", "-o", bin, "./internal/engine/docker/")
	build.Dir = "/devcell-2"
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Setenv("TERMPROOF_HELPER", "midscreen")
	tm, err := termoscope.Start(ctx, 120, 34, bin, "-test.run=^TestBootPanelVisual_MidScreen$")
	if err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	termoscope.Record(t, tm)

	if _, err := tm.WaitFor(ctx, "DONE"); err != nil {
		t.Fatal(err)
	}
	_ = tm.Wait()

	termoscope.SavePNG(t, tm, "final")
	termoscope.SaveSVG(t, tm, "final")

	// Dump all non-blank lines for diagnostics
	for y := 0; y < tm.Height(); y++ {
		line := strings.TrimRight(tm.Line(y), " ")
		if line != "" {
			t.Logf("line %2d: %q", y, line)
		}
	}

	// Lines 0-9: prior terminal output
	for i := 0; i < 10; i++ {
		line := tm.Line(i)
		want := fmt.Sprintf("prior-output-%d", i)
		if !strings.Contains(line, want) {
			t.Errorf("line %d: expected %q, got %q", i, want, strings.TrimRight(line, " "))
		}
	}

	// Line 10: banner
	line10 := tm.Line(10)
	if !strings.Contains(line10, "cell") || !strings.Contains(line10, "CELL") {
		t.Errorf("line 10 (banner): expected 'cell ▸ CELL', got %q", strings.TrimRight(line10, " "))
	}

	// Lines 11-15: group labels
	groupLabels := []string{"Prepare", "Configure", "Boot", "Services", "Environment"}
	for i, label := range groupLabels {
		line := tm.Line(11 + i)
		if !strings.Contains(line, label) {
			t.Errorf("line %d: expected group %q, got %q", 11+i, label, strings.TrimRight(line, " "))
		}
	}

	// Line 16: rule (panel finish)
	if !strings.Contains(tm.Line(16), "───") {
		t.Errorf("line 16 (rule): expected ─── separator, got %q", strings.TrimRight(tm.Line(16), " "))
	}

	// Line 17: Claude Code logo + title
	if !strings.Contains(tm.Line(17), "Claude Code") {
		t.Errorf("line 17 (title): expected 'Claude Code', got %q", strings.TrimRight(tm.Line(17), " "))
	}

	// Line 18: model info
	if !strings.Contains(tm.Line(18), "Opus") {
		t.Errorf("line 18 (model): expected 'Opus', got %q", strings.TrimRight(tm.Line(18), " "))
	}

	// Line 19: project path
	if !strings.Contains(tm.Line(19), "/devcell-2") {
		t.Errorf("line 19 (project): expected '/devcell-2', got %q", strings.TrimRight(tm.Line(19), " "))
	}

	// Line 23: prompt
	if !strings.Contains(tm.Line(23), "❯") {
		t.Errorf("line 23 (prompt): expected '❯', got %q", strings.TrimRight(tm.Line(23), " "))
	}

	// Line 25: status bar
	if !strings.Contains(tm.Line(25), "bypass") {
		t.Errorf("line 25 (status): expected 'bypass', got %q", strings.TrimRight(tm.Line(25), " "))
	}

	// The prior output (lines 0-9) must be untouched by ClearBelowGroups
	for i := 0; i < 10; i++ {
		line := tm.Line(i)
		want := fmt.Sprintf("prior-output-%d", i)
		if !strings.Contains(line, want) {
			t.Errorf("prior output line %d was corrupted: %q", i, strings.TrimRight(line, " "))
		}
	}

	// Logo starts at line 17 = 10 (prior) + 1 (banner) + 5 (groups) + 1 (rule)
	logoLine := -1
	for y := 0; y < tm.Height(); y++ {
		if strings.Contains(tm.Line(y), "Claude Code") {
			logoLine = y
			break
		}
	}
	if logoLine != 17 {
		t.Errorf("Claude Code logo on line %d, want line 17 (10 prior + banner + 5 groups + rule)", logoLine)
	}
}

func countDots(tm *termoscope.Terminal) (green, yellow, red, grey int) {
	for y := 0; y < tm.Height(); y++ {
		for x := 0; x < tm.Width(); x++ {
			c := tm.CellAt(x, y)
			if c == nil || c.Content != "■" || c.Style.Fg == nil {
				continue
			}
			switch classifyDotColor(c.Style.Fg) {
			case "green":
				green++
			case "yellow":
				yellow++
			case "red":
				red++
			case "grey":
				grey++
			}
		}
	}
	return
}

// classifyDotColor identifies a dot's status from its foreground color using
// HSL hue. Works for both light and dark terminal themes.
func classifyDotColor(c color.Color) string {
	ri, gi, bi, _ := c.RGBA()
	R, G, B := float64(ri)/65535.0, float64(gi)/65535.0, float64(bi)/65535.0

	maxC := math.Max(R, math.Max(G, B))
	minC := math.Min(R, math.Min(G, B))
	chroma := maxC - minC

	if chroma < 0.15 {
		return "grey"
	}

	var hue float64
	switch {
	case maxC == R:
		hue = 60 * math.Mod((G-B)/chroma, 6)
	case maxC == G:
		hue = 60 * ((B-R)/chroma + 2)
	default:
		hue = 60 * ((R-G)/chroma + 4)
	}
	if hue < 0 {
		hue += 360
	}

	switch {
	case hue >= 80 && hue < 180:
		return "green"
	case hue >= 20 && hue < 80:
		return "yellow"
	case hue >= 330 || hue < 20:
		return "red"
	case hue >= 200 && hue < 270:
		return "blue"
	}
	return "unknown"
}

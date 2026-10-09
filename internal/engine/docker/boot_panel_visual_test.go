package docker

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	streak "github.com/dimmkirr/go-streak-chart"
	"github.com/dimmkirr/termoscope"

	"github.com/DimmKirr/devcell/internal/ux"
)

// helperRealBoot replicates a real `cell claude` boot with 5 groups (27 dots):
//
//	Prepare      ▄ ▄ ▄ ▄ ▄ ▄  Nix store: drifted
//	Configure    ▄ ▄ ▄ ▄ ▄    Secrets : 1 failed — 18 resolved, 1 failed
//	Boot         ▄ ▄ ▄ ▄ ▄    Session services ready
//	Services     ▄ ▄ ▄ ▄ ▄    (Display, Desktop, Remote, Audio, D-Bus)
//	Environment  ▄ ▄ ▄ ▄ ▄ ▄
//
// Expected: 25 green, 2 yellow (Nix store + Secrets), 0 red.
func helperRealBoot() {
	ux.LogPlainText = false

	groups := bootGroups("claude", groupOpts{
		hasPackages:  true,
		hasGitLookup: true,
		hasSecrets:   true,
		hasAPIKeys:   true,
		hasMCP:       true,
		hasGUI:       true,
	})
	panel := ux.NewBootPanel(groups)

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

	// Boot: Container set by runner, rest via boot events
	panel.SetBoot("Boot", "Container", streak.Done, "Container started")

	events := make(chan BootEvent, 24)
	events <- BootEvent{Component: "entrypoint", State: "ready", Title: "Entrypoint ready"}
	events <- BootEvent{Component: "nix", State: "starting", Title: "Configuring nix"}
	events <- BootEvent{Component: "nix", State: "ready", Title: "Nix ready"}
	events <- BootEvent{Component: "s6", State: "starting", Title: "Activating session services"}
	events <- BootEvent{Component: "s6", State: "ready", Title: "Session services ready"}
	events <- BootEvent{Component: "secrets", State: "ready", Title: "Secrets ready"}
	events <- BootEvent{Component: "xvfb", State: "ready", Title: "Display server ready"}
	events <- BootEvent{Component: "dbus-session", State: "ready", Title: "D-Bus session ready"}
	events <- BootEvent{Component: "window-manager", State: "ready", Title: "Window manager ready"}
	events <- BootEvent{Component: "xrdp", State: "ready", Title: "Remote desktop ready"}
	events <- BootEvent{Component: "pulseaudio", State: "ready", Title: "Audio server ready"}
	events <- BootEvent{Component: "shell", State: "ready", Title: "Shell ready"}
	events <- BootEvent{Component: "mise", State: "ready", Title: "Mise ready"}
	events <- BootEvent{Component: "home", State: "ready", Title: "Home directory ready"}
	events <- BootEvent{Component: "claude", State: "ready", Title: "Claude ready"}
	events <- BootEvent{Component: "mcp-toggle", State: "ready", Title: "MCP ready"}
	events <- BootEvent{Component: "gui", State: "ready", Title: "GUI ready"}
	events <- BootEvent{Component: "boot", State: "ready"}
	close(events)

	ConsumeBootEventsPanel(events, panel)
	fmt.Println("DONE")
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
	tm, err := termoscope.Start(ctx, 120, 20, bin, "-test.run=^TestBootPanelVisual$")
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
}

func countDots(tm *termoscope.Terminal) (green, yellow, red, grey int) {
	for y := 0; y < tm.Height(); y++ {
		for x := 0; x < tm.Width(); x++ {
			c := tm.CellAt(x, y)
			if c == nil || c.Content != "▄" || c.Style.Fg == nil {
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

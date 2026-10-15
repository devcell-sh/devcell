package cage_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devcell-sh/devcell/internal/cage"
	"github.com/devcell-sh/devcell/internal/cfg"
)

// CELL-655: [theme] picks a preset by name and overrides single colours.

func TestResolveTheme_DefaultIsCoolTone(t *testing.T) {
	th, err := cage.ResolveTheme(cfg.ThemeSection{})
	if err != nil {
		t.Fatal(err)
	}
	if th.WorkspaceBG != "#38bdf8" || th.CellBG != "#334155" || th.BarBG != "#0f172a" || th.Separator != " | " {
		t.Errorf("default theme = %+v, want the cool-tone devcell palette", th)
	}
	brand, err := cage.ResolveTheme(cfg.ThemeSection{Name: "brand"})
	if err != nil || brand.WorkspaceBG != "#e85d26" {
		t.Errorf("brand preset should carry the site orange, got %+v (%v)", brand, err)
	}
}

func TestResolveTheme_PresetByName(t *testing.T) {
	for _, name := range cage.ThemeNames() {
		th, err := cage.ResolveTheme(cfg.ThemeSection{Name: name})
		if err != nil {
			t.Errorf("preset %q: %v", name, err)
			continue
		}
		if th.BarBG == "" || th.BarFG == "" || th.WorkspaceBG == "" || th.WorkspaceFG == "" || th.CellBG == "" || th.CellFG == "" || th.StatusBG == "" || th.StatusFG == "" || th.Accent == "" {
			t.Errorf("preset %q has an empty colour: %+v", name, th)
		}
	}
	if _, err := cage.ResolveTheme(cfg.ThemeSection{Name: "solarized"}); err == nil || !strings.Contains(err.Error(), "solarized") {
		t.Errorf("unknown preset should name itself in the error, got %v", err)
	}
}

func TestResolveTheme_OverridesWinOverPreset(t *testing.T) {
	th, err := cage.ResolveTheme(cfg.ThemeSection{Name: "nord", CellBG: "colour208", Separator: " / "})
	if err != nil {
		t.Fatal(err)
	}
	if th.CellBG != "colour208" || th.Separator != " / " {
		t.Errorf("overrides not applied: %+v", th)
	}
	nord, _ := cage.ResolveTheme(cfg.ThemeSection{Name: "nord"})
	if th.BarBG != nord.BarBG {
		t.Errorf("untouched colours must come from the preset: %q vs %q", th.BarBG, nord.BarBG)
	}
}

// A bad colour would make tmux stop reading the config at that line, so it
// is rejected before the cell boots.
func TestResolveTheme_RejectsBadColours(t *testing.T) {
	for _, bad := range []string{"#12345", "red; run-shell x", "colour999", "#gggggg", `"`} {
		if _, err := cage.ResolveTheme(cfg.ThemeSection{Accent: bad}); err == nil {
			t.Errorf("colour %q should be rejected", bad)
		}
	}
	for _, ok := range []string{"#E85D26", "colour208", "color8", "default", "brightred", "terminal"} {
		if _, err := cage.ResolveTheme(cfg.ThemeSection{Accent: ok}); err != nil {
			t.Errorf("colour %q should be accepted: %v", ok, err)
		}
	}
}

func TestTmuxConf_AppliesTheme(t *testing.T) {
	th, _ := cage.ResolveTheme(cfg.ThemeSection{Name: "nord", Separator: " / "})
	conf := cage.TmuxConf("main", "proj-1", th)
	for _, want := range []string{
		`set -g status-style "bg=#2e3440,fg=`,
		`#[bg=#88c0d0,fg=#2e3440,bold] main #[bg=#434c5e,fg=#eceff4] proj-1 #[default]`,
		`UTC #(TZ=UTC date +%%H:%%M) / ET `,
		`set -g mode-style`,
		`set -g message-style`,
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing %q:\n%s", want, conf)
		}
	}
	if strings.Contains(conf, "{{") || strings.Contains(conf, "<no value>") {
		t.Errorf("template not fully rendered:\n%s", conf)
	}
}

// Every preset must load in tmux without a single error line.
func TestTmuxConf_AllPresetsLoadInTmux(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	for _, name := range cage.ThemeNames() {
		th, err := cage.ResolveTheme(cfg.ThemeSection{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		conf := filepath.Join(t.TempDir(), name+".conf")
		if err := os.WriteFile(conf, []byte(cage.TmuxConf("ws", "1", th)), 0o644); err != nil {
			t.Fatal(err)
		}
		sock := "devcell-cage-theme-" + filepath.Base(filepath.Dir(conf))
		out, err := exec.Command("tmux", "-L", sock, "-f", conf, "start-server", ";", "kill-server").CombinedOutput()
		if err != nil || len(strings.TrimSpace(string(out))) > 0 {
			t.Errorf("preset %q: tmux rejected the config: %v\n%s", name, err, out)
		}
	}
}

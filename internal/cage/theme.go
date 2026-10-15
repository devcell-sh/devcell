package cage

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/devcell-sh/devcell/internal/cfg"
)

// Theme is the resolved colour set the cage template renders. The bar has
// three elements, each coloured on its own: the workspace name, the cell
// name and the status panel (clocks). Bar* colours fill the gap between.
type Theme struct {
	BarBG, BarFG             string
	WorkspaceBG, WorkspaceFG string
	CellBG, CellFG           string
	StatusBG, StatusFG       string
	Accent                   string
	Separator                string
}

const defaultPreset = "devcell"

// presets are the built-in palettes. The default is a cool-tone palette
// (slate, sky, indigo); "brand" carries the website's orange and lime.
var presets = map[string]Theme{
	"devcell": {
		BarBG: "#0f172a", BarFG: "#94a3b8",
		WorkspaceBG: "#38bdf8", WorkspaceFG: "#0f172a",
		CellBG: "#334155", CellFG: "#e2e8f0",
		StatusBG: "#1e293b", StatusFG: "#cbd5e1",
		Accent: "#818cf8",
	},
	"brand": {
		BarBG: "#111111", BarFG: "#aaaaaa",
		WorkspaceBG: "#e85d26", WorkspaceFG: "#0d0d0d",
		CellBG: "#2a2a2a", CellFG: "#e5e5e5",
		StatusBG: "#1a1a1a", StatusFG: "#e5e5e5",
		Accent: "#c8ff00",
	},
	"nord": {
		BarBG: "#2e3440", BarFG: "#d8dee9",
		WorkspaceBG: "#88c0d0", WorkspaceFG: "#2e3440",
		CellBG: "#434c5e", CellFG: "#eceff4",
		StatusBG: "#3b4252", StatusFG: "#eceff4",
		Accent: "#81a1c1",
	},
	"catppuccin-mocha": {
		BarBG: "#1e1e2e", BarFG: "#cdd6f4",
		WorkspaceBG: "#89b4fa", WorkspaceFG: "#11111b",
		CellBG: "#45475a", CellFG: "#cdd6f4",
		StatusBG: "#313244", StatusFG: "#cdd6f4",
		Accent: "#cba6f7",
	},
	"tokyo-night": {
		BarBG: "#1a1b26", BarFG: "#a9b1d6",
		WorkspaceBG: "#7aa2f7", WorkspaceFG: "#1a1b26",
		CellBG: "#414868", CellFG: "#c0caf5",
		StatusBG: "#24283b", StatusFG: "#c0caf5",
		Accent: "#bb9af7",
	},
}

// ThemeNames lists the presets, sorted.
func ThemeNames() []string {
	names := make([]string, 0, len(presets))
	for n := range presets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ResolveTheme turns [theme] into a Theme: the named preset (or the default)
// with every non-empty override applied. Colours are checked here because
// tmux stops reading the config at the first bad one.
func ResolveTheme(t cfg.ThemeSection) (Theme, error) {
	name := t.Name
	if name == "" {
		name = defaultPreset
	}
	th, ok := presets[name]
	if !ok {
		return Theme{}, fmt.Errorf("[theme] name %q unknown; presets: %v", name, ThemeNames())
	}
	th.Separator = " | "
	if t.Separator != "" {
		th.Separator = t.Separator
	}
	for _, o := range []struct {
		key  string
		val  string
		dest *string
	}{
		{"bar_bg", t.BarBG, &th.BarBG}, {"bar_fg", t.BarFG, &th.BarFG},
		{"workspace_bg", t.WorkspaceBG, &th.WorkspaceBG}, {"workspace_fg", t.WorkspaceFG, &th.WorkspaceFG},
		{"cell_bg", t.CellBG, &th.CellBG}, {"cell_fg", t.CellFG, &th.CellFG},
		{"status_bg", t.StatusBG, &th.StatusBG}, {"status_fg", t.StatusFG, &th.StatusFG},
		{"accent", t.Accent, &th.Accent},
	} {
		if o.val == "" {
			continue
		}
		if !validTmuxColour(o.val) {
			return Theme{}, fmt.Errorf("[theme] %s = %q: not a tmux colour (#rrggbb, colour0-255 or a colour name)", o.key, o.val)
		}
		*o.dest = o.val
	}
	return th, nil
}

var (
	hexColourRe   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	indexColourRe = regexp.MustCompile(`^colou?r([0-9]{1,3})$`)
	nameColourRe  = regexp.MustCompile(`^[a-z]+$`)
)

// validTmuxColour accepts what tmux's colour_fromstring does and nothing
// that could break out of a style string.
func validTmuxColour(s string) bool {
	if hexColourRe.MatchString(s) || nameColourRe.MatchString(s) {
		return true
	}
	if m := indexColourRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n <= 255
	}
	return false
}

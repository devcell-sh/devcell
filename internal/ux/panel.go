package ux

import (
	"fmt"
	"os"
	"strings"

	streak "github.com/dimmkirr/go-streak-chart"
)

// PanelGroup defines one row in the panel: a label and its ordered steps.
type PanelGroup struct {
	Label string
	Steps []string
}

// BootPanel renders startup progress as a streak-chart panel. Each group
// is a row; each step within a group is a dot (column) that transitions
// Pending -> Running -> Done/Error/Warning.
//
// Implements Phaser so it can be used as a drop-in for PhaseRunner.
type BootPanel struct {
	loader *streak.Loader
	groups []PanelGroup
	// groupIdx maps group label -> row index in the grid.
	groupIdx map[string]int
	// stepIdx maps "group\x00step" -> column index within that row.
	stepIdx map[string]int
}

func stepKey(group, step string) string { return group + "\x00" + step }

// NewBootPanel creates a panel from the given groups and starts the Loader.
func NewBootPanel(groups []PanelGroup) *BootPanel {
	labels := make([]string, len(groups))
	maxCols := 0
	for i, g := range groups {
		labels[i] = g.Label
		if len(g.Steps) > maxCols {
			maxCols = len(g.Steps)
		}
	}

	grid := streak.NewGrid(labels, maxCols)
	for i, g := range groups {
		_ = grid.SetRowCols(i, len(g.Steps))
	}
	theme := streak.DefaultTheme()
	theme.Layout = streak.RowLayout

	opts := []streak.Option{streak.WithTheme(theme)}
	if LogPlainText || Verbose {
		opts = append(opts, streak.WithPlain(true))
	}

	groupIdx := make(map[string]int, len(groups))
	stepIdx := make(map[string]int)
	for i, g := range groups {
		groupIdx[g.Label] = i
		for j, s := range g.Steps {
			stepIdx[stepKey(g.Label, s)] = j
		}
	}

	loader := streak.NewLoader(grid, opts...)
	loader.Start()

	return &BootPanel{
		loader:   loader,
		groups:   groups,
		groupIdx: groupIdx,
		stepIdx:  stepIdx,
	}
}

// resolve returns (row, col) for a step name. It searches all groups for
// the step name when group is empty (backward compat with Phaser callers).
func (p *BootPanel) resolve(group, step string) (int, int, bool) {
	if group != "" {
		row, ok := p.groupIdx[group]
		if !ok {
			return 0, 0, false
		}
		col, ok := p.stepIdx[stepKey(group, step)]
		if !ok {
			return 0, 0, false
		}
		return row, col, true
	}
	// No group specified: scan all groups for the step name.
	for _, g := range p.groups {
		if row, ok := p.groupIdx[g.Label]; ok {
			if col, ok := p.stepIdx[stepKey(g.Label, step)]; ok {
				return row, col, true
			}
		}
	}
	return 0, 0, false
}

func (p *BootPanel) setRunning(step, msg string) {
	if r, c, ok := p.resolve("", step); ok {
		_ = p.loader.Set(r, c, streak.Running)
		_ = p.loader.RowMessage(r, msg, streak.Running)
	}
}

func (p *BootPanel) setDone(step, detail string) {
	if r, c, ok := p.resolve("", step); ok {
		_ = p.loader.Set(r, c, streak.Done)
		msg := step
		if detail != "" {
			msg = step + " : " + detail
		}
		_ = p.loader.RowMessage(r, msg, streak.Done)
	}
}

func (p *BootPanel) setFail(step, errMsg string) {
	if r, c, ok := p.resolve("", step); ok {
		short := step + ": " + errMsg
		if tag, _, ok := strings.Cut(errMsg, " — "); ok {
			short = step + ": " + tag
		}
		_ = p.loader.Fail(r, c, short)
	}
}

func (p *BootPanel) setWarn(step, detail string) {
	if r, c, ok := p.resolve("", step); ok {
		short := step
		long := step
		if detail != "" {
			if tag, _, ok := strings.Cut(detail, " — "); ok {
				short = step + ": " + tag
			} else {
				short = step + ": " + detail
			}
			long = step + " : " + detail
		}
		_ = p.loader.Warn(r, c, short)
		_ = p.loader.RowMessage(r, long, streak.Warning)
	}
}

// Phase runs fn under the named step. Sets the dot to Running while fn
// executes, then Done on success or Error on failure.
func (p *BootPanel) Phase(name string, fn func() error) error {
	p.setRunning(name, name)
	if err := fn(); err != nil {
		p.setFail(name, err.Error())
		return err
	}
	p.setDone(name, "")
	return nil
}

// PhaseDetailed is like Phase but the callback returns a detail string.
func (p *BootPanel) PhaseDetailed(name string, fn func() (string, error)) error {
	p.setRunning(name, name)
	detail, err := fn()
	if err != nil {
		p.setFail(name, err.Error())
		return err
	}
	p.setDone(name, detail)
	return nil
}

// PhaseDetailedWarn is like PhaseDetailed but the callback also returns a
// warn bool. When true, the dot renders as Warning instead of Done.
func (p *BootPanel) PhaseDetailedWarn(name string, fn func() (string, bool, error)) error {
	p.setRunning(name, name)
	detail, warn, err := fn()
	if err != nil {
		p.setFail(name, err.Error())
		return err
	}
	if warn {
		p.setWarn(name, detail)
	} else {
		p.setDone(name, detail)
	}
	return nil
}

// PhaseDetailedRunning shows a different label while running vs the final row.
func (p *BootPanel) PhaseDetailedRunning(running, finalName string, fn func() (string, error)) error {
	p.setRunning(finalName, running)
	detail, err := fn()
	if err != nil {
		p.setFail(finalName, err.Error())
		return err
	}
	p.setDone(finalName, detail)
	return nil
}

// PhaseDetailedRunningWarn combines PhaseDetailedRunning and PhaseDetailedWarn.
func (p *BootPanel) PhaseDetailedRunningWarn(running, finalName string, fn func() (string, bool, error)) error {
	p.setRunning(finalName, running)
	detail, warn, err := fn()
	if err != nil {
		p.setFail(finalName, err.Error())
		return err
	}
	if warn {
		p.setWarn(finalName, detail)
	} else {
		p.setDone(finalName, detail)
	}
	return nil
}

// UpdateText changes the footer message while a phase is running.
func (p *BootPanel) UpdateText(message string) {
	p.loader.Message(message, streak.Running)
}

// Seal marks a step as Done without running work.
func (p *BootPanel) Seal(name string) {
	p.setDone(name, "")
}

// Message sets the footer message text.
func (p *BootPanel) Message(text string, s streak.Status) {
	p.loader.Message(text, s)
}

// SetBoot updates a boot event step by its group and step name.
func (p *BootPanel) SetBoot(group, step string, s streak.Status, title string) {
	if r, c, ok := p.resolve(group, step); ok {
		_ = p.loader.Set(r, c, s)
		if title != "" {
			_ = p.loader.RowMessage(r, title, s)
		}
	}
}

// WarnBoot shows a warning in the footer (for warnings not tied to a step).
func (p *BootPanel) WarnBoot(msg string) {
	p.loader.Message(msg, streak.Warning)
}

// Snapshot returns a copy of the underlying grid for inspection (tests).
func (p *BootPanel) Snapshot() *streak.Grid {
	return p.loader.Snapshot()
}

// PromoteRunning sets every Running dot to Done. Call when an external
// signal (e.g. boot.ready) confirms all services are up, even if some
// never emitted their own "up" event (longruns that exec into a daemon).
func (p *BootPanel) PromoteRunning() {
	snap := p.loader.Snapshot()
	for r := 0; r < snap.Rows(); r++ {
		for c := 0; c < snap.RowCols(r); c++ {
			if s, _ := snap.Get(r, c); s == streak.Running {
				_ = p.loader.Set(r, c, streak.Done)
			}
		}
	}
}

// Finish stops the Loader and draws the final frame.
func (p *BootPanel) Finish(text string) {
	p.loader.Finish(text, streak.Done)
}

// FinishError stops the Loader with an error status.
func (p *BootPanel) FinishError(text string) {
	p.loader.Finish(text, streak.Error)
}

// SaveCursor emits DEC save-cursor (\x1b7) to stderr. Call before the
// banner so Clear can restore to that position and erase everything the
// panel wrote. No-op in plain/verbose mode.
func SaveCursor() {
	if !LogPlainText && !Verbose {
		fmt.Fprintf(os.Stderr, "\x1b7")
	}
}

// Clear restores the cursor to the position saved by SaveCursor and
// erases from there to the end of the screen, removing the banner and
// panel output. No-op in plain/verbose mode.
func (p *BootPanel) Clear() {
	if LogPlainText || Verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "\x1b8\x1b[J")
}

// ClearBelowGroups restores the saved cursor, moves past the banner,
// group rows and rule, then erases everything below (issue lines,
// footer message). The agent starts on a clean line right after the
// rule. No-op in plain/verbose mode.
func (p *BootPanel) ClearBelowGroups() {
	if LogPlainText || Verbose {
		return
	}
	n := len(p.groups)
	fmt.Fprintf(os.Stderr, "\x1b8\x1b[%dB\x1b[J", n)
}

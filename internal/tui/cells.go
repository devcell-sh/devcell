package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devcell-sh/devcell/internal/engine/docker"
	"github.com/devcell-sh/devcell/internal/ux"
)

// cellsModel renders the running cells table.
type cellsModel struct {
	data   *scanResultMsg
	width  int
	height int
}

func (m cellsModel) View() string {
	if m.data == nil {
		return "  No scan data available."
	}

	var b strings.Builder
	sectionStyle := lipgloss.NewStyle().Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))

	cells := m.data.cells.Cells
	stats := m.data.cells.Stats

	if m.data.cells.CellsErr != nil {
		b.WriteString(fmt.Sprintf("  %s Failed to list cells: %v\n",
			ux.StyleWarning.Render("⚠"), m.data.cells.CellsErr))
		return b.String()
	}

	if len(cells) == 0 {
		b.WriteString("  " + mutedStyle.Render("No running cells detected") + "\n")
		return b.String()
	}

	// Build stats lookup by container name
	statsMap := make(map[string]docker.CellStats, len(stats))
	for _, s := range stats {
		statsMap[s.Name] = s
	}

	// Build profile hash lookup from live closures
	profileMap := make(map[string]string)
	if m.data.build.LiveClosures != nil {
		for _, lc := range m.data.build.LiveClosures {
			if h := docker.StoreHash(lc.ProfilePath); h != "" {
				profileMap[lc.Container] = h
			}
		}
	}

	// Detect own cell
	ownCellID := os.Getenv("DEVCELL_CELL_ID")

	// Header
	b.WriteString(fmt.Sprintf("  %s\n\n", sectionStyle.Render("Running Cells")))
	b.WriteString(fmt.Sprintf("  %-24s %-10s %-12s %-10s %-12s %-6s %-8s\n",
		mutedStyle.Render("CELL"),
		mutedStyle.Render("WORKSPACE"),
		mutedStyle.Render("STACK"),
		mutedStyle.Render("PROFILE"),
		mutedStyle.Render("UPTIME"),
		mutedStyle.Render("CPU"),
		mutedStyle.Render("MEM"),
	))

	for _, c := range cells {
		status := ux.StyleSuccess.Render("●")
		if c.Status != "running" {
			status = mutedStyle.Render("○")
		}

		cellName := c.Name
		isSelf := false
		if ownCellID != "" && c.CellID == ownCellID {
			isSelf = true
			cellName += " (self)"
		}

		cpu := "-"
		mem := "-"
		if st, ok := statsMap[c.Name]; ok {
			cpu = fmt.Sprintf("%.1f%%", st.CPUPercent)
			mem = st.MemUsage
		}

		stack := c.Stack
		if stack == "" {
			stack = "-"
		}

		profile := "-"
		if h, ok := profileMap[c.Name]; ok && len(h) >= 8 {
			profile = h[:8]
		} else if h != "" {
			profile = h
		}

		line := fmt.Sprintf("  %s %-24s %-10s %-12s %-10s %-12s %-6s %-8s",
			status, cellName, c.Workspace, stack, profile, c.Uptime, cpu, mem)

		if isSelf {
			line = lipgloss.NewStyle().Foreground(ux.StyleInfo.GetForeground()).Render(line)
		}
		b.WriteString(line + "\n")
	}

	if len(m.data.build.Stale) > 0 {
		b.WriteString(fmt.Sprintf("\n  %s %d stale container(s) detected\n",
			ux.StyleWarning.Render("⚠"), len(m.data.build.Stale)))
		for _, s := range m.data.build.Stale {
			b.WriteString(fmt.Sprintf("    %s %s (%s)\n",
				mutedStyle.Render("○"), s.Name, s.Uptime))
		}
	}

	b.WriteString("\n")
	b.WriteString("  " + mutedStyle.Render("[s]top cell (v2)  [R]efresh  [esc] back") + "\n")

	return b.String()
}

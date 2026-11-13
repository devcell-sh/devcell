package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/dustin/go-humanize"
)

// diskRow is one entry in the hierarchical disk view.
type diskRow struct {
	label    string
	bytes    int64
	indent   int
	children []diskRow
	expanded bool
	detail   string // extra info shown after the size
}

// diskModel renders the hierarchical disk storage screen.
type diskModel struct {
	data     *scanResultMsg
	width    int
	height   int
	cursor   int
	rows     []diskRow // flattened visible rows
	allRows  []diskRow // top-level rows (tree roots)
}

func (m *diskModel) rebuild() {
	if m.data == nil {
		m.rows = nil
		return
	}

	du := m.data.build.DiskUsage
	nix := m.data.nix

	var topRows []diskRow

	// Volumes
	volRow := diskRow{label: "Volumes", bytes: parseSize(du.Volumes.Size), detail: du.Volumes.Size}
	if nix.NixOK && nix.NixReport.VolumeBytes > 0 {
		nixChild := diskRow{
			label:  "nix store",
			bytes:  nix.NixReport.VolumeBytes,
			indent: 1,
			detail: "(filesystem)",
		}
		for _, tp := range nix.NixReport.TopPaths {
			nixChild.children = append(nixChild.children, diskRow{
				label:  tp.Name,
				bytes:  tp.Bytes,
				indent: 2,
			})
		}
		volRow.children = append(volRow.children, nixChild)
	}
	topRows = append(topRows, volRow)

	// Images
	topRows = append(topRows, diskRow{label: "Images", bytes: parseSize(du.Images.Size), detail: du.Images.Size})

	// Containers
	topRows = append(topRows, diskRow{label: "Containers", bytes: parseSize(du.Containers.Size), detail: du.Containers.Size})

	// Build cache
	topRows = append(topRows, diskRow{label: "Build cache", bytes: parseSize(du.BuildCache.Size), detail: du.BuildCache.Size})

	// Host caches
	if m.data.cache.TotalBytes > 0 {
		topRows = append(topRows, diskRow{
			label:  "Caches (host)",
			bytes:  m.data.cache.TotalBytes,
			detail: "(host filesystem)",
		})
	}

	// Nix profiles (if nix data available)
	if nix.NixOK && len(nix.NixReport.Metas) > 0 {
		profileRow := diskRow{label: "Nix profiles", indent: 0}
		var totalClosure int64
		for _, meta := range nix.NixReport.Metas {
			child := diskRow{
				label:  fmt.Sprintf("%s (cell=%s)", meta.Hash[:min(8, len(meta.Hash))], meta.Cell),
				bytes:  meta.ClosureBytes,
				indent: 1,
				detail: fmt.Sprintf("stack=%s", meta.Stack),
			}
			totalClosure += meta.ClosureBytes
			profileRow.children = append(profileRow.children, child)
		}
		profileRow.bytes = totalClosure
		profileRow.detail = "(logical, overlapping)"
		topRows = append(topRows, profileRow)
	}

	m.allRows = topRows
	m.rows = flattenRows(topRows)
	if m.cursor >= len(m.rows) {
		m.cursor = max(0, len(m.rows)-1)
	}
}

// flattenRows produces the visible list from the tree, respecting expanded state.
func flattenRows(rows []diskRow) []diskRow {
	var out []diskRow
	for i := range rows {
		out = append(out, rows[i])
		if rows[i].expanded && len(rows[i].children) > 0 {
			out = append(out, flattenRows(rows[i].children)...)
		}
	}
	return out
}

// toggleCursor expands or collapses the row at the cursor position.
func (m *diskModel) toggleCursor() {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	target := m.rows[m.cursor]
	if len(target.children) == 0 {
		return
	}
	// Walk the tree and find the matching row by label+indent, toggle it.
	toggleInTree(m.allRows, target.label, target.indent)
	m.rows = flattenRows(m.allRows)
}

func toggleInTree(rows []diskRow, label string, indent int) bool {
	for i := range rows {
		if rows[i].label == label && rows[i].indent == indent {
			rows[i].expanded = !rows[i].expanded
			return true
		}
		if rows[i].expanded && toggleInTree(rows[i].children, label, indent) {
			return true
		}
	}
	return false
}

func (m diskModel) handlesKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "j", "k", "up", "down", "enter", " ":
		return true
	}
	return false
}

func (m diskModel) update(msg tea.KeyMsg) diskModel {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter", " ":
		m.toggleCursor()
	}
	return m
}

func (m diskModel) View() string {
	if m.data == nil {
		return "  No scan data available."
	}

	var b strings.Builder
	sectionStyle := lipgloss.NewStyle().Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(ux.StyleInfo.GetForeground())

	// VM disk header
	if m.data.docker.VMDiskOK {
		vm := m.data.docker.VMDisk
		pct := float64(0)
		if vm.TotalBytes > 0 {
			pct = float64(vm.UsedBytes) / float64(vm.TotalBytes)
		}
		b.WriteString(fmt.Sprintf("  %s Docker VM: %s used / %s total (%d%%)\n",
			sectionStyle.Render(""),
			humanize.IBytes(uint64(vm.UsedBytes)),
			humanize.IBytes(uint64(vm.TotalBytes)),
			int(pct*100),
		))
		barWidth := min(40, m.width-6)
		if barWidth > 0 {
			b.WriteString("  " + renderBar(pct, barWidth, lipgloss.Color("#E85D26")) + "\n")
		}
		b.WriteString("\n")
	}

	// Category rows
	maxBytes := int64(0)
	for _, r := range m.rows {
		if r.indent == 0 && r.bytes > maxBytes {
			maxBytes = r.bytes
		}
	}

	barWidth := min(20, m.width-50)
	if barWidth < 4 {
		barWidth = 4
	}

	for i, r := range m.rows {
		indent := strings.Repeat("  ", r.indent)
		prefix := "  "
		if len(r.children) > 0 {
			if r.expanded {
				prefix = "▾ "
			} else {
				prefix = "▸ "
			}
		} else if r.indent > 0 {
			prefix = "  "
		}

		label := fmt.Sprintf("%s%s%s", indent, prefix, r.label)
		sizeStr := ""
		if r.bytes > 0 {
			sizeStr = humanize.IBytes(uint64(r.bytes))
		}

		// Build bar for top-level rows
		bar := ""
		if r.indent == 0 && maxBytes > 0 && r.bytes > 0 {
			frac := float64(r.bytes) / float64(maxBytes)
			bar = renderBar(frac, barWidth, lipgloss.Color("#E85D26"))
		}

		line := fmt.Sprintf("  %-30s %8s  %s", label, sizeStr, bar)
		if r.detail != "" && r.indent > 0 {
			line += " " + mutedStyle.Render(r.detail)
		}

		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}

	if m.data.docker.VMDiskOK && m.data.docker.VMDisk.AvailBytes > 0 {
		b.WriteString(fmt.Sprintf("\n  Free: %s\n",
			humanize.IBytes(uint64(m.data.docker.VMDisk.AvailBytes))))
	}

	b.WriteString("\n")
	b.WriteString("  " + mutedStyle.Render("[enter] expand  [j/k] navigate  [R]efresh  [esc] back") + "\n")

	return b.String()
}

// parseSize converts Docker's human-readable size strings to bytes (best effort).
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0B" {
		return 0
	}
	// Try common Docker formats: "11.5GB", "200MB", "1.1kB"
	var val float64
	var unit string
	_, err := fmt.Sscanf(s, "%f%s", &val, &unit)
	if err != nil {
		return 0
	}
	unit = strings.ToUpper(strings.TrimSpace(unit))
	switch unit {
	case "B":
		return int64(val)
	case "KB":
		return int64(val * 1000)
	case "MB":
		return int64(val * 1000 * 1000)
	case "GB":
		return int64(val * 1000 * 1000 * 1000)
	case "TB":
		return int64(val * 1000 * 1000 * 1000 * 1000)
	case "KIB":
		return int64(val * 1024)
	case "MIB":
		return int64(val * 1024 * 1024)
	case "GIB":
		return int64(val * 1024 * 1024 * 1024)
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

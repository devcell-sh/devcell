package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/dustin/go-humanize"
)

// overviewModel renders the summary landing page.
type overviewModel struct {
	data   *scanResultMsg
	width  int
	height int
}

func (m overviewModel) View() string {
	if m.data == nil {
		return "  No scan data available."
	}

	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().Bold(true)
	labelStyle := lipgloss.NewStyle().Width(22)
	valueStyle := lipgloss.NewStyle().Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))

	// Disk Usage section
	b.WriteString("  " + sectionStyle.Render("Disk Usage") + "\n")
	du := m.data.build.DiskUsage
	if du.Volumes.Size != "" {
		b.WriteString(fmt.Sprintf("  %s%s\n",
			labelStyle.Render("Volumes"),
			valueStyle.Render(du.Volumes.Size),
		))
	}
	if du.Images.Size != "" {
		b.WriteString(fmt.Sprintf("  %s%s\n",
			labelStyle.Render("Images"),
			valueStyle.Render(du.Images.Size),
		))
	}
	if du.Containers.Size != "" {
		b.WriteString(fmt.Sprintf("  %s%s\n",
			labelStyle.Render("Containers"),
			valueStyle.Render(du.Containers.Size),
		))
	}
	if du.BuildCache.Size != "" {
		b.WriteString(fmt.Sprintf("  %s%s\n",
			labelStyle.Render("Build cache"),
			valueStyle.Render(du.BuildCache.Size),
		))
	}
	if m.data.nix.NixOK && m.data.nix.NixReport.VolumeBytes > 0 {
		b.WriteString(fmt.Sprintf("  %s%s (filesystem)\n",
			labelStyle.Render("Nix store"),
			valueStyle.Render(humanize.IBytes(uint64(m.data.nix.NixReport.VolumeBytes))),
		))
		b.WriteString(fmt.Sprintf("  %s%d\n",
			labelStyle.Render("Nix profiles"),
			len(m.data.nix.NixReport.Metas),
		))
	}
	if m.data.cache.TotalBytes > 0 {
		b.WriteString(fmt.Sprintf("  %s%s\n",
			labelStyle.Render("Host caches"),
			valueStyle.Render(humanize.IBytes(uint64(m.data.cache.TotalBytes))),
		))
	}

	b.WriteString("\n")

	// Running Cells section
	b.WriteString("  " + sectionStyle.Render("Running Cells") + "\n")
	if len(du.RunningCells) == 0 {
		b.WriteString("  " + mutedStyle.Render("No running cells detected") + "\n")
	} else {
		for _, c := range du.RunningCells {
			status := ux.StyleSuccess.Render("●")
			b.WriteString(fmt.Sprintf("  %s %-24s %s\n", status, c.Name, mutedStyle.Render(c.Uptime)))
		}
	}

	b.WriteString("\n")

	// Health section
	b.WriteString("  " + sectionStyle.Render("Health") + "\n")
	if m.data.nix.NixOK {
		b.WriteString("  " + ux.StyleSuccess.Render("✓") + " nix store ok\n")
	} else {
		b.WriteString("  " + ux.StyleWarning.Render("⚠") + " nix store unavailable\n")
	}
	if len(m.data.build.Stale) > 0 {
		b.WriteString(fmt.Sprintf("  %s %d stale container(s)\n",
			ux.StyleWarning.Render("⚠"),
			len(m.data.build.Stale),
		))
	} else {
		b.WriteString("  " + ux.StyleSuccess.Render("✓") + " no stale containers\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + mutedStyle.Render(fmt.Sprintf("Scanned at %s", m.data.scannedAt.Format("15:04:05"))) + "\n")

	return b.String()
}

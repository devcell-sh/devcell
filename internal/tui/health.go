package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devcell-sh/devcell/internal/ux"
	"github.com/dustin/go-humanize"
)

// healthModel renders the system health screen.
type healthModel struct {
	data   *scanResultMsg
	width  int
	height int
}

func (m healthModel) View() string {
	if m.data == nil {
		return "  No scan data available."
	}

	var b strings.Builder
	sectionStyle := lipgloss.NewStyle().Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	labelStyle := lipgloss.NewStyle().Width(28)

	ok := ux.StyleSuccess.Render("✓")
	warn := ux.StyleWarning.Render("⚠")

	// Nix Store Health
	b.WriteString(fmt.Sprintf("  %s\n\n", sectionStyle.Render("Nix Store")))

	if !m.data.nix.NixOK {
		b.WriteString(fmt.Sprintf("  %s nix store unavailable\n", warn))
	} else {
		h := m.data.nix.NixReport.Health
		b.WriteString(fmt.Sprintf("  %s nix store accessible\n", ok))

		// Roots
		if h.StaleRoots > 0 {
			b.WriteString(fmt.Sprintf("  %s %s%d stale / %d total roots\n",
				warn, labelStyle.Render("GC roots:"), h.StaleRoots, h.TotalRoots))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s%d roots, none stale\n",
				ok, labelStyle.Render("GC roots:"), h.TotalRoots))
		}

		// Profile hashes (drift)
		if h.ProfileHashes > 1 {
			b.WriteString(fmt.Sprintf("  %s %s%d distinct hashes (config/lock drift)\n",
				warn, labelStyle.Render("Profile hashes:"), h.ProfileHashes))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s%d hash\n",
				ok, labelStyle.Render("Profile hashes:"), h.ProfileHashes))
		}

		// Generations
		if h.OrphanedGenerations > 0 {
			b.WriteString(fmt.Sprintf("  %s %s%d orphaned / %d total (reclaimable)\n",
				warn, labelStyle.Render("Generations:"), h.OrphanedGenerations, h.Generations))
		} else if h.Generations > 0 {
			b.WriteString(fmt.Sprintf("  %s %s%d generations, none orphaned\n",
				ok, labelStyle.Render("Generations:"), h.Generations))
		}

		// Nixpkgs rev drift
		if h.DistinctRevs > 1 {
			b.WriteString(fmt.Sprintf("  %s %s%d distinct nixpkgs revs\n",
				warn, labelStyle.Render("Nixpkgs revisions:"), h.DistinctRevs))
		} else if h.DistinctRevs == 1 {
			b.WriteString(fmt.Sprintf("  %s %sall cells on same rev\n",
				ok, labelStyle.Render("Nixpkgs revisions:")))
		}

		// Nix volume size
		if m.data.nix.NixReport.VolumeBytes > 0 {
			b.WriteString(fmt.Sprintf("  %s %s%s\n",
				ok, labelStyle.Render("Volume (filesystem):"),
				humanize.IBytes(uint64(m.data.nix.NixReport.VolumeBytes))))
		}
	}

	b.WriteString("\n")

	// Docker Health
	b.WriteString(fmt.Sprintf("  %s\n\n", sectionStyle.Render("Docker")))

	du := m.data.build.DiskUsage
	if du.Images.Size != "" || du.Volumes.Size != "" {
		b.WriteString(fmt.Sprintf("  %s Docker daemon responding\n", ok))
	} else {
		b.WriteString(fmt.Sprintf("  %s Docker daemon not responding or no data\n", warn))
	}

	if m.data.docker.VMDiskOK {
		vm := m.data.docker.VMDisk
		pct := float64(0)
		if vm.TotalBytes > 0 {
			pct = float64(vm.UsedBytes) / float64(vm.TotalBytes) * 100
		}
		vmIcon := ok
		if pct > 85 {
			vmIcon = warn
		}
		b.WriteString(fmt.Sprintf("  %s %s%.0f%% used (%s / %s)\n",
			vmIcon, labelStyle.Render("VM disk:"),
			pct, humanize.IBytes(uint64(vm.UsedBytes)), humanize.IBytes(uint64(vm.TotalBytes))))
	}

	// Stale cells
	if len(m.data.build.Stale) > 0 {
		b.WriteString(fmt.Sprintf("  %s %s%d stale containers\n",
			warn, labelStyle.Render("Stale cells:"), len(m.data.build.Stale)))
	} else {
		b.WriteString(fmt.Sprintf("  %s no stale containers\n", ok))
	}

	// Closure scan errors
	if m.data.build.ClosureErr != nil {
		b.WriteString(fmt.Sprintf("  %s %sclosure scan failed: %v\n",
			warn, labelStyle.Render("Live closures:"), m.data.build.ClosureErr))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s%d live closures tracked\n",
			ok, labelStyle.Render("Live closures:"), len(m.data.build.LiveClosures)))
	}

	b.WriteString("\n")

	// Cleanup actions (v2 placeholder)
	b.WriteString(fmt.Sprintf("  %s\n\n", sectionStyle.Render("Cleanup")))
	b.WriteString("  " + mutedStyle.Render("Broad cleanup actions coming in v2.") + "\n")
	b.WriteString("  " + mutedStyle.Render("Use `cell clean` for interactive cleanup.") + "\n")

	b.WriteString("\n")
	b.WriteString("  " + mutedStyle.Render("[R]efresh  [esc] back") + "\n")

	return b.String()
}


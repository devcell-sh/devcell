package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	barFull  = '█'
	barHalf  = '▌'
	barLight = '░'
)

// renderBar draws a proportional bar of the given width.
// fraction is clamped to [0,1].
func renderBar(fraction float64, width int, fg lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	filled := fraction * float64(width)
	fullBlocks := int(filled)
	remainder := filled - float64(fullBlocks)

	var b strings.Builder
	b.Grow(width * 4)

	style := lipgloss.NewStyle().Foreground(fg)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3b3b3b"))

	if fullBlocks > 0 {
		b.WriteString(style.Render(strings.Repeat(string(barFull), fullBlocks)))
	}
	if fullBlocks < width {
		if remainder >= 0.5 {
			b.WriteString(style.Render(string(barHalf)))
			fullBlocks++
		}
		if fullBlocks < width {
			b.WriteString(emptyStyle.Render(strings.Repeat(string(barLight), width-fullBlocks)))
		}
	}
	return b.String()
}

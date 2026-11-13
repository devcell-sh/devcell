package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devcell-sh/devcell/internal/scan"
	"github.com/devcell-sh/devcell/internal/ux"
)

// screen identifies which screen the TUI is showing.
type screen int

const (
	screenOverview screen = iota
	screenDisk
	screenCells
	screenHealth
)

// scanResultMsg carries completed scan data into the TUI.
type scanResultMsg struct {
	cache     scan.CacheResult
	docker    scan.DockerResult
	nix       scan.NixResult
	build     scan.BuildResult
	cells     scan.CellsResult
	scannedAt time.Time
}

// doctorModel is the top-level tea.Model that owns scan results and
// delegates to the active screen.
type doctorModel struct {
	active    screen
	width     int
	height    int
	spinner   spinner.Model
	scanning  bool
	scanData  *scanResultMsg
	overview  overviewModel
	disk      diskModel
	cells     cellsModel
	health    healthModel
}

func newDoctorModel() doctorModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ux.StyleInfo.GetForeground())
	return doctorModel{
		active:   screenOverview,
		spinner:  s,
		scanning: true,
	}
}

func (m doctorModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doScan())
}

func (m doctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.overview.width = msg.Width
		m.overview.height = msg.Height
		m.disk.width = msg.Width
		m.disk.height = msg.Height
		m.cells.width = msg.Width
		m.cells.height = msg.Height
		m.health.width = msg.Width
		m.health.height = msg.Height
		return m, nil

	case scanResultMsg:
		m.scanning = false
		m.scanData = &msg
		m.overview.data = &msg
		m.disk.data = &msg
		m.disk.rebuild()
		m.cells.data = &msg
		m.health.data = &msg
		return m, nil

	case tea.KeyMsg:
		// Global keys first
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "R":
			if !m.scanning {
				m.scanning = true
				return m, tea.Batch(m.spinner.Tick, doScan())
			}
			return m, nil
		}

		// Screen-local keys: if the active screen handles the key, don't switch tabs
		if !m.scanning {
			switch m.active {
			case screenDisk:
				if m.disk.handlesKey(msg) {
					m.disk = m.disk.update(msg)
					return m, nil
				}
			}
		}

		// Tab switching
		switch msg.String() {
		case "o", "1":
			m.active = screenOverview
			return m, nil
		case "d", "2":
			m.active = screenDisk
			return m, nil
		case "c", "3":
			m.active = screenCells
			return m, nil
		case "h", "4":
			m.active = screenHealth
			return m, nil
		}

	case spinner.TickMsg:
		if m.scanning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m doctorModel) View() string {
	if m.width == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ux.StyleInfo.GetForeground())

	title := titleStyle.Render("cell doctor")

	tabs := renderTabs(m.active)
	header := title + "  " + tabs

	if m.scanning {
		return header + "\n\n  " + m.spinner.View() + " Scanning…\n"
	}

	var body string
	switch m.active {
	case screenOverview:
		body = m.overview.View()
	case screenDisk:
		body = m.disk.View()
	case screenCells:
		body = m.cells.View()
	case screenHealth:
		body = m.health.View()
	}

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8b949e")).
		Render("  [o]verview  [d]isk  [c]ells  [h]ealth  [R]efresh  [q]uit")

	return header + "\n\n" + body + "\n\n" + help + "\n"
}

func renderTabs(active screen) string {
	names := []struct {
		s    screen
		label string
	}{
		{screenOverview, "Overview"},
		{screenDisk, "Disk"},
		{screenCells, "Cells"},
		{screenHealth, "Health"},
	}
	activeStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))

	var parts []string
	for _, n := range names {
		if n.s == active {
			parts = append(parts, activeStyle.Render(n.label))
		} else {
			parts = append(parts, inactiveStyle.Render(n.label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, joinWith(parts, "  ")...)
}

func joinWith(parts []string, sep string) []string {
	if len(parts) == 0 {
		return nil
	}
	result := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		if i > 0 {
			result = append(result, sep)
		}
		result = append(result, p)
	}
	return result
}

func doScan() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()

		cache := scan.Caches(true)
		dockerResult := scan.Docker(ctx)
		nixResult := scan.Nix(ctx)
		buildResult := scan.MergeBuild(dockerResult, nixResult)
		cellsResult := scan.Cells(ctx)

		return scanResultMsg{
			cache:     cache,
			docker:    dockerResult,
			nix:       nixResult,
			build:     buildResult,
			cells:     cellsResult,
			scannedAt: time.Now(),
		}
	}
}

// RunDoctor launches the full-screen TUI.
func RunDoctor() error {
	p := tea.NewProgram(
		newDoctorModel(),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("doctor TUI: %w", err)
	}
	return nil
}

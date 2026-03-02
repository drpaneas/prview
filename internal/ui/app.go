package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/drpaneas/prview/internal/model"
)

type App struct {
	report  *model.PRReport
	content string
	lines   []string
	offset  int
	height  int
	width   int
	ready   bool
}

func New(report *model.PRReport) *App {
	return &App{report: report}
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height - 2 // room for help bar
		a.content = renderReport(a.report, a.width)
		a.lines = strings.Split(a.content, "\n")
		a.ready = true
		return a, nil

	case tea.KeyMsg:
		action := parseKey(msg)
		switch action {
		case keyQuit:
			return a, tea.Quit
		case keyUp:
			if a.offset > 0 {
				a.offset--
			}
		case keyDown:
			if a.offset < a.maxOffset() {
				a.offset++
			}
		case keyPageUp:
			a.offset -= a.height / 2
			if a.offset < 0 {
				a.offset = 0
			}
		case keyPageDown:
			a.offset += a.height / 2
			if a.offset > a.maxOffset() {
				a.offset = a.maxOffset()
			}
		case keyTop:
			a.offset = 0
		case keyBottom:
			a.offset = a.maxOffset()
		}
	}

	return a, nil
}

func (a *App) View() string {
	if !a.ready {
		return "Loading..."
	}

	end := a.offset + a.height
	if end > len(a.lines) {
		end = len(a.lines)
	}

	visible := a.lines[a.offset:end]
	view := strings.Join(visible, "\n")

	scrollInfo := fmt.Sprintf(" %d/%d ", a.offset+1, len(a.lines))
	percent := ""
	if a.maxOffset() > 0 {
		pct := float64(a.offset) / float64(a.maxOffset()) * 100
		percent = fmt.Sprintf("%.0f%%", pct)
	} else {
		percent = "100%"
	}

	help := helpStyle.Render(
		fmt.Sprintf("  j/k: scroll  |  f/b: page  |  g/G: top/bottom  |  q: quit  |  %s  %s",
			scrollInfo, dimText.Render(percent)),
	)

	return lipgloss.JoinVertical(lipgloss.Left, view, help)
}

func (a *App) maxOffset() int {
	max := len(a.lines) - a.height
	if max < 0 {
		return 0
	}
	return max
}

func Run(report *model.PRReport) error {
	app := New(report)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

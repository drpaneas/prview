package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen  = lipgloss.Color("42")
	colorYellow = lipgloss.Color("214")
	colorRed    = lipgloss.Color("196")
	colorBlue   = lipgloss.Color("39")
	colorDim    = lipgloss.Color("243")
	colorWhite  = lipgloss.Color("255")
	colorCyan   = lipgloss.Color("87")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	headerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 2)

	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorCyan).
				MarginTop(1)

	greenText  = lipgloss.NewStyle().Foreground(colorGreen)
	yellowText = lipgloss.NewStyle().Foreground(colorYellow)
	redText    = lipgloss.NewStyle().Foreground(colorRed)
	blueText   = lipgloss.NewStyle().Foreground(colorBlue)
	dimText    = lipgloss.NewStyle().Foreground(colorDim)
	boldText   = lipgloss.NewStyle().Bold(true)

	barFull  = lipgloss.NewStyle().Foreground(colorBlue)
	barEmpty = lipgloss.NewStyle().Foreground(colorDim)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			MarginTop(1)
)

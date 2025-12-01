package queuepanel

import "github.com/charmbracelet/lipgloss"

const (
	playingSymbol  = "\u25B6" // ▶
	selectedSymbol = "●"      // filled circle for selected
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	trackStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	playingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")). // cyan/blue
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

	dimmedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	multiSelectHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")) // cyan to indicate mode
)

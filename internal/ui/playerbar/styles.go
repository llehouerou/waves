package playerbar

import "github.com/charmbracelet/lipgloss"

// Player status symbols
const (
	playSymbol  = "▶"
	pauseSymbol = "⏸"
)

var barStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("240"))

var expandedBarStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("240")).
	Padding(0, 2) // horizontal padding

// Text styles for expanded view
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	artistStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	progressTimeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	progressBarFilled = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")) // cyan/blue

	progressBarEmpty = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))
)

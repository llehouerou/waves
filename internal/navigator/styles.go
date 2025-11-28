package navigator

import "github.com/charmbracelet/lipgloss"

var selectionStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("236")).
	Foreground(lipgloss.Color("252")).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("240")).
	BorderTop(false).
	BorderBottom(false)

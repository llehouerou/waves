package styles

import "github.com/charmbracelet/lipgloss"

var (
	unfocusedBorderColor = lipgloss.Color("240")
	focusedBorderColor   = lipgloss.Color("39") // cyan/blue

	unfocusedPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(unfocusedBorderColor)

	focusedPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(focusedBorderColor)
)

// PanelStyle returns the appropriate panel style based on focus state.
func PanelStyle(focused bool) lipgloss.Style {
	if focused {
		return focusedPanelStyle
	}
	return unfocusedPanelStyle
}

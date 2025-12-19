package styles

import "github.com/charmbracelet/lipgloss"

// PanelStyle returns the appropriate panel style based on focus state.
func PanelStyle(focused bool) lipgloss.Style {
	t := T()
	borderColor := t.Border
	if focused {
		borderColor = t.BorderFocus
	}
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Foreground(t.FgBase)
}

package navigator

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/styles"
)

func selectionStyle() lipgloss.Style {
	t := styles.T()
	return lipgloss.NewStyle().
		Background(t.BgCursor).
		Foreground(t.FgBase).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		BorderTop(false).
		BorderBottom(false)
}

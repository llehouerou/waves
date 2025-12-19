package playerbar

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/styles"
)

// Player status symbols
const (
	playSymbol  = "▶"
	pauseSymbol = "⏸"
)

func barStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.T().Border)
}

func expandedBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.T().Border).
		Padding(0, 2)
}

func titleStyle() lipgloss.Style {
	return styles.T().S().Title
}

func artistStyle() lipgloss.Style {
	return styles.T().S().Base
}

func metaStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func progressTimeStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func progressBarFilled() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(styles.T().Primary)
}

func progressBarEmpty() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(styles.T().FgSubtle)
}

func radioStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(styles.T().Secondary)
}

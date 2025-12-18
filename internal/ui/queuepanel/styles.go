package queuepanel

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/styles"
)

const (
	playingSymbol  = "\u25B6" // ▶
	selectedSymbol = "●"      // filled circle for selected
)

func defaultHeaderStyle() lipgloss.Style {
	return styles.T().S().Title
}

func trackStyle() lipgloss.Style {
	return styles.T().S().Base
}

func playingStyle() lipgloss.Style {
	return styles.T().S().Playing
}

func cursorStyle() lipgloss.Style {
	return styles.T().S().Cursor
}

func dimmedStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func multiSelectHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.T().Primary)
}

func modeIconStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.T().Primary).
		Bold(true)
}

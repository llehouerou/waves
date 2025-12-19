package navigator

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/styles"
)

func headerStyle() lipgloss.Style {
	return styles.T().S().Title
}

// sideColumnStyle is used for parent and preview columns (same as album name in album view).
func sideColumnStyle() lipgloss.Style {
	return styles.T().S().Muted
}

// currentColumnStyle is used for the current column (same as non-played track in queue).
func currentColumnStyle() lipgloss.Style {
	return styles.T().S().Base
}

// cursorStyle is used for the selected item in the current column.
func cursorStyle() lipgloss.Style {
	return styles.T().S().Cursor
}

// columnSeparatorStyle is used for the vertical separators between columns.
func columnSeparatorStyle() lipgloss.Style {
	return styles.T().S().Base
}

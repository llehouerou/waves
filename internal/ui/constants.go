// Package ui provides shared UI constants and utilities.
package ui

// Layout constants for consistent sizing across UI components.
const (
	// ScrollMargin is the number of items to keep visible above/below the cursor.
	ScrollMargin = 5

	// BorderHeight is the vertical space consumed by a standard panel border.
	BorderHeight = 2

	// HeaderHeight is the space for header + separator in panels.
	HeaderHeight = 2

	// PanelOverhead is the total vertical overhead (border + header + separator).
	// Used to calculate available list height: listHeight = panelHeight - PanelOverhead
	PanelOverhead = BorderHeight + HeaderHeight

	// ColumnWidthDivisor determines the width ratio between current and preview columns.
	// Current column gets 1/ColumnWidthDivisor of the width.
	ColumnWidthDivisor = 4

	// MinProgressBarWidth is the minimum width for a usable progress bar.
	MinProgressBarWidth = 5

	// MinExpandedWidth is the minimum width for expanded player bar mode.
	MinExpandedWidth = 40
)

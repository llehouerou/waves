// Package render provides text rendering utilities for TUI components.
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/llehouerou/waves/internal/ui/styles"
)

// Truncate shortens a string to fit within maxWidth, adding an ellipsis if truncated.
// Uses runewidth for proper handling of wide characters (CJK, emoji).
func Truncate(s string, maxWidth int) string {
	return runewidth.Truncate(s, maxWidth, "...")
}

// TruncateEllipsis shortens a string using a single character ellipsis (…).
// Uses runewidth so wide characters (CJK, emoji) are measured and cut whole.
func TruncateEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	return runewidth.Truncate(s, maxWidth, "…")
}

// Pad fills a string with spaces to reach the specified width.
// Uses runewidth for proper handling of wide characters.
func Pad(s string, width int) string {
	return runewidth.FillRight(s, width)
}

// TruncateAndPad truncates a string if necessary, then pads to the exact width.
// This ensures the output is exactly width characters wide.
func TruncateAndPad(s string, width int) string {
	return Pad(Truncate(s, width), width)
}

// TruncateAndPadEllipsis is like TruncateAndPad but uses a single character ellipsis.
func TruncateAndPadEllipsis(s string, width int) string {
	return Pad(TruncateEllipsis(s, width), width)
}

// Row creates a row with left and right aligned content separated by spaces.
// The total width of the output will be exactly width characters.
// Applies the theme background to gap spaces when an explicit background is set.
func Row(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := max(width-leftWidth-rightWidth, 1)
	return left + EmptyLine(gap) + right
}

// Separator creates a horizontal separator line of the specified width.
// Applies the theme background when an explicit background is set.
func Separator(width int) string {
	return styles.T().Bg(strings.Repeat("─", width))
}

// EmptyLine creates an empty line (spaces) of the specified width.
// Applies the theme background when an explicit background is set.
func EmptyLine(width int) string {
	return styles.T().Bg(strings.Repeat(" ", width))
}

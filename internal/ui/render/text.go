// Package render provides text rendering utilities for TUI components.
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/llehouerou/waves/internal/ui/styles"
)

// Truncate shortens a string to fit within maxWidth, between two characters,
// ending it with "..." if truncated. It measures widths as lipgloss.Width
// does, whatever the locale, and keeps ANSI escape codes whole.
// Under 3 columns it returns "..." wider than maxWidth: use Cut where the
// room can be that small.
func Truncate(s string, maxWidth int) string {
	if ansi.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth < 3 {
		return "..."
	}
	return ansi.Truncate(s, maxWidth, "...")
}

// TruncateEllipsis shortens a string using a single character ellipsis (…):
// nothing at 0 columns or less. Like Truncate, it cuts between two
// characters, measures widths as lipgloss.Width does and keeps ANSI escape
// codes whole.
func TruncateEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	return ansi.Truncate(s, maxWidth, "…")
}

// Cut shortens s to width columns, between two characters, ending with "..."
// when there is room for it: nothing at 0 columns or less. Like Truncate, it
// keeps ANSI escape codes whole and measures widths as lipgloss.Width does;
// unlike Truncate, it is never wider than width, even under 3 columns.
func Cut(s string, width int) string {
	if width < 3 {
		return ansi.Truncate(s, width, "")
	}
	return ansi.Truncate(s, width, "...")
}

// CutLeft shortens s to width columns from its start, between two characters,
// keeping its end and beginning with "..." when there is room for it: nothing
// at 0 columns or less. Like Cut, it is never wider than width, keeps ANSI
// escape codes whole and measures widths as lipgloss.Width does.
func CutLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	total := ansi.StringWidth(s)
	if total <= width {
		return s
	}
	prefix := "..."
	if width < 3 {
		prefix = ""
	}
	keep := width - len(prefix)
	tail := ansi.TruncateLeft(s, total-keep, "")
	if ansi.StringWidth(tail) > keep {
		// A wide character straddles the cut: TruncateLeft keeps it, give it up.
		tail = ansi.TruncateLeft(s, total-keep+1, "")
	}
	return prefix + tail
}

// Pad fills a string with spaces to reach the specified width, measured as
// lipgloss.Width does: ANSI escape codes count for no column. A string as wide
// as width or wider is returned unchanged.
func Pad(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// TruncateAndPad truncates a string if necessary, then pads to the exact width.
// This ensures the output is exactly width columns wide, as lipgloss.Width measures.
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
// A width of 0 or less is an empty string.
func Separator(width int) string {
	if width <= 0 {
		return ""
	}
	return styles.T().Bg(strings.Repeat("─", width))
}

// EmptyLine creates an empty line (spaces) of the specified width.
// Applies the theme background when an explicit background is set.
// A width of 0 or less is an empty string.
func EmptyLine(width int) string {
	if width <= 0 {
		return ""
	}
	return styles.T().Bg(strings.Repeat(" ", width))
}

// Package render provides text rendering utilities for TUI components.
package render

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Sanitize removes control characters (except tab/space) and replaces
// invalid UTF-8 bytes with the Unicode replacement character.
// This prevents broken terminal rendering from bad metadata.
func Sanitize(s string) string {
	if !needsSanitize(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			// Invalid byte — skip it
			i++
			continue
		}
		if r != '\t' && unicode.IsControl(r) {
			// Control character — skip
			i += size
			continue
		}
		// Replace non-breaking space with regular space
		if r == '\u00a0' {
			b.WriteByte(' ')
			i += size
			continue
		}
		b.WriteString(s[i : i+size])
		i += size
	}
	return b.String()
}

// needsSanitize returns true if the string contains bytes that need sanitizing.
func needsSanitize(s string) bool {
	for i := range len(s) {
		b := s[i]
		if b < 0x20 && b != '\t' { // ASCII control chars (except tab)
			return true
		}
		if b >= 0x80 && b <= 0x9f { // C1 control range / invalid lead bytes
			return true
		}
		if b == 0xc2 { // Potential 2-byte sequence for U+00A0 (NBSP) or C1 controls
			if i+1 < len(s) && s[i+1] == 0xa0 {
				return true
			}
		}
	}
	return false
}

// Truncate shortens a string to fit within maxWidth, adding an ellipsis if truncated.
// Uses runewidth for proper handling of wide characters (CJK, emoji).
// Sanitizes the input to remove control characters and invalid UTF-8.
func Truncate(s string, maxWidth int) string {
	return runewidth.Truncate(Sanitize(s), maxWidth, "...")
}

// TruncateEllipsis shortens a string using a single character ellipsis (…).
// Useful when a cleaner truncation appearance is desired.
func TruncateEllipsis(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	for lipgloss.Width(s) > maxWidth-1 && s != "" {
		s = s[:len(s)-1]
	}
	return s + "…"
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
func Row(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := max(width-leftWidth-rightWidth, 1)
	return left + strings.Repeat(" ", gap) + right
}

// Separator creates a horizontal separator line of the specified width.
func Separator(width int) string {
	return strings.Repeat("─", width)
}

// EmptyLine creates an empty line (spaces) of the specified width.
func EmptyLine(width int) string {
	return strings.Repeat(" ", width)
}

package overlay

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Compose overlays content on top of a base view.
// Non-space characters in overlay replace the base at the same position.
// This function is ANSI-aware and handles styled text correctly.
func Compose(base, overlay string, width, _ int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, overlayLine := range overlayLines {
		if i >= len(baseLines) {
			break
		}

		// Strip ANSI to find visible content bounds
		plainOverlay := ansi.Strip(overlayLine)
		if strings.TrimSpace(plainOverlay) == "" {
			continue // empty line (visually)
		}

		// Find visible start and end positions (in display columns)
		startCol := 0
		for _, r := range plainOverlay {
			if r != ' ' {
				break
			}
			startCol++
		}

		// Trim trailing spaces from end position
		trimmed := strings.TrimRight(plainOverlay, " ")
		endCol := startCol + ansi.StringWidth(trimmed[startCol:])

		// Extract the overlay content (with ANSI codes intact)
		overlayContent := ansi.Cut(overlayLine, startCol, endCol)

		// Build new line: base prefix + overlay content + base suffix
		baseLine := baseLines[i]
		baseWidth := ansi.StringWidth(ansi.Strip(baseLine))

		// Pad base line if needed
		if baseWidth < width {
			baseLine += strings.Repeat(" ", width-baseWidth)
		}

		// Construct result: base[0:startCol] + overlay + base[endCol:]
		result := ansi.Cut(baseLine, 0, startCol) + overlayContent
		if endCol < width {
			result += ansi.Cut(baseLine, endCol, width)
		}

		baseLines[i] = result
	}

	return strings.Join(baseLines, "\n")
}

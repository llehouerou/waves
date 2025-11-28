package overlay

import "strings"

// Compose overlays content on top of a base view.
// Non-space characters in overlay replace the base at the same position.
func Compose(base, overlay string, width, _ int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, overlayLine := range overlayLines {
		if i >= len(baseLines) {
			break
		}

		// Find the actual content bounds in the overlay line (by rune position)
		runes := []rune(overlayLine)
		startPos := -1
		endPos := -1

		for j, r := range runes {
			if r != ' ' {
				if startPos == -1 {
					startPos = j
				}
				endPos = j + 1
			}
		}

		if startPos == -1 {
			continue // empty line
		}

		overlayContent := string(runes[startPos:endPos])

		// Build new line: base prefix + overlay content + base suffix
		baseRunes := []rune(baseLines[i])
		// Pad base line if needed
		for len(baseRunes) < width {
			baseRunes = append(baseRunes, ' ')
		}

		var result []rune
		// Copy base up to start
		if startPos <= len(baseRunes) {
			result = append(result, baseRunes[:startPos]...)
		}

		// Add overlay content
		result = append(result, []rune(overlayContent)...)

		// Copy base after end
		if endPos < len(baseRunes) {
			result = append(result, baseRunes[endPos:]...)
		}

		baseLines[i] = string(result)
	}

	return strings.Join(baseLines, "\n")
}

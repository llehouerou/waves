package playerbar

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	filledBlock = "▓"
	emptyBlock  = "░"
)

// RenderProgressBar renders a block-style progress bar.
// Format: ▶  1:23  ▓▓▓▓▓░░░░░  4:56
func RenderProgressBar(position, duration time.Duration, width int, playing bool) string {
	status := "▶"
	if !playing {
		status = "⏸"
	}

	posStr := formatDuration(position)
	durStr := formatDuration(duration)

	// Calculate space for the bar itself
	// Format: "▶  1:23  ▓▓▓░░░  4:56"
	fixedWidth := lipgloss.Width(status) + 2 + lipgloss.Width(posStr) + 2 + 2 + lipgloss.Width(durStr)
	barWidth := width - fixedWidth

	if barWidth < 3 {
		// Too narrow for bar, just show times
		return status + "  " + posStr + " / " + durStr
	}

	// Calculate filled portion
	var ratio float64
	if duration > 0 {
		ratio = float64(position) / float64(duration)
	}
	filled := min(int(float64(barWidth)*ratio), barWidth)

	bar := strings.Repeat(filledBlock, filled) + strings.Repeat(emptyBlock, barWidth-filled)

	return status + "  " + posStr + "  " + bar + "  " + durStr
}

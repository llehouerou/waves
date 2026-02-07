package playerbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
)

// volumeChars represents volume bar characters from low to high.
//
//nolint:gochecknoglobals // used by volume rendering functions
var volumeChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderVolumeCompact renders the volume indicator for compact mode.
// Format: "75% ▆▆▆" or "🔇75% ░░░" when muted
func RenderVolumeCompact(volume float64, muted bool) string {
	pct := int(volume * 100)
	bar := VolumeBar(volume, 3)

	if muted {
		muteIcon := icons.VolumeMute()
		dimBar := VolumeStyle().Foreground(lipgloss.Color("240")).Render("░░░")
		return fmt.Sprintf("%s%d%% %s", muteIcon, pct, dimBar)
	}

	return fmt.Sprintf("%d%% %s", pct, VolumeStyle().Render(bar))
}

// RenderVolumeExpanded renders the volume indicator for expanded mode.
// Returns a vertical column with percentage on top and bar below.
func RenderVolumeExpanded(volume float64, muted bool, height int) string {
	pct := int(volume * 100)

	// Build percentage line
	var pctLine string
	if muted {
		pctLine = fmt.Sprintf("%s%d%%", icons.VolumeMute(), pct)
	} else {
		pctLine = fmt.Sprintf("%d%%", pct)
	}

	// Center the percentage
	pctLine = lipgloss.PlaceHorizontal(5, lipgloss.Center, pctLine)

	// Build vertical bar (height-1 because first line is percentage)
	barHeight := max(height-1, 1)

	var lines []string
	lines = append(lines, pctLine)

	if muted {
		// Show empty bars when muted
		dimStyle := VolumeStyle().Foreground(lipgloss.Color("240"))
		for range barHeight {
			lines = append(lines, lipgloss.PlaceHorizontal(5, lipgloss.Center, dimStyle.Render("░")))
		}
	} else {
		// Fill from bottom to top based on volume
		filledBars := int(float64(barHeight) * volume)
		for i := range barHeight {
			// i=0 is top, i=barHeight-1 is bottom
			// We want to fill from bottom, so check if (barHeight - 1 - i) < filledBars
			fromBottom := barHeight - 1 - i
			var char string
			if fromBottom < filledBars {
				// Calculate which character to use based on position
				charIdx := int(float64(fromBottom+1) / float64(barHeight) * float64(len(volumeChars)-1))
				char = string(volumeChars[charIdx])
			} else {
				char = "░"
			}
			lines = append(lines, lipgloss.PlaceHorizontal(5, lipgloss.Center, VolumeStyle().Render(char)))
		}
	}

	return strings.Join(lines, "\n")
}

// VolumeBar creates a horizontal bar representation of volume.
func VolumeBar(volume float64, width int) string {
	if width <= 0 {
		return ""
	}

	// Map volume to character index
	charIdx := int(volume * float64(len(volumeChars)-1))
	if charIdx >= len(volumeChars) {
		charIdx = len(volumeChars) - 1
	}
	if charIdx < 0 {
		charIdx = 0
	}

	char := volumeChars[charIdx]
	return strings.Repeat(string(char), width)
}

// VolumeStyle returns the style for volume indicator.
func VolumeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Cyan
}

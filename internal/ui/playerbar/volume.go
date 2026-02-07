package playerbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
)

const volumeBarWidth = 5

// RenderVolumeCompact renders the volume indicator for compact mode.
// Format: "100% ━━━━━" or "🔇 100% ─────" when muted
// Uses horizontal bar like progress bar (filled/empty).
func RenderVolumeCompact(volume float64, muted bool) string {
	pct := int(volume * 100)
	pctStr := volumeTextStyle().Render(fmt.Sprintf("%3d%%", pct))

	// Calculate filled portion (0% = 0 filled, 100% = all filled)
	filled := int(float64(volumeBarWidth) * volume)
	if volume > 0 && filled == 0 {
		filled = 1 // Show at least 1 when not zero
	}
	empty := volumeBarWidth - filled

	if muted {
		muteIcon := icons.VolumeMute()
		dimBar := volumeMutedStyle().Render(strings.Repeat("─", volumeBarWidth))
		return fmt.Sprintf("%s %s %s", muteIcon, pctStr, dimBar)
	}

	filledBar := volumeFilledStyle().Render(strings.Repeat("━", filled))
	emptyBar := volumeEmptyStyle().Render(strings.Repeat("─", empty))

	return fmt.Sprintf("%s %s%s", pctStr, filledBar, emptyBar)
}

// RenderVolumeExpanded renders the volume indicator for expanded mode.
// Returns a vertical column with percentage on top and bar below.
func RenderVolumeExpanded(volume float64, muted bool, height int) string {
	pct := int(volume * 100)

	// Build percentage line
	var pctLine string
	if muted {
		pctLine = fmt.Sprintf("%s %d%%", icons.VolumeMute(), pct)
	} else {
		pctLine = fmt.Sprintf("%d%%", pct)
	}

	// Style and center the percentage
	pctLine = lipgloss.PlaceHorizontal(6, lipgloss.Center, volumeTextStyle().Render(pctLine))

	// Build vertical bar (height-1 because first line is percentage)
	barHeight := max(height-1, 1)

	var lines []string
	lines = append(lines, pctLine)

	// Calculate filled bars (0% = 0 filled, 100% = all filled)
	filledBars := int(float64(barHeight) * volume)
	if volume > 0 && filledBars == 0 {
		filledBars = 1
	}

	if muted {
		// Show empty bars when muted
		for range barHeight {
			lines = append(lines, lipgloss.PlaceHorizontal(6, lipgloss.Center, volumeMutedStyle().Render("─")))
		}
	} else {
		for i := range barHeight {
			// i=0 is top, i=barHeight-1 is bottom
			// Fill from bottom: if (barHeight - 1 - i) < filledBars, it's filled
			fromBottom := barHeight - 1 - i
			var char string
			var style lipgloss.Style
			if fromBottom < filledBars {
				char = "┃"
				style = volumeFilledStyle()
			} else {
				char = "│"
				style = volumeEmptyStyle()
			}
			lines = append(lines, lipgloss.PlaceHorizontal(6, lipgloss.Center, style.Render(char)))
		}
	}

	return strings.Join(lines, "\n")
}

// volumeTextStyle returns the style for volume percentage text.
func volumeTextStyle() lipgloss.Style {
	return artistStyle()
}

// volumeFilledStyle returns the style for filled volume bar.
func volumeFilledStyle() lipgloss.Style {
	return progressBarFilled()
}

// volumeEmptyStyle returns the style for empty volume bar.
func volumeEmptyStyle() lipgloss.Style {
	return progressBarEmpty()
}

// volumeMutedStyle returns the style for muted volume bar (very dim).
func volumeMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
}

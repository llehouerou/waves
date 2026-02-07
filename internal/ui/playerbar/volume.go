package playerbar

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
)

// RenderVolumeCompact renders the volume indicator for compact mode.
// Format: "🔊 100%" or "🔇 100%" when muted
func RenderVolumeCompact(volume float64, muted bool) string {
	pct := int(volume * 100)
	icon := icons.Volume()
	if muted {
		icon = icons.VolumeMute()
	}
	return fmt.Sprintf("%s %3d%%", icon, pct)
}

// RenderVolumeExpanded renders the volume indicator for expanded mode.
// Returns a vertical column with icon on top and percentage below.
func RenderVolumeExpanded(volume float64, muted bool, _ int) string {
	pct := int(volume * 100)
	icon := icons.Volume()
	if muted {
		icon = icons.VolumeMute()
	}

	iconLine := lipgloss.PlaceHorizontal(5, lipgloss.Center, icon)
	pctLine := lipgloss.PlaceHorizontal(5, lipgloss.Center, volumeTextStyle().Render(fmt.Sprintf("%d%%", pct)))

	return iconLine + "\n" + pctLine
}

// volumeTextStyle returns the style for volume percentage text.
func volumeTextStyle() lipgloss.Style {
	return artistStyle()
}

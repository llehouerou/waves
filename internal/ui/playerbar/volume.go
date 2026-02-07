package playerbar

import (
	"fmt"

	"github.com/llehouerou/waves/internal/icons"
)

// RenderVolumeCompact renders the volume indicator.
// Format: "🔊 100%" or "🔇 100%" when muted
func RenderVolumeCompact(volume float64, muted bool) string {
	pct := int(volume * 100)
	icon := icons.Volume()
	if muted {
		icon = icons.VolumeMute()
	}
	return progressTimeStyle().Render(fmt.Sprintf("%s %3d%%", icon, pct))
}

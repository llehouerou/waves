package playerbar

import (
	"fmt"

	"github.com/llehouerou/waves/internal/icons"
)

// RenderVolumeCompact renders the volume indicator.
// Icon changes based on volume level: mute, low, medium, high.
func RenderVolumeCompact(volume float64, muted bool) string {
	pct := int(volume * 100)
	icon := icons.VolumeIcon(volume, muted)
	return progressTimeStyle().Render(fmt.Sprintf("%s %3d%%", icon, pct))
}

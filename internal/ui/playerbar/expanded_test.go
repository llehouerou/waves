package playerbar

import (
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/ui/testutil"
)

// Too narrow for the expanded view, the player bar draws what the compact mode
// draws at that width (issue #179).
func TestRenderExpanded_TooNarrowDrawsTheCompactBar(t *testing.T) {
	expanded := State{Playing: true, Title: "Song", Artist: "Band", DisplayMode: ModeExpanded}
	compact := expanded
	compact.DisplayMode = ModeCompact
	for _, width := range []int{0, 1, 20, 45} {
		if got, want := RenderExpanded(expanded, width), Render(compact, width); got != want {
			t.Errorf("RenderExpanded(width %d) =\n%s\nwant the compact bar:\n%s", width, got, want)
		}
	}
}

// The player bar shows the genres of a track with " / " between them, joined
// by "; " as waves writes them, or by ";" as it wrote them before.
func TestRenderExpanded_SeparatesTheGenresBySlashes(t *testing.T) {
	for _, genre := range []string{"Rock; Pop", "Rock;Pop"} {
		s := State{Playing: true, Title: "Song", Artist: "Band", Genre: genre, DisplayMode: ModeExpanded}
		if view := testutil.StripANSI(RenderExpanded(s, 120)); !strings.Contains(view, "Rock / Pop") {
			t.Errorf("genre %q shown as:\n%s\nwant Rock / Pop", genre, view)
		}
	}
}

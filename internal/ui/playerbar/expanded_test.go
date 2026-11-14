package playerbar

import "testing"

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

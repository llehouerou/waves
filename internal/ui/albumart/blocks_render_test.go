package albumart

import (
	"testing"
)

// End to end through the Renderer: a real cover comes out as block art inside
// the reserved area, and nothing is written to the terminal out of band.
// Run with -v to eyeball the result.
func TestBlockArt_RendersFixtureCoverInline(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	p := trueColorBlocks()
	r := New(p)
	r.SetSize(16, 8)

	const path = "testdata/with/track.mp3"
	r.Commit(path, r.LoadTrack(path))

	if !r.HasImage() {
		t.Fatal("fixture has no cover")
	}
	if cmd := r.GetPlacementCmd(1, 1); cmd != "" {
		t.Errorf("block art must not emit a placement command, got %q", cmd)
	}
	t.Logf("\n%s", r.GetPlaceholder())
}

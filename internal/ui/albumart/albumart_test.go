package albumart

import (
	"os"
	"testing"
)

// The renderer splits loading (slow, off the UI goroutine) from committing
// (cheap, in Update). These cover the contract between the two, including the
// no-cover case, which still has to drop the previous image.
func newTestRenderer(t *testing.T) *Renderer {
	t.Helper()
	r := &Renderer{protocol: &KittyProtocol{}}
	r.SetSize(20, 10)
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // keep the disk cache out of the way
	return r
}

// Fixtures live in dedicated directories: ExtractCoverArt falls back to folder
// images, so a stray cover.png next to a fixture would make these pass for the
// wrong reason.
func trackWithCover(t *testing.T) string {
	t.Helper()
	const path = "testdata/with/track.mp3"
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	return path
}

func TestLoadTrack_ReturnsPNGForEmbeddedCover(t *testing.T) {
	r := newTestRenderer(t)

	png := r.LoadTrack(trackWithCover(t))
	if len(png) == 0 {
		t.Fatal("LoadTrack returned no data for a track with a cover")
	}
	if string(png[1:4]) != "PNG" {
		t.Errorf("data is not PNG: % x", png[:8])
	}
}

func TestLoadTrack_NilWithoutCover(t *testing.T) {
	r := newTestRenderer(t)

	if png := r.LoadTrack("testdata/without/track.mp3"); png != nil {
		t.Errorf("expected nil for a track without a cover, got %d bytes", len(png))
	}
	if png := r.LoadTrack("testdata/nonexistent.mp3"); png != nil {
		t.Errorf("expected nil for a missing file, got %d bytes", len(png))
	}
}

func TestCommit_InstallsImageAndClearsNeedsLoad(t *testing.T) {
	r := newTestRenderer(t)
	path := trackWithCover(t)

	if !r.NeedsLoad(path) {
		t.Fatal("NeedsLoad should be true before anything is loaded")
	}

	r.Commit(path, r.LoadTrack(path))

	if !r.HasImage() {
		t.Error("HasImage should be true after committing a cover")
	}
	if r.CurrentPath() != path {
		t.Errorf("CurrentPath = %q, want %q", r.CurrentPath(), path)
	}
	if r.NeedsLoad(path) {
		t.Error("NeedsLoad should be false once the art is committed")
	}
}

// A track without a cover must still count as handled, otherwise the UI reloads
// it on every tick, and the previous image must be dropped.
func TestCommit_NoCoverDropsPreviousImage(t *testing.T) {
	r := newTestRenderer(t)
	withCover := trackWithCover(t)

	r.Commit(withCover, r.LoadTrack(withCover))
	if !r.HasImage() {
		t.Fatal("setup: expected an image")
	}

	deleteCmd := r.Commit("testdata/without/track.mp3", nil)

	if r.HasImage() {
		t.Error("HasImage should be false after committing a track with no cover")
	}
	if deleteCmd == "" {
		t.Error("expected a delete command to drop the previous image")
	}
	if r.NeedsLoad("testdata/without/track.mp3") {
		t.Error("a coverless track must not be reloaded forever")
	}
}

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

// recordingProtocol notes which image IDs were asked to be deleted, so the
// tests can check that no removal is ever lost.
type recordingProtocol struct {
	KittyProtocol
	deleted []uint32
}

func (p *recordingProtocol) Delete(id uint32) string {
	p.deleted = append(p.deleted, id)
	return p.KittyProtocol.Delete(id)
}

func (p *recordingProtocol) deletedIDs() map[uint32]bool {
	ids := make(map[uint32]bool, len(p.deleted))
	for _, id := range p.deleted {
		ids[id] = true
	}
	return ids
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

// Skipping tracks faster than the UI renders used to leave ghost art on screen
// (issue #30): each Commit returned a delete command that the next Commit
// overwrote before it ever reached the terminal. Removals therefore have to be
// re-emitted with the placement, which is rebuilt on every frame.
func TestGetPlacementCmd_RemovesImagesFromSkippedFrames(t *testing.T) {
	proto := &recordingProtocol{}
	r := &Renderer{protocol: proto}
	r.SetSize(20, 10)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	path := trackWithCover(t)
	png := r.LoadTrack(path)

	r.Commit(path, png)
	first := r.currentImageID
	r.Commit(path, png)
	second := r.currentImageID
	r.Commit(path, png)
	current := r.currentImageID

	proto.deleted = nil
	out := r.GetPlacementCmd(1, 1)

	deleted := proto.deletedIDs()
	if !deleted[first] || !deleted[second] {
		t.Errorf("placement dropped stale images %d/%d, deleted %v", first, second, proto.deleted)
	}
	if deleted[current] {
		t.Error("placement deleted the image it is about to show")
	}
	if out == "" {
		t.Error("placement command is empty")
	}
}

// Bubble Tea skips frame lines identical to the previous frame, so a frame
// carrying a removal must never repeat itself.
func TestGetPlacementCmd_VariesWhileRemovalsPending(t *testing.T) {
	r := newTestRenderer(t)
	path := trackWithCover(t)
	png := r.LoadTrack(path)

	r.Commit(path, png)
	r.Commit(path, png)

	if first, second := r.GetPlacementCmd(1, 1), r.GetPlacementCmd(1, 1); first == second {
		t.Error("consecutive frames are identical, the pending removal can be dropped")
	}
}

// Stale IDs must not pile up for the whole session: the list is bounded.
func TestGetPlacementCmd_BoundsStaleImages(t *testing.T) {
	proto := &recordingProtocol{}
	r := &Renderer{protocol: proto}
	r.SetSize(20, 10)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	path := trackWithCover(t)
	png := r.LoadTrack(path)
	for range 50 {
		r.Commit(path, png)
	}

	proto.deleted = nil
	r.GetPlacementCmd(1, 1)

	if len(proto.deleted) > maxStaleImages {
		t.Errorf("placement emitted %d deletes, want at most %d", len(proto.deleted), maxStaleImages)
	}
}

// Once art is cleared, nothing must be placed, but the removal still has to be
// repeated until a frame carries it.
func TestGetPlacementCmd_AfterClear(t *testing.T) {
	proto := &recordingProtocol{}
	r := &Renderer{protocol: proto}
	r.SetSize(20, 10)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	path := trackWithCover(t)
	r.Commit(path, r.LoadTrack(path))
	cleared := r.currentImageID
	r.Clear()

	proto.deleted = nil
	out := r.GetPlacementCmd(1, 1)

	if !proto.deletedIDs()[cleared] {
		t.Errorf("cleared image %d was never deleted, deleted %v", cleared, proto.deleted)
	}
	if out == "" {
		t.Error("expected the delete command, got nothing")
	}
}

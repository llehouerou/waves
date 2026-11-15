package library_test

import (
	"path/filepath"
	"slices"
	"testing"
)

// addTracks adds paths to the library and returns what it left out because
// no source holds it.
func (f scanFixture) addTracks(t *testing.T, paths ...string) []string {
	t.Helper()
	outside, err := f.lib.AddTracks(paths)
	if err != nil {
		t.Fatalf("add tracks: %v", err)
	}
	return outside
}

func TestAddTracks_LeavesOutAFileNoSourceHolds(t *testing.T) {
	f := newScanFixture(t)
	track := writeTrack(t, filepath.Join(t.TempDir(), "a.mp3"), "A")

	outside := f.addTracks(t, track)

	if !slices.Equal(outside, []string{track}) {
		t.Errorf("left out %v, want %v", outside, []string{track})
	}
	if f.trackID(t, track) != 0 {
		t.Error("the track no source holds is in the library")
	}
}

func TestAddTracks_AddsAFileASourceHolds(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "A")

	if outside := f.addTracks(t, track); len(outside) != 0 {
		t.Errorf("left out %v, want nothing", outside)
	}
	if f.trackID(t, track) == 0 {
		t.Error("the track is not in the library")
	}
}

func TestAddTracks_WritesNoTrackOfARemovedSource(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "A")
	f.addTracks(t, track)
	f.remove(t, source)

	outside := f.addTracks(t, track)

	if !slices.Equal(outside, []string{track}) {
		t.Errorf("left out %v, want %v", outside, []string{track})
	}
	if f.trackID(t, track) != 0 {
		t.Error("the track of the removed source is in the library")
	}
	results, err := f.lib.SearchFTS("")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := describe(results); len(got) != 0 {
		t.Errorf("search index = %v, want it empty", got)
	}
}

func TestAddTracks_WritesWhatAnOuterSourceHoldsOnceTheInnerOneIsRemoved(t *testing.T) {
	f := newScanFixture(t)
	music, sub, _, one := nestedSources(t)
	f.add(t, music, sub)
	f.remove(t, sub)

	if outside := f.addTracks(t, one); len(outside) != 0 {
		t.Errorf("left out %v, want nothing: music holds it", outside)
	}
	if f.trackID(t, one) == 0 {
		t.Error("one.mp3, which music holds, is not in the library")
	}
}

func TestAddTracks_WritesWhatAnInnerSourceHoldsOnceTheOuterOneIsRemoved(t *testing.T) {
	f := newScanFixture(t)
	music, sub, top, one := nestedSources(t)
	f.add(t, music, sub)
	f.remove(t, music)

	outside := f.addTracks(t, top, one)

	if !slices.Equal(outside, []string{top}) {
		t.Errorf("left out %v, want %v", outside, []string{top})
	}
	if f.trackID(t, top) != 0 {
		t.Error("top.mp3, which no source holds any more, is in the library")
	}
	if f.trackID(t, one) == 0 {
		t.Error("one.mp3, which music/sub holds, is not in the library")
	}
}

// A track written under no source before AddTracks looked at the sources
// stays as it is: it is neither updated nor deleted.
func TestAddTracks_LeavesATrackAlreadyUnderNoSourceAsItIs(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "A")
	f.addTracks(t, track)
	id := f.trackID(t, track)
	if _, err := f.db.Exec(`DELETE FROM library_sources`); err != nil {
		t.Fatalf("drop the source alone: %v", err)
	}
	writeTrack(t, track, "A changed")
	later(t, track)

	f.addTracks(t, track)

	got, err := f.lib.TrackByPath(track)
	if err != nil || got == nil || got.ID != id || got.Title != "A" {
		t.Errorf("track = %+v (err %v), want track %d still titled A", got, err, id)
	}
}

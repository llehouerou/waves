package library_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
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

// search returns what the search index holds, as describe names it.
func (f scanFixture) search(t *testing.T) []string {
	t.Helper()
	results, err := f.lib.SearchFTS("")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return describe(results)
}

func TestAddTracks_IndexesANewFile(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "A")

	f.addTracks(t, track)

	want := []string{"artist Low", "album Low/Trust", "track A"}
	if got := f.search(t); !slices.Equal(got, want) {
		t.Errorf("search index = %v, want %v", got, want)
	}
}

func TestAddTracks_ReplacesTheSearchEntryOfATrackWithOtherTags(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "A")
	f.addTracks(t, track)
	tagstest.WriteMP3(t, track, tags.Tag{Artist: "Yo La Tengo", AlbumArtist: "Yo La Tengo", Album: "Fade", Title: "Ohm"})
	later(t, track)

	f.addTracks(t, track)

	want := []string{"artist Yo La Tengo", "album Yo La Tengo/Fade", "track Ohm"}
	if got := f.search(t); !slices.Equal(got, want) {
		t.Errorf("search index = %v, want %v", got, want)
	}
}

// The error rolls back the whole call: a new file before the broken one in
// the same call is not written either.
func TestAddTracks_ReturnsTheErrorOfATrackItCannotRead(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "Old title")
	f.addTracks(t, track)
	if _, err := f.db.Exec(`UPDATE library_tracks SET mtime = 'abc' WHERE path = ?`, track); err != nil {
		t.Fatalf("break the row: %v", err)
	}
	before := f.search(t)
	writeTrack(t, track, "New title")
	later(t, track)
	fresh := writeTrack(t, filepath.Join(source, "b.mp3"), "B")

	outside, err := f.lib.AddTracks([]string{fresh, track})

	if err == nil || !strings.Contains(err.Error(), track) {
		t.Errorf("error = %v, want the error of the read of %s", err, track)
	}
	if outside != nil {
		t.Errorf("left out %v, want nothing on an error", outside)
	}
	if f.trackID(t, fresh) != 0 {
		t.Error("b.mp3 is in the library, want nothing of the call written")
	}
	if got := f.search(t); !slices.Equal(got, before) {
		t.Errorf("search index = %v, want it unchanged: %v", got, before)
	}
}

func TestAddTracks_WritesNoTrackItCannotReadBack(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "A")
	_, err := f.db.Exec(`CREATE TRIGGER break_mtime AFTER INSERT ON library_tracks
		BEGIN UPDATE library_tracks SET mtime = 'abc' WHERE id = NEW.id; END`)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	if _, err := f.lib.AddTracks([]string{track}); err == nil {
		t.Error("AddTracks returned no error, want the error of the read of the track it wrote")
	}
	// Counted, not read: the read of this track fails.
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM library_tracks WHERE path = ?`, track).Scan(&count); err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if count != 0 {
		t.Error("the track is in the library, want no track written without its search entry")
	}
	if got := f.search(t); len(got) != 0 {
		t.Errorf("search index = %v, want it empty", got)
	}
}

// A scan that adds the track of a file while AddTracks reads the other files
// of its call leaves one search entry for it once AddTracks has written it.
func TestAddTracks_IndexesOnceATrackAScanAddedMeanwhile(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	f.add(t, source)
	track := writeTrack(t, filepath.Join(source, "a.mp3"), "A")
	// tags.Read has no fallback for this extension: it opens the FIFO once.
	slow := filepath.Join(t.TempDir(), "slow.fifo")
	if err := syscall.Mkfifo(slow, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := f.lib.AddTracks([]string{track, slow})
		done <- err
	}()
	// Opening this end returns once AddTracks has opened the other.
	w, err := os.OpenFile(slow, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open the FIFO: %v", err)
	}
	// A failed test must not leave AddTracks waiting on the FIFO.
	t.Cleanup(func() { w.Close() })
	f.scan(t, source)
	if err := f.lib.RebuildFTSIndex(); err != nil {
		t.Fatalf("rebuild search index: %v", err)
	}
	w.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("add tracks: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AddTracks did not return")
	}
	want := []string{"artist Low", "album Low/Trust", "track A"}
	if got := f.search(t); !slices.Equal(got, want) {
		t.Errorf("search index = %v, want %v", got, want)
	}
}

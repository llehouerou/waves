package library_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// refused is the message of a write the database refuses.
const refused = "write refused"

// write is a kind of write on the tracks of the library, as SQL names it.
type write string

const (
	insert write = "INSERT"
	update write = "UPDATE"
	remove write = "DELETE"
)

// refuse makes the database abort every write of kind on the tracks of the
// library, until the returned function lifts it.
func (f scanFixture) refuse(t *testing.T, kind write) func() {
	t.Helper()
	name := "refuse_" + strings.ToLower(string(kind))
	_, err := f.db.Exec(`CREATE TRIGGER ` + name + ` BEFORE ` + string(kind) + ` ON library_tracks
		BEGIN SELECT RAISE(ABORT, '` + refused + `'); END`)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	return func() {
		t.Helper()
		if _, err := f.db.Exec(`DROP TRIGGER ` + name); err != nil {
			t.Fatalf("drop trigger: %v", err)
		}
	}
}

// scanFailing runs a scan of sources that must fail, and returns its error.
// A scan that fails reports no stats.
func (f scanFixture) scanFailing(t *testing.T, sources ...string) error {
	t.Helper()
	stats, err := f.run(nil, sources...)
	if stats != nil {
		t.Errorf("the scan reported stats %+v, want none from a scan that fails", stats)
	}
	if err == nil {
		t.Fatal("the scan returned no error, want the write it could not do")
	}
	if !strings.Contains(err.Error(), refused) {
		t.Errorf("scan error = %q, want it to wrap the database error %q", err, refused)
	}
	return err
}

// later gives path a modification time after the one a scan saw.
func later(t *testing.T, path string) {
	t.Helper()
	at := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func TestRefresh_AnInsertTheDatabaseRefusesFailsTheScanAndRemovesNoTrack(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	gone := writeTrack(t, filepath.Join(source, "gone.mp3"), "Gone")
	writeTrack(t, filepath.Join(source, "kept.mp3"), "Kept")
	f.scan(t, source)
	ids := f.ids(t, gone)

	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	added := writeTrack(t, filepath.Join(source, "added.mp3"), "Added")
	f.refuse(t, insert)
	err := f.scanFailing(t, source)

	if !strings.Contains(err.Error(), added) {
		t.Errorf("scan error = %q, want it to name %s", err, added)
	}
	if f.trackID(t, added) != 0 {
		t.Error("added.mp3 is in the library, want no track for it")
	}
	// The failure ends the scan before its cleaning.
	if got := f.ids(t, gone); !slices.Equal(got, ids) {
		t.Errorf("gone.mp3 track IDs = %v, want %v: kept by a scan that failed", got, ids)
	}
}

func TestRefresh_AnUpdateTheDatabaseRefusesFailsTheScanAndKeepsTheOldTags(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	changed := writeTrack(t, filepath.Join(source, "changed.mp3"), "Before")
	f.scan(t, source)

	writeTrack(t, changed, "After")
	later(t, changed)
	f.refuse(t, update)
	err := f.scanFailing(t, source)

	if !strings.Contains(err.Error(), changed) {
		t.Errorf("scan error = %q, want it to name %s", err, changed)
	}
	track, err := f.lib.TrackByPath(changed)
	if err != nil || track.Title != "Before" {
		t.Errorf("track = %+v (err %v), want its old title Before", track, err)
	}
}

func TestRefresh_ADeleteTheDatabaseRefusesFailsTheScanAndKeepsTheTrack(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	kept := writeTrack(t, filepath.Join(source, "kept.mp3"), "Kept")
	gone := writeTrack(t, filepath.Join(source, "gone.mp3"), "Gone")
	f.scan(t, source)
	ids := f.ids(t, kept, gone)
	playlist := f.playlistOf(t, kept, gone)

	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	f.refuse(t, remove)
	err := f.scanFailing(t, source)

	if !strings.Contains(err.Error(), gone) {
		t.Errorf("scan error = %q, want it to name %s", err, gone)
	}
	if got := f.ids(t, kept, gone); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v: unchanged", got, ids)
	}
	if got := f.entries(t, playlist); !slices.Equal(got, []string{kept, gone}) {
		t.Errorf("playlist = %v, want %v", got, []string{kept, gone})
	}
}

func TestRefresh_TheScanAfterAFailedOneDoesWhatItLeft(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	changed := writeTrack(t, filepath.Join(source, "changed.mp3"), "Before")
	gone := writeTrack(t, filepath.Join(source, "gone.mp3"), "Gone")
	f.scan(t, source)

	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	added := writeTrack(t, filepath.Join(source, "added.mp3"), "Added")
	writeTrack(t, changed, "After")
	later(t, changed)
	lifts := []func(){f.refuse(t, insert), f.refuse(t, update), f.refuse(t, remove)}
	_ = f.scanFailing(t, source) // the first of the three writes fails, whichever it is
	for _, lift := range lifts {
		lift()
	}
	stats := f.scan(t, source)

	got := stats.BySource[source]
	if !slices.Equal(got.Added, []string{"added.mp3"}) ||
		!slices.Equal(got.Updated, []string{"changed.mp3"}) ||
		!slices.Equal(got.Removed, []string{"gone.mp3"}) {
		t.Errorf("stats = %+v, want added.mp3 added, changed.mp3 updated, gone.mp3 removed", got)
	}
	if f.trackID(t, added) == 0 || f.trackID(t, gone) != 0 {
		t.Error("the library does not hold added.mp3 without gone.mp3")
	}
	if track, err := f.lib.TrackByPath(changed); err != nil || track.Title != "After" {
		t.Errorf("track = %+v (err %v), want its new title After", track, err)
	}
}

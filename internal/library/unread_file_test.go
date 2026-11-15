package library_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// fullScan runs a FullRefresh of sources to its end and returns its stats.
func (f scanFixture) fullScan(t *testing.T, sources ...string) *library.ScanStats {
	t.Helper()
	return f.scanBy(t, f.lib.FullRefresh, nil, sources...)
}

// track returns the track at path, failing the test when the library has none.
func (f scanFixture) track(t *testing.T, path string) library.Track {
	t.Helper()
	track, err := f.lib.TrackByPath(path)
	if err != nil || track == nil {
		t.Fatalf("track %s = %v (err %v), want one", path, track, err)
	}
	return *track
}

// givesNoTrack changes the file at path so that a scan gets no track from it.
type givesNoTrack struct {
	name   string
	change func(t *testing.T, path string)
}

var filesThatGiveNoTrack = []givesNoTrack{
	{"no artist", func(t *testing.T, path string) {
		tagstest.WriteMP3(t, path, tags.Tag{Album: "Trust", Title: "Changed"})
	}},
	{"no album", func(t *testing.T, path string) {
		tagstest.WriteMP3(t, path, tags.Tag{Artist: "Low", AlbumArtist: "Low", Title: "Changed"})
	}},
	{"not an mp3", func(t *testing.T, path string) {
		if err := os.WriteFile(path, []byte("not an mp3"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}},
	{"cannot be opened", func(t *testing.T, path string) {
		unreadable(t, path, 0)
	}},
}

func TestRefresh_KeepsAndListsTheTrackOfAChangedFileThatGivesNoTrack(t *testing.T) {
	for _, tc := range filesThatGiveNoTrack {
		t.Run(tc.name, func(t *testing.T) {
			f := newScanFixture(t)
			source := t.TempDir()
			changed := writeTrack(t, filepath.Join(source, "changed.mp3"), "Before")
			updated := writeTrack(t, filepath.Join(source, "updated.mp3"), "Before")
			gone := writeTrack(t, filepath.Join(source, "gone.mp3"), "Gone")
			writeTrack(t, filepath.Join(source, "kept.mp3"), "Kept")
			f.scan(t, source)
			before := f.track(t, changed)
			playlist := f.playlistOf(t, changed)

			tc.change(t, changed)
			later(t, changed)
			writeTrack(t, updated, "After")
			later(t, updated)
			if err := os.Remove(gone); err != nil {
				t.Fatalf("remove: %v", err)
			}
			writeTrack(t, filepath.Join(source, "added.mp3"), "Added")
			stats := f.scan(t, source)

			got := stats.BySource[source]
			if !slices.Equal(got.Unread, []string{"changed.mp3"}) {
				t.Errorf("unread = %v, want [changed.mp3]", got.Unread)
			}
			if !slices.Equal(got.Added, []string{"added.mp3"}) ||
				!slices.Equal(got.Updated, []string{"updated.mp3"}) ||
				!slices.Equal(got.Removed, []string{"gone.mp3"}) {
				t.Errorf("stats = %+v, want added.mp3 added, updated.mp3 updated, gone.mp3 removed", got)
			}
			if after := f.track(t, changed); after != before {
				t.Errorf("track = %+v, want it untouched: %+v", after, before)
			}
			if got := f.entries(t, playlist); !slices.Equal(got, []string{changed}) {
				t.Errorf("playlist = %v, want [%s]", got, changed)
			}

			if got := f.scan(t, source).BySource[source]; !slices.Equal(got.Unread, []string{"changed.mp3"}) ||
				len(got.Added)+len(got.Updated)+len(got.Removed) != 0 {
				t.Errorf("second scan stats = %+v, want changed.mp3 unread again and nothing else", got)
			}
			if got := f.fullScan(t, source).BySource[source]; !slices.Equal(got.Unread, []string{"changed.mp3"}) {
				t.Errorf("full scan unread = %v, want [changed.mp3]", got.Unread)
			}
			if after := f.track(t, changed); after != before {
				t.Errorf("track after the later scans = %+v, want it untouched: %+v", after, before)
			}
		})
	}
}

func TestRefresh_UpdatesAKeptTrackOnceItsFileGivesATrackAgain(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	changed := writeTrack(t, filepath.Join(source, "changed.mp3"), "Before")
	f.scan(t, source)
	before := f.track(t, changed)
	playlist := f.playlistOf(t, changed)

	tagstest.WriteMP3(t, changed, tags.Tag{Album: "Trust", Title: "Stripped"})
	later(t, changed)
	f.scan(t, source)

	writeTrack(t, changed, "After")
	later(t, changed)
	stats := f.scan(t, source)

	if got := stats.BySource[source]; !slices.Equal(got.Updated, []string{"changed.mp3"}) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want changed.mp3 updated and nothing unread", got)
	}
	after := f.track(t, changed)
	if after.ID != before.ID || after.Title != "After" || after.Mtime == before.Mtime {
		t.Errorf("track = %+v, want ID %d, title After and a new mtime (was %d)", after, before.ID, before.Mtime)
	}
	if got := f.entries(t, playlist); !slices.Equal(got, []string{changed}) {
		t.Errorf("playlist = %v, want [%s]", got, changed)
	}
}

func TestRefresh_ANewFileThatGivesNoTrackIsNotListed(t *testing.T) {
	for _, tc := range filesThatGiveNoTrack {
		t.Run(tc.name, func(t *testing.T) {
			f := newScanFixture(t)
			source := t.TempDir()
			writeTrack(t, filepath.Join(source, "kept.mp3"), "Kept")
			f.scan(t, source)

			fresh := writeTrack(t, filepath.Join(source, "new.mp3"), "New")
			tc.change(t, fresh)
			stats := f.scan(t, source)

			if f.trackID(t, fresh) != 0 {
				t.Error("new.mp3 is in the library, want no track for it")
			}
			if got := stats.BySource[source]; len(got.Unread)+len(got.Added)+len(got.Updated)+len(got.Removed) != 0 {
				t.Errorf("stats = %+v, want nothing listed", got)
			}
		})
	}
}

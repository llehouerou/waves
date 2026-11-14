package library_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// scanFixture is a library on the real schema, with its playlists.
type scanFixture struct {
	db        *sql.DB
	lib       *library.Library
	playlists *playlists.Playlists
}

func newScanFixture(t *testing.T) scanFixture {
	t.Helper()
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "waves.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	lib := library.New(st.DB())
	return scanFixture{db: st.DB(), lib: lib, playlists: playlists.New(st.DB(), lib)}
}

// scan runs a scan of sources to its end and returns its stats.
func (f scanFixture) scan(t *testing.T, sources ...string) *library.ScanStats {
	t.Helper()
	return f.scanWith(t, nil, sources...)
}

// scanWith is scan with stop given to the scan.
func (f scanFixture) scanWith(t *testing.T, stop func() bool, sources ...string) *library.ScanStats {
	t.Helper()
	stats, err := f.run(stop, sources...)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats == nil {
		t.Fatal("the scan reported no stats")
	}
	return stats
}

// run runs a scan of sources to its end, with stop given to it, and returns
// the stats it reported and its error.
func (f scanFixture) run(stop func() bool, sources ...string) (*library.ScanStats, error) {
	reports, err := f.reports(stop, sources...)
	var stats *library.ScanStats
	for _, p := range reports {
		if p.Stats != nil {
			stats = p.Stats
		}
	}
	return stats, err
}

// reports runs a scan of sources to its end, with stop given to it, and
// returns the progress it reported and its error.
func (f scanFixture) reports(stop func() bool, sources ...string) ([]library.ScanProgress, error) {
	progress := make(chan library.ScanProgress)
	done := make(chan error, 1)
	go func() { done <- f.lib.Refresh(sources, progress, stop) }()
	var reports []library.ScanProgress
	for p := range progress {
		reports = append(reports, p)
	}
	return reports, <-done
}

// trackID returns the ID of the track at path, 0 when the library has none.
func (f scanFixture) trackID(t *testing.T, path string) int64 {
	t.Helper()
	track, err := f.lib.TrackByPath(path)
	if err != nil || track == nil {
		return 0
	}
	return track.ID
}

// playlistOf puts the tracks at paths in a new playlist and returns its ID.
func (f scanFixture) playlistOf(t *testing.T, paths ...string) int64 {
	t.Helper()
	id, err := f.playlists.Create(nil, "Mix")
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	ids := make([]int64, 0, len(paths))
	for _, path := range paths {
		trackID := f.trackID(t, path)
		if trackID == 0 {
			t.Fatalf("no track for %s", path)
		}
		ids = append(ids, trackID)
	}
	if err := f.playlists.AddTracks(id, ids); err != nil {
		t.Fatalf("add tracks: %v", err)
	}
	return id
}

// entries returns the paths of the tracks of a playlist, in order.
func (f scanFixture) entries(t *testing.T, playlistID int64) []string {
	t.Helper()
	tracks, err := f.playlists.Tracks(playlistID)
	if err != nil {
		t.Fatalf("playlist tracks: %v", err)
	}
	paths := make([]string, 0, len(tracks))
	for i := range tracks {
		paths = append(paths, tracks[i].Path)
	}
	return paths
}

// ids returns the track IDs of paths, in order.
func (f scanFixture) ids(t *testing.T, paths ...string) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(paths))
	for _, path := range paths {
		ids = append(ids, f.trackID(t, path))
	}
	return ids
}

func writeTrack(t *testing.T, path, title string) string {
	t.Helper()
	tagstest.WriteMP3(t, path, tags.Tag{Artist: "Low", AlbumArtist: "Low", Album: "Trust", Title: title})
	return path
}

// unreadable makes dir unreadable with mode until the test ends.
func unreadable(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads every folder")
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestRefresh_KeepsTheTracksOfASourceThatDoesNotExist(t *testing.T) {
	f := newScanFixture(t)
	source := filepath.Join(t.TempDir(), "music")
	one := writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One")
	two := writeTrack(t, filepath.Join(source, "b", "two.mp3"), "Two")
	f.scan(t, source)
	ids := f.ids(t, one, two)
	mix := f.playlistOf(t, one, two)

	away := source + ".away"
	if err := os.Rename(source, away); err != nil {
		t.Fatalf("rename: %v", err)
	}
	stats := f.scan(t, source)

	if got := f.ids(t, one, two); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{one, two}) {
		t.Errorf("playlist = %v, want both tracks kept", got)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want the source unread and nothing removed", got)
	}

	// The source is back, with one file deleted in the meantime.
	if err := os.Rename(away, source); err != nil {
		t.Fatalf("rename back: %v", err)
	}
	if err := os.Remove(two); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats = f.scan(t, source)

	if got := f.ids(t, one, two); !slices.Equal(got, []int64{ids[0], 0}) {
		t.Errorf("track IDs = %v, want %v: one unchanged, two removed", got, []int64{ids[0], 0})
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{one}) {
		t.Errorf("playlist = %v, want one left", got)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Removed, []string{"b/two.mp3"}) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want b/two.mp3 removed and nothing unread", got)
	}
}

func TestRefresh_KeepsTheTracksOfASourceItCannotList(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	one := writeTrack(t, filepath.Join(source, "one.mp3"), "One")
	f.scan(t, source)
	ids := f.ids(t, one)
	mix := f.playlistOf(t, one)

	unreadable(t, source, 0)
	stats := f.scan(t, source)

	if got := f.ids(t, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{one}) {
		t.Errorf("playlist = %v, want the track kept", got)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want the source unread and nothing removed", got)
	}
}

func TestRefresh_KeepsTheTracksOfAFolderItCannotList(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	locked := writeTrack(t, filepath.Join(source, "locked", "one.mp3"), "One")
	open := writeTrack(t, filepath.Join(source, "open", "two.mp3"), "Two")
	gone := writeTrack(t, filepath.Join(source, "open", "three.mp3"), "Three")
	f.scan(t, source)
	ids := f.ids(t, locked, open)
	mix := f.playlistOf(t, locked, open, gone)

	unreadable(t, filepath.Join(source, "locked"), 0)
	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats := f.scan(t, source)

	if got := f.ids(t, locked, open, gone); !slices.Equal(got, append(ids, 0)) {
		t.Errorf("track IDs = %v, want %v: the unread and the open kept, the deleted removed", got, append(ids, 0))
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{locked, open}) {
		t.Errorf("playlist = %v, want the unread and the open tracks", got)
	}
	got := stats.BySource[source]
	if !slices.Equal(got.Unread, []string{"locked"}) || !slices.Equal(got.Removed, []string{"open/three.mp3"}) {
		t.Errorf("stats = %+v, want locked unread and open/three.mp3 removed", got)
	}
}

func TestRefresh_KeepsTheTracksOfFilesItCannotStat(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	one := writeTrack(t, filepath.Join(source, "dir", "one.mp3"), "One")
	f.scan(t, source)
	ids := f.ids(t, one)

	// Listed, but no file in it can be stat'ed.
	unreadable(t, filepath.Join(source, "dir"), 0o444)
	stats := f.scan(t, source)

	if got := f.ids(t, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{"dir/one.mp3"}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want dir/one.mp3 unread and nothing removed", got)
	}
}

func TestRefresh_KeepsTheTracksOfASourceWhoseRootIsEmpty(t *testing.T) {
	f := newScanFixture(t)
	source := filepath.Join(t.TempDir(), "music")
	one := writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One")
	f.scan(t, source)
	ids := f.ids(t, one)
	mix := f.playlistOf(t, one)

	// What an unmounted mount point looks like.
	if err := os.RemoveAll(source); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stats := f.scan(t, source)

	if got := f.ids(t, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{one}) {
		t.Errorf("playlist = %v, want the track kept", got)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want the source unread and nothing removed", got)
	}
}

func TestRefresh_AnEmptySourceWithNoTrackIsNotUnread(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()

	stats := f.scan(t, source)

	if got := stats.BySource[source]; len(got.Unread) != 0 {
		t.Errorf("unread = %v, want none", got.Unread)
	}
}

func TestRefresh_AMissingSourceLeavesTheOtherSourcesScanned(t *testing.T) {
	f := newScanFixture(t)
	missing := filepath.Join(t.TempDir(), "nas")
	present := t.TempDir()
	kept := writeTrack(t, filepath.Join(missing, "kept.mp3"), "Kept")
	changed := writeTrack(t, filepath.Join(present, "changed.mp3"), "Changed")
	gone := writeTrack(t, filepath.Join(present, "gone.mp3"), "Gone")
	f.scan(t, missing, present)
	ids := f.ids(t, kept, changed)

	if err := os.RemoveAll(missing); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	writeTrack(t, changed, "Changed again")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(changed, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	added := writeTrack(t, filepath.Join(present, "added.mp3"), "Added")
	stats := f.scan(t, missing, present)

	if got := f.ids(t, kept, changed, gone); !slices.Equal(got, append(ids, 0)) {
		t.Errorf("track IDs = %v, want %v", got, append(ids, 0))
	}
	if f.trackID(t, added) == 0 {
		t.Error("added.mp3 is not in the library")
	}
	if got := stats.BySource[missing]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats of the missing source = %+v, want it unread and nothing removed", got)
	}
	got := stats.BySource[present]
	if len(got.Unread) != 0 ||
		!slices.Equal(got.Added, []string{"added.mp3"}) ||
		!slices.Equal(got.Updated, []string{"changed.mp3"}) ||
		!slices.Equal(got.Removed, []string{"gone.mp3"}) {
		t.Errorf("stats of the present source = %+v, want added.mp3 added, changed.mp3 updated, gone.mp3 removed", got)
	}
}

func TestRefresh_KeepsTheTracksOfAFolderThatVanishesDuringTheWalk(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	one := writeTrack(t, filepath.Join(source, "nas", "one.mp3"), "One")
	f.scan(t, source)
	ids := f.ids(t, one)

	// The scan asks stop before each entry it visits: the source, then nas,
	// which it lists next. A mount that drops between the two.
	calls := 0
	unmount := func() bool {
		calls++
		if calls == 2 {
			if err := os.Rename(filepath.Join(source, "nas"), filepath.Join(t.TempDir(), "away")); err != nil {
				t.Errorf("rename: %v", err)
			}
		}
		return false
	}
	stats := f.scanWith(t, unmount, source)

	if got := f.ids(t, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{"nas"}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want nas unread and nothing removed", got)
	}
}

func TestRefresh_KeepsTheTracksOfASiblingSourceWhosePathStartsWithIt(t *testing.T) {
	cases := []struct {
		name string
		// prepare makes the state of music when it is scanned, and returns
		// the track it holds, if any.
		prepare func(t *testing.T, music string) string
		unread  []string
	}{
		{
			name: "it holds a track of its own",
			prepare: func(t *testing.T, music string) string {
				return writeTrack(t, filepath.Join(music, "own.mp3"), "Own")
			},
		},
		{
			name: "it is an empty directory",
			prepare: func(t *testing.T, music string) string {
				if err := os.Mkdir(music, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				return ""
			},
		},
		{
			name:    "it does not exist",
			prepare: func(*testing.T, string) string { return "" },
			unread:  []string{library.SourceRoot},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newScanFixture(t)
			root := t.TempDir()
			music, music2 := filepath.Join(root, "music"), filepath.Join(root, "music2")
			one := writeTrack(t, filepath.Join(music2, "a", "a.mp3"), "A")
			two := writeTrack(t, filepath.Join(music2, "b", "b.mp3"), "B")
			f.scan(t, music2)
			ids := f.ids(t, one, two)
			mix := f.playlistOf(t, one, two)
			own := tc.prepare(t, music)

			stats := f.scan(t, music)

			if got := f.ids(t, one, two); !slices.Equal(got, ids) {
				t.Errorf("track IDs of music2 = %v, want %v kept", got, ids)
			}
			if got := f.entries(t, mix); !slices.Equal(got, []string{one, two}) {
				t.Errorf("playlist = %v, want both tracks of music2 kept", got)
			}
			if count, err := f.lib.TrackCountBySource(music2); err != nil || count != 2 {
				t.Errorf("tracks of music2 = %d (err %v), want 2", count, err)
			}
			got := stats.BySource[music]
			if len(got.Removed) != 0 || !slices.Equal(got.Unread, tc.unread) {
				t.Errorf("stats of music = %+v, want nothing removed and unread %v", got, tc.unread)
			}
			if own != "" {
				if f.trackID(t, own) == 0 {
					t.Error("the track of music is not in the library")
				}
				if !slices.Equal(got.Added, []string{"own.mp3"}) {
					t.Errorf("added = %v, want own.mp3", got.Added)
				}
			}
		})
	}
}

func TestRefresh_ReportsARemovedTrackUnderItsOwnSourceOnly(t *testing.T) {
	f := newScanFixture(t)
	root := t.TempDir()
	music, music2 := filepath.Join(root, "music"), filepath.Join(root, "music2")
	writeTrack(t, filepath.Join(music, "own.mp3"), "Own")
	kept := writeTrack(t, filepath.Join(music2, "kept.mp3"), "Kept")
	gone := writeTrack(t, filepath.Join(music2, "gone.mp3"), "Gone")
	f.scan(t, music, music2)
	ids := f.ids(t, kept)

	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats := f.scan(t, music, music2)

	if got := f.ids(t, kept, gone); !slices.Equal(got, append(ids, 0)) {
		t.Errorf("track IDs = %v, want %v: kept unchanged, gone removed", got, append(ids, 0))
	}
	if got := stats.BySource[music2].Removed; !slices.Equal(got, []string{"gone.mp3"}) {
		t.Errorf("removed from music2 = %v, want gone.mp3", got)
	}
	if got := stats.BySource[music].Removed; len(got) != 0 {
		t.Errorf("removed from music = %v, want nothing", got)
	}
}

func TestRefresh_CleansASourceGivenWithATrailingSeparator(t *testing.T) {
	f := newScanFixture(t)
	music := filepath.Join(t.TempDir(), "music")
	kept := writeTrack(t, filepath.Join(music, "kept.mp3"), "Kept")
	gone := writeTrack(t, filepath.Join(music, "a", "gone.mp3"), "Gone")
	source := music + string(filepath.Separator)
	f.scan(t, source)
	ids := f.ids(t, kept)

	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats := f.scan(t, source)

	if got := f.ids(t, kept, gone); !slices.Equal(got, append(ids, 0)) {
		t.Errorf("track IDs = %v, want %v: kept unchanged, gone removed", got, append(ids, 0))
	}
	if got := stats.BySource[source].Removed; !slices.Equal(got, []string{"a/gone.mp3"}) {
		t.Errorf("removed = %v, want a/gone.mp3", got)
	}
}

// emptyOfMusic deletes every music file under source, and leaves what a
// failed mount whose mount point is not empty holds: a non-music file and a
// folder.
func emptyOfMusic(t *testing.T, source string) {
	t.Helper()
	if err := os.RemoveAll(source); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(source, "@eaDir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, ".DS_Store"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestRefresh_KeepsTheTracksOfASourceWithNoMusicFileLeft(t *testing.T) {
	cases := []struct {
		name string
		// empty leaves no music file in source, whose tracks are one and two.
		empty func(t *testing.T, source, one, two string)
	}{
		{
			name:  "its root holds a non-music file and a folder",
			empty: func(t *testing.T, source, _, _ string) { emptyOfMusic(t, source) },
		},
		{
			name: "its folders are still there",
			empty: func(t *testing.T, _, one, two string) {
				for _, path := range []string{one, two} {
					if err := os.Remove(path); err != nil {
						t.Fatalf("remove: %v", err)
					}
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newScanFixture(t)
			source := filepath.Join(t.TempDir(), "music")
			one := writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One")
			two := writeTrack(t, filepath.Join(source, "b", "two.mp3"), "Two")
			f.scan(t, source)
			ids := f.ids(t, one, two)
			mix := f.playlistOf(t, one, two)

			tc.empty(t, source, one, two)
			stats := f.scan(t, source)

			if got := f.ids(t, one, two); !slices.Equal(got, ids) {
				t.Errorf("track IDs = %v, want %v kept", got, ids)
			}
			if got := f.entries(t, mix); !slices.Equal(got, []string{one, two}) {
				t.Errorf("playlist = %v, want both tracks kept", got)
			}
			if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
				t.Errorf("stats = %+v, want the source unread and nothing removed", got)
			}
		})
	}
}

func TestRefresh_ASourceWithNoMusicFileAndNoTrackIsNotUnread(t *testing.T) {
	f := newScanFixture(t)
	source := filepath.Join(t.TempDir(), "music")
	emptyOfMusic(t, source)

	stats := f.scan(t, source)

	if got := stats.BySource[source]; len(got.Unread) != 0 {
		t.Errorf("unread = %v, want none", got.Unread)
	}
}

func TestRefresh_ASourceWithNoMusicFileLeavesTheOtherSourcesCleaned(t *testing.T) {
	f := newScanFixture(t)
	root := t.TempDir()
	nas, present := filepath.Join(root, "nas"), filepath.Join(root, "present")
	kept := writeTrack(t, filepath.Join(nas, "kept.mp3"), "Kept")
	stays := writeTrack(t, filepath.Join(present, "stays.mp3"), "Stays")
	gone := writeTrack(t, filepath.Join(present, "gone.mp3"), "Gone")
	f.scan(t, nas, present)
	ids := f.ids(t, kept, stays)

	emptyOfMusic(t, nas)
	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats := f.scan(t, nas, present)

	if got := f.ids(t, kept, stays, gone); !slices.Equal(got, append(ids, 0)) {
		t.Errorf("track IDs = %v, want %v", got, append(ids, 0))
	}
	if got := stats.BySource[nas]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats of nas = %+v, want it unread and nothing removed", got)
	}
	if got := stats.BySource[present]; len(got.Unread) != 0 || !slices.Equal(got.Removed, []string{"gone.mp3"}) {
		t.Errorf("stats of present = %+v, want gone.mp3 removed and nothing unread", got)
	}
}

func TestRefresh_KeepsEveryTrackOfASourceWithNoMusicFileAndAFolderItCannotList(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	locked := writeTrack(t, filepath.Join(source, "locked", "one.mp3"), "One")
	gone := writeTrack(t, filepath.Join(source, "open", "two.mp3"), "Two")
	f.scan(t, source)
	ids := f.ids(t, locked, gone)

	unreadable(t, filepath.Join(source, "locked"), 0)
	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats := f.scan(t, source)

	if got := f.ids(t, locked, gone); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	want := []string{"locked", library.SourceRoot}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, want) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want %v unread and nothing removed", got, want)
	}
}

func TestRefresh_CleansASourceOnceAMusicFileIsBackInIt(t *testing.T) {
	f := newScanFixture(t)
	source := filepath.Join(t.TempDir(), "music")
	one := writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One")
	two := writeTrack(t, filepath.Join(source, "b", "two.mp3"), "Two")
	f.scan(t, source)
	emptyOfMusic(t, source)
	f.scan(t, source)

	back := writeTrack(t, one, "One")
	stats := f.scan(t, source)

	if f.trackID(t, back) == 0 {
		t.Error("one.mp3 is not in the library")
	}
	if f.trackID(t, two) != 0 {
		t.Error("two.mp3 is still in the library")
	}
	if got := stats.BySource[source]; !slices.Equal(got.Removed, []string{"b/two.mp3"}) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want b/two.mp3 removed and nothing unread", got)
	}
}

func TestRefresh_RemovesTheTracksWhoseFilesAreGoneWhenSomeMusicFileIsLeft(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	kept := writeTrack(t, filepath.Join(source, "a", "kept.mp3"), "Kept")
	gone := writeTrack(t, filepath.Join(source, "b", "gone.mp3"), "Gone")
	f.scan(t, source)
	ids := f.ids(t, kept)

	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats := f.scan(t, source)

	if got := f.ids(t, kept, gone); !slices.Equal(got, append(ids, 0)) {
		t.Errorf("track IDs = %v, want %v: kept unchanged, gone removed", got, append(ids, 0))
	}
	if got := stats.BySource[source]; !slices.Equal(got.Removed, []string{"b/gone.mp3"}) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want b/gone.mp3 removed and nothing unread", got)
	}
}

func TestRefresh_AMusicFileWhoseTagsCannotBeReadCountsAsFound(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	gone := writeTrack(t, filepath.Join(source, "a", "gone.mp3"), "Gone")
	f.scan(t, source)

	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "broken.mp3"), []byte("not an mp3"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	stats := f.scan(t, source)

	if f.trackID(t, gone) != 0 {
		t.Error("gone.mp3 is still in the library")
	}
	if f.trackID(t, filepath.Join(source, "broken.mp3")) != 0 {
		t.Error("broken.mp3 is in the library: the test wants a file that gives no track")
	}
	if got := stats.BySource[source]; !slices.Equal(got.Removed, []string{"a/gone.mp3"}) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want a/gone.mp3 removed and nothing unread", got)
	}
}

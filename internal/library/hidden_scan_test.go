package library_test

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/linkfs/linkfstest"
)

// hiddenSource is a source holding a/one.mp3, and hidden music files: a/.two.mp3,
// a/.stversions/one~20240101-000000.mp3 and .Trash-1000/files/old.mp3, and
// a/._one.mp3, a hidden file that is no audio.
type hiddenSource struct {
	source string
	one    string
	hidden []string
}

func newHiddenSource(t *testing.T) hiddenSource {
	t.Helper()
	source := filepath.Join(t.TempDir(), "music")
	s := hiddenSource{
		source: source,
		one:    writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One"),
		hidden: []string{
			writeTrack(t, filepath.Join(source, "a", ".two.mp3"), "Two"),
			writeTrack(t, filepath.Join(source, "a", ".stversions", "one~20240101-000000.mp3"), "One old"),
			writeTrack(t, filepath.Join(source, ".Trash-1000", "files", "old.mp3"), "Old"),
		},
	}
	if err := os.WriteFile(filepath.Join(source, "a", "._one.mp3"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return s
}

// holdTracks puts the tracks of paths in the library, as an import does,
// whatever their names: what a scan made before it left out hidden entries.
func (f scanFixture) holdTracks(t *testing.T, paths ...string) {
	t.Helper()
	if outside := f.addTracks(t, paths...); len(outside) != 0 {
		t.Fatalf("add tracks: %v left out", outside)
	}
	if got := f.ids(t, paths...); slices.Contains(got, 0) {
		t.Fatalf("track IDs of %v = %v, want a track for each", paths, got)
	}
}

// positions returns the positions of the entries of a playlist, in order.
func (f scanFixture) positions(t *testing.T, playlistID int64) []int {
	t.Helper()
	rows, err := f.db.Query(`SELECT position FROM playlist_tracks WHERE playlist_id = ? ORDER BY position`, playlistID)
	if err != nil {
		t.Fatalf("positions: %v", err)
	}
	defer rows.Close()
	var positions []int
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("position: %v", err)
		}
		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("positions: %v", err)
	}
	return positions
}

// scanActingAfter scans sources and calls act once the scan has asked stop
// more than asked times: the scan asks stop at each path it walks, then before
// each file it reads, then once before it removes the tracks whose file is gone.
func (f scanFixture) scanActingAfter(t *testing.T, asked int64, act func(), sources ...string) *library.ScanStats {
	t.Helper()
	var calls atomic.Int64
	var once sync.Once
	stop := func() bool {
		if calls.Add(1) > asked {
			once.Do(act)
		}
		return false
	}
	return f.scanWith(t, stop, sources...)
}

// scans are the ways to scan a library, as a refreshFunc of the fixture.
var scans = []struct {
	name    string
	refresh func(f scanFixture) refreshFunc
}{
	{name: "Refresh", refresh: func(f scanFixture) refreshFunc { return f.lib.Refresh }},
	{name: "FullRefresh", refresh: func(f scanFixture) refreshFunc { return f.lib.FullRefresh }},
	{name: "RefreshSource", refresh: func(f scanFixture) refreshFunc {
		return func(sources []string, progress chan<- library.ScanProgress, stop func() bool) error {
			return f.lib.RefreshSource(sources[0], progress, stop)
		}
	}},
}

func TestScan_LeavesOutTheHiddenEntriesOfASource(t *testing.T) {
	for _, tc := range scans {
		t.Run(tc.name, func(t *testing.T) {
			f := newScanFixture(t)
			s := newHiddenSource(t)
			f.add(t, s.source)

			reports, err := f.reportsBy(tc.refresh(f), nil, s.source)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}

			if f.trackID(t, s.one) == 0 {
				t.Error("a/one.mp3 is not in the library")
			}
			for _, path := range s.hidden {
				if f.trackID(t, path) != 0 {
					t.Errorf("%s is in the library", path)
				}
			}
			var stats *library.ScanStats
			for _, p := range reports {
				if p.Phase == "processing" && p.Total != 1 {
					t.Errorf("processing total = %d, want 1", p.Total)
				}
				if p.Stats != nil {
					stats = p.Stats
				}
			}
			if stats == nil {
				t.Fatal("the scan reported no stats")
			}
			got := stats.BySource[s.source]
			if !slices.Equal(got.Added, []string{"a/one.mp3"}) || len(got.Unread) != 0 || len(got.Updated) != 0 {
				t.Errorf("stats = %+v, want a/one.mp3 alone added and nothing unread", got)
			}
		})
	}
}

func TestRefresh_LeavesOutAHiddenSymbolicLink(t *testing.T) {
	f := newScanFixture(t)
	source := filepath.Join(t.TempDir(), "music")
	one := writeTrack(t, filepath.Join(source, "one.mp3"), "One")
	elsewhere := t.TempDir()
	writeTrack(t, filepath.Join(elsewhere, "folder", "linked.mp3"), "Linked")
	toFile := linkfstest.Link(t, filepath.Join(elsewhere, "folder", "linked.mp3"), filepath.Join(source, ".file.mp3"))
	toFolder := linkfstest.Link(t, filepath.Join(elsewhere, "folder"), filepath.Join(source, ".folder"))
	// A link to nothing: no file the scan could not read.
	dangling := linkfstest.Link(t, filepath.Join(elsewhere, "nothing.mp3"), filepath.Join(source, ".dangling.mp3"))

	stats := f.scan(t, source)

	if f.trackID(t, one) == 0 {
		t.Error("one.mp3 is not in the library")
	}
	for _, path := range []string{toFile, filepath.Join(toFolder, "linked.mp3"), dangling} {
		if f.trackID(t, path) != 0 {
			t.Errorf("%s is in the library", path)
		}
	}
	if got := stats.BySource[source]; !slices.Equal(got.Added, []string{"one.mp3"}) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want one.mp3 alone added and nothing unread", got)
	}
}

func TestRefresh_TakesTheRootOfASourceWhateverItsName(t *testing.T) {
	t.Run("a source named with a dot", func(t *testing.T) {
		f := newScanFixture(t)
		source := filepath.Join(t.TempDir(), ".music")
		one := writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One")
		two := writeTrack(t, filepath.Join(source, ".b", "two.mp3"), "Two")

		stats := f.scan(t, source)

		if f.trackID(t, one) == 0 || f.trackID(t, two) != 0 {
			t.Errorf("track IDs of a/one.mp3 and .b/two.mp3 = %v, want one alone", f.ids(t, one, two))
		}
		if got := stats.BySource[source].Added; !slices.Equal(got, []string{"a/one.mp3"}) {
			t.Errorf("added = %v, want a/one.mp3", got)
		}
	})
	t.Run("a source in a folder named with a dot", func(t *testing.T) {
		f := newScanFixture(t)
		source := filepath.Join(t.TempDir(), ".local", "share", "music")
		one := writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One")

		stats := f.scan(t, source)

		if f.trackID(t, one) == 0 {
			t.Error("a/one.mp3 is not in the library")
		}
		if got := stats.BySource[source].Added; !slices.Equal(got, []string{"a/one.mp3"}) {
			t.Errorf("added = %v, want a/one.mp3", got)
		}
	})
}

func TestRefresh_RemovesTheTracksAtHiddenPaths(t *testing.T) {
	f := newScanFixture(t)
	s := newHiddenSource(t)
	two, old := s.hidden[0], s.hidden[2]
	f.add(t, s.source)
	f.holdTracks(t, s.one, two, old)
	id := f.trackID(t, s.one)
	mix := f.playlistOf(t, two, s.one, old)

	stats := f.scan(t, s.source)

	if got := f.ids(t, s.one, two, old); !slices.Equal(got, []int64{id, 0, 0}) {
		t.Errorf("track IDs = %v, want %v: one kept, the hidden ones removed", got, []int64{id, 0, 0})
	}
	got := stats.BySource[s.source]
	if want := []string{".Trash-1000/files/old.mp3", "a/.two.mp3"}; !slices.Equal(sorted(got.Removed), want) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want %v removed and nothing unread", got, want)
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{s.one}) {
		t.Errorf("playlist = %v, want one alone", got)
	}
	if got := f.positions(t, mix); !slices.Equal(got, []int{0}) {
		t.Errorf("playlist positions = %v, want [0]", got)
	}
	results, err := f.lib.SearchFTS("")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, r := range describe(results) {
		if r == "track Two" || r == "track Old" {
			t.Errorf("search finds %q", r)
		}
	}
}

func TestRefresh_RemovesTheTracksOfASourceWhoseOnlyMusicFilesAreHidden(t *testing.T) {
	f := newScanFixture(t)
	s := newHiddenSource(t)
	two, old := s.hidden[0], s.hidden[2]
	f.add(t, s.source)
	f.holdTracks(t, s.one, two, old)
	if err := os.Remove(s.one); err != nil {
		t.Fatalf("remove: %v", err)
	}

	stats := f.scan(t, s.source)

	if got := f.ids(t, s.one, two, old); !slices.Equal(got, []int64{0, 0, 0}) {
		t.Errorf("track IDs = %v, want every track removed", got)
	}
	got := stats.BySource[s.source]
	if want := []string{".Trash-1000/files/old.mp3", "a/.two.mp3", "a/one.mp3"}; !slices.Equal(sorted(got.Removed), want) || len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want %v removed and nothing unread", got, want)
	}
}

func TestRefresh_KeepsTheTracksAtHiddenPathsOfASourceWithNoMusicFile(t *testing.T) {
	f := newScanFixture(t)
	s := newHiddenSource(t)
	two, old := s.hidden[0], s.hidden[2]
	f.add(t, s.source)
	f.holdTracks(t, s.one, two, old)
	ids := f.ids(t, s.one, two, old)

	emptyOfMusic(t, s.source)
	stats := f.scan(t, s.source)

	if got := f.ids(t, s.one, two, old); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[s.source]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want the source unread and nothing removed", got)
	}
}

func TestRefresh_CutShortRemovesNoTrackAtAHiddenPath(t *testing.T) {
	f := newScanFixture(t)
	s := newHiddenSource(t)
	two := s.hidden[0]
	f.add(t, s.source)
	f.holdTracks(t, s.one, two)
	ids := f.ids(t, s.one, two)

	if _, err := f.run(func() bool { return true }, s.source); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if got := f.ids(t, s.one, two); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
}

// hiddenInner writes music/top.mp3 and music/.inner/one.mp3, and returns
// music, music/.inner and the two files.
func hiddenInner(t *testing.T) (music, inner, top, one string) {
	t.Helper()
	music = filepath.Join(t.TempDir(), "music")
	inner = filepath.Join(music, ".inner")
	top = writeTrack(t, filepath.Join(music, "top.mp3"), "Top")
	one = writeTrack(t, filepath.Join(inner, "one.mp3"), "One")
	return music, inner, top, one
}

func TestRefresh_ScansASourceInAHiddenFolder(t *testing.T) {
	t.Run("with the outer source", func(t *testing.T) {
		f := newScanFixture(t)
		music, inner, top, one := hiddenInner(t)

		stats := f.scan(t, music, inner)

		if got := f.ids(t, top, one); slices.Contains(got, 0) {
			t.Errorf("track IDs = %v, want both tracks", got)
		}
		if got := stats.BySource[inner].Added; !slices.Equal(got, []string{"one.mp3"}) {
			t.Errorf("added under .inner = %v, want one.mp3", got)
		}
		if got := stats.BySource[music].Added; !slices.Equal(got, []string{"top.mp3"}) {
			t.Errorf("added under music = %v, want top.mp3", got)
		}
	})
	t.Run("the outer source alone", func(t *testing.T) {
		f := newScanFixture(t)
		music, inner, top, one := hiddenInner(t)
		f.scan(t, music, inner)
		ids := f.ids(t, top, one)

		added := writeTrack(t, filepath.Join(inner, "new.mp3"), "New")
		stats := f.scan(t, music)

		if got := f.ids(t, top, one); !slices.Equal(got, ids) {
			t.Errorf("track IDs = %v, want %v kept", got, ids)
		}
		if f.trackID(t, added) == 0 {
			t.Error(".inner/new.mp3 is not in the library")
		}
		got := stats.BySource[music]
		if !slices.Equal(got.Added, []string{".inner/new.mp3"}) || len(got.Removed) != 0 || len(got.Unread) != 0 {
			t.Errorf("stats of music = %+v, want .inner/new.mp3 added, nothing removed or unread", got)
		}
	})
}

func TestRefresh_LeavesOutAHiddenFileNextToASourceInAHiddenFolder(t *testing.T) {
	cases := []struct {
		name  string
		outer bool
	}{{name: "with the inner source"}, {name: "the outer source alone", outer: true}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newScanFixture(t)
			music := filepath.Join(t.TempDir(), "music")
			inner := filepath.Join(music, ".h", "inner")
			one := writeTrack(t, filepath.Join(inner, "one.mp3"), "One")
			other := writeTrack(t, filepath.Join(music, ".h", "other.mp3"), "Other")
			f.add(t, music, inner)

			sources := []string{music, inner}
			if tc.outer {
				sources = []string{music}
			}
			stats := f.scan(t, sources...)

			if f.trackID(t, one) == 0 {
				t.Error(".h/inner/one.mp3 is not in the library")
			}
			if f.trackID(t, other) != 0 {
				t.Error(".h/other.mp3 is in the library")
			}
			for _, src := range sources {
				if got := stats.BySource[src]; slices.Contains(got.Added, ".h/other.mp3") || len(got.Unread) != 0 {
					t.Errorf("stats of %s = %+v, want .h/other.mp3 not added and nothing unread", src, got)
				}
			}
		})
	}
}

func TestRefresh_KeepsTheTracksOfAHiddenFolderMadeASourceDuringTheScan(t *testing.T) {
	f := newScanFixture(t)
	music := filepath.Join(t.TempDir(), "music")
	top := writeTrack(t, filepath.Join(music, "a", "top.mp3"), "Top")
	hidden := filepath.Join(music, ".h")
	one := writeTrack(t, filepath.Join(hidden, "one.mp3"), "One")
	f.scan(t, music)
	f.holdTracks(t, one)
	ids := f.ids(t, top, one)

	// The walk: music, .h, a, a/top.mp3; top.mp3 is unchanged, none is read.
	f.scanActingAfter(t, 4, func() { f.add(t, hidden) }, music)

	if got := f.ids(t, top, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
}

func TestRefresh_RemovesTheTracksOfAHiddenFolderItCannotList(t *testing.T) {
	f := newScanFixture(t)
	music := filepath.Join(t.TempDir(), "music")
	top := writeTrack(t, filepath.Join(music, "top.mp3"), "Top")
	locked := writeTrack(t, filepath.Join(music, ".h", "one.mp3"), "One")
	f.add(t, music)
	f.holdTracks(t, top, locked)
	id := f.trackID(t, top)

	unreadable(t, filepath.Join(music, ".h"), 0)
	stats := f.scan(t, music)

	if got := f.ids(t, top, locked); !slices.Equal(got, []int64{id, 0}) {
		t.Errorf("track IDs = %v, want top kept and .h/one.mp3 removed", got)
	}
	if got := stats.BySource[music]; len(got.Unread) != 0 || !slices.Equal(got.Removed, []string{".h/one.mp3"}) {
		t.Errorf("stats = %+v, want .h/one.mp3 removed and nothing unread", got)
	}
}

func TestRefresh_ReportsAHiddenFolderItCannotListThatHoldsASource(t *testing.T) {
	f := newScanFixture(t)
	music := filepath.Join(t.TempDir(), "music")
	top := writeTrack(t, filepath.Join(music, "top.mp3"), "Top")
	inner := filepath.Join(music, ".h", "inner")
	one := writeTrack(t, filepath.Join(inner, "one.mp3"), "One")
	f.scan(t, music, inner)
	ids := f.ids(t, top, one)

	unreadable(t, filepath.Join(music, ".h"), 0)
	stats := f.scan(t, music)

	if got := f.ids(t, top, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[music]; !slices.Equal(got.Unread, []string{".h"}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want .h unread and nothing removed", got)
	}
}

func TestRefresh_AHiddenMusicFileOfALinkedInnerSourceCountsForTheOuterOne(t *testing.T) {
	f := newScanFixture(t)
	root := t.TempDir()
	music := filepath.Join(root, "music")
	if err := os.Mkdir(music, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(root, "real")
	writeTrack(t, filepath.Join(target, ".h", "old.mp3"), "Old")
	inner := linkfstest.Link(t, target, filepath.Join(music, "inner"))
	old := filepath.Join(inner, ".h", "old.mp3")
	f.add(t, music, inner)
	f.holdTracks(t, old)

	stats := f.scan(t, music, inner)

	if f.trackID(t, old) != 0 {
		t.Error("inner/.h/old.mp3 is still in the library")
	}
	if got := stats.BySource[inner]; !slices.Equal(got.Removed, []string{".h/old.mp3"}) || len(got.Unread) != 0 {
		t.Errorf("stats of inner = %+v, want .h/old.mp3 removed and nothing unread", got)
	}
	if got := stats.BySource[music]; len(got.Unread) != 0 {
		t.Errorf("unread under music = %v, want nothing", got.Unread)
	}
}

package library_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// nestedLibrary is a library whose sources are music and music/sub, with the
// tracks of music/top.mp3 and music/sub/one.mp3 in the playlist mix, by two
// artists, and its search index built.
type nestedLibrary struct {
	scanFixture
	music, sub string
	top, one   string
	trackIDs   []int64
	mix        int64
}

func newNestedLibrary(t *testing.T) nestedLibrary {
	t.Helper()
	f := newScanFixture(t)
	music, sub, top, one := nestedSources(t)
	for _, src := range []string{music, sub} {
		if _, err := f.lib.AddSource(src); err != nil {
			t.Fatalf("add source: %v", err)
		}
	}
	f.scan(t, music, sub)
	if err := f.lib.RebuildFTSIndex(); err != nil {
		t.Fatalf("rebuild search index: %v", err)
	}
	return nestedLibrary{
		scanFixture: f,
		music:       music, sub: sub,
		top: top, one: one,
		trackIDs: f.ids(t, top, one),
		mix:      f.playlistOf(t, top, one),
	}
}

func (n nestedLibrary) search(t *testing.T) []string {
	t.Helper()
	results, err := n.lib.SearchFTS("")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return describe(results)
}

func (n nestedLibrary) count(t *testing.T, source string) int {
	t.Helper()
	count, err := n.lib.TrackCountBySource(source)
	if err != nil {
		t.Fatalf("track count of %s: %v", source, err)
	}
	return count
}

func (n nestedLibrary) remove(t *testing.T, source string) {
	t.Helper()
	if err := n.lib.RemoveSource(source); err != nil {
		t.Fatalf("remove source %s: %v", source, err)
	}
}

func (n nestedLibrary) sources(t *testing.T) []string {
	t.Helper()
	sources, err := n.lib.Sources()
	if err != nil {
		t.Fatalf("sources: %v", err)
	}
	return sources
}

func TestRemoveSource_OfAnInnerSourceKeepsTheTracksTheOuterOneHolds(t *testing.T) {
	n := newNestedLibrary(t)

	if got := n.count(t, n.sub); got != 0 {
		t.Errorf("TrackCountBySource(sub) = %d, want 0", got)
	}
	n.remove(t, n.sub)

	if got := n.ids(t, n.top, n.one); !slices.Equal(got, n.trackIDs) {
		t.Errorf("track IDs = %v, want %v kept", got, n.trackIDs)
	}
	if got := n.entries(t, n.mix); !slices.Equal(got, []string{n.top, n.one}) {
		t.Errorf("playlist = %v, want both tracks", got)
	}
	want := []string{"artist Sub", "artist Top", "album Sub/Low", "album Top/High", "track Down", "track Up"}
	if got := n.search(t); !slices.Equal(sorted(got), sorted(want)) {
		t.Errorf("search index = %v, want %v", got, want)
	}
	if got := n.sources(t); !slices.Equal(got, []string{n.music}) {
		t.Errorf("sources = %v, want %v", got, []string{n.music})
	}
}

func TestRemoveSource_OfAnOuterSourceKeepsTheTracksTheInnerOneHolds(t *testing.T) {
	n := newNestedLibrary(t)

	if got := n.count(t, n.music); got != 1 {
		t.Errorf("TrackCountBySource(music) = %d, want 1", got)
	}
	n.remove(t, n.music)

	if got := n.ids(t, n.top, n.one); !slices.Equal(got, []int64{0, n.trackIDs[1]}) {
		t.Errorf("track IDs = %v, want %v: top removed, one kept", got, []int64{0, n.trackIDs[1]})
	}
	if got := n.entries(t, n.mix); !slices.Equal(got, []string{n.one}) {
		t.Errorf("playlist = %v, want one alone", got)
	}
	if err := n.playlists.RemoveTrack(n.mix, 0); err != nil {
		t.Fatalf("remove the track at position 0: %v", err)
	}
	if got := n.entries(t, n.mix); len(got) != 0 {
		t.Errorf("playlist after removing position 0 = %v, want one to have been there", got)
	}
	want := []string{"artist Sub", "album Sub/Low", "track Down"}
	if got := n.search(t); !slices.Equal(got, want) {
		t.Errorf("search index = %v, want %v", got, want)
	}
	if got := n.sources(t); !slices.Equal(got, []string{n.sub}) {
		t.Errorf("sources = %v, want %v", got, []string{n.sub})
	}
}

func TestRemoveSource_HoldsAtAnyDepth(t *testing.T) {
	setup := func(t *testing.T) (scanFixture, []string, []string) {
		t.Helper()
		f := newScanFixture(t)
		a := filepath.Join(t.TempDir(), "a")
		b := filepath.Join(a, "b")
		c := filepath.Join(b, "c")
		files := []string{
			writeTrack(t, filepath.Join(a, "1.mp3"), "One"),
			writeTrack(t, filepath.Join(b, "2.mp3"), "Two"),
			writeTrack(t, filepath.Join(c, "3.mp3"), "Three"),
		}
		for _, src := range []string{a, b, c} {
			if _, err := f.lib.AddSource(src); err != nil {
				t.Fatalf("add source: %v", err)
			}
		}
		f.scan(t, a, b, c)
		return f, []string{a, b, c}, files
	}

	t.Run("removing the middle one deletes nothing", func(t *testing.T) {
		f, sources, files := setup(t)
		ids := f.ids(t, files...)
		if count, err := f.lib.TrackCountBySource(sources[1]); err != nil || count != 0 {
			t.Errorf("TrackCountBySource(a/b) = %d (err %v), want 0", count, err)
		}
		if err := f.lib.RemoveSource(sources[1]); err != nil {
			t.Fatalf("remove source: %v", err)
		}
		if got := f.ids(t, files...); !slices.Equal(got, ids) {
			t.Errorf("track IDs = %v, want %v kept", got, ids)
		}
	})

	t.Run("removing the outer one deletes the tracks outside a/b only", func(t *testing.T) {
		f, sources, files := setup(t)
		ids := f.ids(t, files...)
		if count, err := f.lib.TrackCountBySource(sources[0]); err != nil || count != 1 {
			t.Errorf("TrackCountBySource(a) = %d (err %v), want 1", count, err)
		}
		if err := f.lib.RemoveSource(sources[0]); err != nil {
			t.Fatalf("remove source: %v", err)
		}
		if got, want := f.ids(t, files...), []int64{0, ids[1], ids[2]}; !slices.Equal(got, want) {
			t.Errorf("track IDs = %v, want %v", got, want)
		}
	})
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

// nestedSources writes music/top.mp3, by Top, and music/sub/one.mp3, by Sub,
// and returns music, music/sub and the two files.
func nestedSources(t *testing.T) (music, sub, top, one string) {
	t.Helper()
	music = filepath.Join(t.TempDir(), "music")
	sub = filepath.Join(music, "sub")
	top = filepath.Join(music, "top.mp3")
	one = filepath.Join(sub, "one.mp3")
	tagstest.WriteMP3(t, top, tags.Tag{Artist: "Top", AlbumArtist: "Top", Album: "High", Title: "Up"})
	tagstest.WriteMP3(t, one, tags.Tag{Artist: "Sub", AlbumArtist: "Sub", Album: "Low", Title: "Down"})
	return music, sub, top, one
}

// scanOrder is a way to give a scan music and music/sub.
type scanOrder struct {
	name       string
	innerFirst bool
	// trailing spells music/sub with a trailing separator.
	trailing bool
}

// nestedOrders are the orders a scan can be given music and music/sub in.
var nestedOrders = []scanOrder{
	{name: "outer first"},
	{name: "inner first", innerFirst: true},
}

// sources returns music/sub as the scan spells it, and the sources it scans.
func (o scanOrder) sources(music, sub string) (inner string, sources []string) {
	if o.trailing {
		sub += string(filepath.Separator)
	}
	if o.innerFirst {
		return sub, []string{sub, music}
	}
	return sub, []string{music, sub}
}

// scanTotals runs a scan of sources and returns its stats with the totals of
// its processing phase and of its done report.
func (f scanFixture) scanTotals(t *testing.T, sources ...string) (stats *library.ScanStats, processing, done int) {
	t.Helper()
	reports, err := f.reports(nil, sources...)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, p := range reports {
		switch p.Phase {
		case "processing":
			processing = p.Total
		case "done":
			stats, done = p.Stats, p.Total
		}
	}
	return stats, processing, done
}

func TestRefresh_ReportsEachFileOnceUnderItsInnermostSource(t *testing.T) {
	orders := append(slices.Clone(nestedOrders), scanOrder{name: "inner with a trailing separator", trailing: true})
	for _, order := range orders {
		t.Run(order.name, func(t *testing.T) {
			f := newScanFixture(t)
			music, sub, _, _ := nestedSources(t)
			sub, sources := order.sources(music, sub)

			stats, processing, done := f.scanTotals(t, sources...)

			if got := stats.BySource[music]; !slices.Equal(got.Added, []string{"top.mp3"}) {
				t.Errorf("added under music = %v, want top.mp3", got.Added)
			}
			if got := stats.BySource[sub]; !slices.Equal(got.Added, []string{"one.mp3"}) {
				t.Errorf("added under music/sub = %v, want one.mp3", got.Added)
			}
			if processing != 2 || done != 2 {
				t.Errorf("totals = %d processed, %d done, want 2 and 2", processing, done)
			}
		})
	}
}

func TestRefresh_ReportsAChangedFileOfAnInnerSourceOnceUnderIt(t *testing.T) {
	f := newScanFixture(t)
	music, sub, _, one := nestedSources(t)
	f.scan(t, music, sub)

	writeTrack(t, one, "One again")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(one, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	stats := f.scan(t, music, sub)

	if got := stats.BySource[sub].Updated; !slices.Equal(got, []string{"one.mp3"}) {
		t.Errorf("updated under music/sub = %v, want one.mp3", got)
	}
	if got := stats.BySource[music].Updated; len(got) != 0 {
		t.Errorf("updated under music = %v, want nothing", got)
	}
}

func TestRefresh_ReportsADeletedFileOfAnInnerSourceOnceUnderIt(t *testing.T) {
	for _, order := range nestedOrders {
		t.Run(order.name, func(t *testing.T) {
			f := newScanFixture(t)
			music, sub, _, _ := nestedSources(t)
			gone := writeTrack(t, filepath.Join(sub, "gone.mp3"), "Gone")
			_, sources := order.sources(music, sub)
			f.scan(t, sources...)

			if err := os.Remove(gone); err != nil {
				t.Fatalf("remove: %v", err)
			}
			stats := f.scan(t, sources...)

			if f.trackID(t, gone) != 0 {
				t.Error("gone.mp3 is still in the library")
			}
			if got := stats.BySource[sub].Removed; !slices.Equal(got, []string{"gone.mp3"}) {
				t.Errorf("removed under music/sub = %v, want gone.mp3", got)
			}
			if got := stats.BySource[music].Removed; len(got) != 0 {
				t.Errorf("removed under music = %v, want nothing", got)
			}
		})
	}
}

func TestRefresh_ReportsAnUnreadFolderOfAnInnerSourceOnceUnderIt(t *testing.T) {
	f := newScanFixture(t)
	music, sub, _, _ := nestedSources(t)
	locked := writeTrack(t, filepath.Join(sub, "dir", "locked.mp3"), "Locked")
	f.scan(t, music, sub)
	ids := f.ids(t, locked)

	unreadable(t, filepath.Join(sub, "dir"), 0)
	stats := f.scan(t, music, sub)

	if got := f.ids(t, locked); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[sub].Unread; !slices.Equal(got, []string{"dir"}) {
		t.Errorf("unread under music/sub = %v, want dir", got)
	}
	if got := stats.BySource[music].Unread; len(got) != 0 {
		t.Errorf("unread under music = %v, want nothing", got)
	}
}

func TestRefresh_ReportsAnInnerSourceItCannotListOnceAsItsRoot(t *testing.T) {
	f := newScanFixture(t)
	music, sub, _, one := nestedSources(t)
	f.scan(t, music, sub)
	ids := f.ids(t, one)

	unreadable(t, sub, 0)
	stats := f.scan(t, music, sub)

	if got := f.ids(t, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[sub].Unread; !slices.Equal(got, []string{library.SourceRoot}) {
		t.Errorf("unread under music/sub = %v, want the source itself", got)
	}
	if got := stats.BySource[music].Unread; len(got) != 0 {
		t.Errorf("unread under music = %v, want nothing", got)
	}
}

func TestRefresh_AMusicFileOfAnInnerSourceCountsForTheOuterOne(t *testing.T) {
	f := newScanFixture(t)
	music, sub, top, one := nestedSources(t)
	f.scan(t, music, sub)
	ids := f.ids(t, one)

	if err := os.Remove(top); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stats := f.scan(t, music, sub)

	if got := f.ids(t, top, one); !slices.Equal(got, []int64{0, ids[0]}) {
		t.Errorf("track IDs = %v, want top removed and one kept", got)
	}
	if got := stats.BySource[music].Removed; !slices.Equal(got, []string{"top.mp3"}) {
		t.Errorf("removed under music = %v, want top.mp3", got)
	}
	for _, src := range []string{music, sub} {
		if got := stats.BySource[src].Unread; len(got) != 0 {
			t.Errorf("unread under %s = %v, want nothing", src, got)
		}
	}
}

func TestRefresh_KeepsTheTracksOfAnInnerSourceWithNoMusicFileLeft(t *testing.T) {
	f := newScanFixture(t)
	music, sub, top, one := nestedSources(t)
	f.scan(t, music, sub)
	ids := f.ids(t, top, one)

	emptyOfMusic(t, sub)
	stats := f.scan(t, music, sub)

	if got := f.ids(t, top, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[sub]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats of music/sub = %+v, want it unread and nothing removed", got)
	}
	if got := stats.BySource[music]; len(got.Unread) != 0 || len(got.Removed) != 0 {
		t.Errorf("stats of music = %+v, want nothing unread or removed", got)
	}
}

func TestRefresh_KeepsTheTracksOfNestedSourcesWithNoMusicFileLeftWhateverTheirOrder(t *testing.T) {
	for _, order := range nestedOrders {
		t.Run(order.name, func(t *testing.T) {
			f := newScanFixture(t)
			music, sub, top, one := nestedSources(t)
			_, sources := order.sources(music, sub)
			f.scan(t, sources...)
			ids := f.ids(t, top, one)

			emptyOfMusic(t, music)
			if err := os.Mkdir(sub, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			stats := f.scan(t, sources...)

			if got := f.ids(t, top, one); !slices.Equal(got, ids) {
				t.Errorf("track IDs = %v, want %v kept", got, ids)
			}
			for _, src := range []string{music, sub} {
				if got := stats.BySource[src]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
					t.Errorf("stats of %s = %+v, want it unread and nothing removed", src, got)
				}
			}
		})
	}
}

func TestRefresh_OfAnOuterSourceAloneReportsTheFilesOfTheInnerOneUnderIt(t *testing.T) {
	f := newScanFixture(t)
	music, sub, top, one := nestedSources(t)
	for _, src := range []string{music, sub} {
		if _, err := f.lib.AddSource(src); err != nil {
			t.Fatalf("add source: %v", err)
		}
	}
	f.scan(t, music, sub)
	ids := f.ids(t, top, one)

	added := writeTrack(t, filepath.Join(sub, "new.mp3"), "New")
	stats, processing, _ := f.scanTotals(t, music)

	if got := f.ids(t, top, one); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if f.trackID(t, added) == 0 {
		t.Error("new.mp3 is not in the library")
	}
	if got := stats.BySource[music].Added; !slices.Equal(got, []string{"sub/new.mp3"}) {
		t.Errorf("added under music = %v, want sub/new.mp3", got)
	}
	if processing != 1 {
		t.Errorf("processing total = %d, want 1", processing)
	}
}

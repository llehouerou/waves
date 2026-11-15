package library_test

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/llehouerou/waves/internal/library"
)

// twoSources is a library whose sources are music, holding a.mp3, and
// other, holding o.mp3.
type twoSources struct {
	scanFixture
	music, other string
	a, o         string
}

func newTwoSources(t *testing.T) twoSources {
	t.Helper()
	f := newScanFixture(t)
	root := t.TempDir()
	music, other := filepath.Join(root, "music"), filepath.Join(root, "other")
	s := twoSources{
		scanFixture: f,
		music:       music, other: other,
		a: writeTrack(t, filepath.Join(music, "a.mp3"), "A"),
		o: writeTrack(t, filepath.Join(other, "o.mp3"), "O"),
	}
	s.add(t, music, other)
	return s
}

// remove removes source from the library.
func (f scanFixture) remove(t *testing.T, source string) {
	t.Helper()
	if err := f.lib.RemoveSource(source); err != nil {
		t.Errorf("remove source %s: %v", source, err)
	}
}

// scanRemovingBeforeTheWalk scans sources and removes removed from the
// library before the scan walks them: the scan sends its first report before
// its walk and waits for it to be read.
func (f scanFixture) scanRemovingBeforeTheWalk(t *testing.T, removed string, sources ...string) *library.ScanStats {
	t.Helper()
	progress := make(chan library.ScanProgress)
	done := make(chan error, 1)
	go func() { done <- f.lib.Refresh(sources, progress, nil) }()

	f.remove(t, removed)
	var stats *library.ScanStats
	for p := range progress {
		if p.Stats != nil {
			stats = p.Stats
		}
	}
	if err := <-done; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stats == nil {
		t.Fatal("the scan reported no stats")
	}
	return stats
}

// scanRemovingBeforeReading scans sources and removes removed from the
// library once the scan has walked as many paths as walked (the sources and
// what they hold), before it reads a file: the scan asks stop at each path it
// walks, then before each file it reads.
func (f scanFixture) scanRemovingBeforeReading(t *testing.T, removed string, walked int64, sources ...string) *library.ScanStats {
	t.Helper()
	var calls atomic.Int64
	var once sync.Once
	stop := func() bool {
		if calls.Add(1) > walked {
			once.Do(func() { f.remove(t, removed) })
		}
		return false
	}
	return f.scanWith(t, stop, sources...)
}

func (s twoSources) assertMusicHasNoTrack(t *testing.T, stats *library.ScanStats) {
	t.Helper()
	if s.trackID(t, s.a) != 0 {
		t.Error("a.mp3 of the removed source is in the library")
	}
	if s.trackID(t, s.o) == 0 {
		t.Error("o.mp3 of the other source is not in the library")
	}
	if got := stats.BySource[s.music]; got == nil || len(got.Added) != 0 || len(got.Updated) != 0 {
		t.Errorf("stats of music = %+v, want an entry with nothing added or updated", got)
	}
	if got := stats.BySource[s.other]; got == nil || len(got.Added) != 1 || got.Added[0] != "o.mp3" {
		t.Errorf("stats of other = %+v, want o.mp3 added", got)
	}
}

func TestRefresh_WritesNoTrackOfASourceRemovedBeforeItsWalk(t *testing.T) {
	s := newTwoSources(t)

	stats := s.scanRemovingBeforeTheWalk(t, s.music, s.music, s.other)

	s.assertMusicHasNoTrack(t, stats)
}

func TestRefresh_WritesNoTrackOfASourceRemovedBeforeItsFilesAreRead(t *testing.T) {
	s := newTwoSources(t)

	// The walk: music, a.mp3, other, o.mp3.
	stats := s.scanRemovingBeforeReading(t, s.music, 4, s.music, s.other)

	s.assertMusicHasNoTrack(t, stats)
}

func TestRefresh_DoesNotUpdateTheTrackOfASourceRemovedBeforeItsFileIsRead(t *testing.T) {
	s := newTwoSources(t)
	s.scan(t, s.music, s.other)
	writeTrack(t, s.a, "A changed")
	later(t, s.a)

	// The walk: music, a.mp3, other, o.mp3.
	stats := s.scanRemovingBeforeReading(t, s.music, 4, s.music, s.other)

	if s.trackID(t, s.a) != 0 {
		t.Error("a.mp3 of the removed source is in the library")
	}
	if got := stats.BySource[s.music]; got == nil || len(got.Updated) != 0 || len(got.Added) != 0 {
		t.Errorf("stats of music = %+v, want an entry with nothing updated or added", got)
	}
}

func TestRefresh_DoesNotReportAsRemovedATrackTheRemovalOfItsSourceDeleted(t *testing.T) {
	s := newTwoSources(t)
	// b.mp3 stays: a source with no music file would be taken for an
	// unmounted one, whose tracks a scan keeps.
	b := writeTrack(t, filepath.Join(s.music, "b.mp3"), "B")
	s.scan(t, s.music, s.other)
	if err := os.Remove(s.a); err != nil {
		t.Fatalf("remove a.mp3: %v", err)
	}
	writeTrack(t, s.o, "O changed")
	later(t, s.o)

	// The walk: music, b.mp3, other, o.mp3; o.mp3 alone is read.
	stats := s.scanRemovingBeforeReading(t, s.music, 4, s.music, s.other)

	if s.trackID(t, s.a) != 0 || s.trackID(t, b) != 0 {
		t.Error("a track of the removed source is in the library")
	}
	if got := stats.BySource[s.music]; got == nil || len(got.Removed) != 0 {
		t.Errorf("stats of music = %+v, want an entry with nothing removed", got)
	}
}

func TestRefresh_WritesTheTracksAnOuterSourceStillHoldsWhenTheInnerOneIsRemoved(t *testing.T) {
	f := newScanFixture(t)
	music, sub, top, one := nestedSources(t)
	f.add(t, music, sub)

	stats := f.scanRemovingBeforeTheWalk(t, sub, music, sub)

	if f.trackID(t, top) == 0 || f.trackID(t, one) == 0 {
		t.Error("a track music holds is not in the library")
	}
	if got := stats.BySource[sub]; got == nil || len(got.Added) != 1 || got.Added[0] != "one.mp3" {
		t.Errorf("stats of music/sub = %+v, want one.mp3 added", got)
	}
}

func TestRefresh_WritesTheTracksAnInnerSourceStillHoldsWhenTheOuterOneIsRemoved(t *testing.T) {
	f := newScanFixture(t)
	music, sub, top, one := nestedSources(t)
	f.add(t, music, sub)

	stats := f.scanRemovingBeforeTheWalk(t, music, music, sub)

	if f.trackID(t, top) != 0 {
		t.Error("top.mp3, which no source holds any more, is in the library")
	}
	if f.trackID(t, one) == 0 {
		t.Error("one.mp3, which music/sub holds, is not in the library")
	}
	if got := stats.BySource[music]; got == nil || len(got.Added) != 0 {
		t.Errorf("stats of music = %+v, want an entry with nothing added", got)
	}
}

func TestRefresh_OfADirectoryThatIsNotASourceWritesNoTrack(t *testing.T) {
	f := newScanFixture(t)
	dir := t.TempDir()
	track := writeTrack(t, filepath.Join(dir, "a.mp3"), "A")

	progress := make(chan library.ScanProgress)
	done := make(chan error, 1)
	go func() { done <- f.lib.Refresh([]string{dir}, progress, nil) }()
	for range progress {
	}
	if err := <-done; err != nil {
		t.Fatalf("scan: %v", err)
	}

	if f.trackID(t, track) != 0 {
		t.Error("the track of a directory that is not a source is in the library")
	}
}

// A source that is the path of a file, not a directory, holds nothing: it is
// not under itself, and its removal would not delete its track.
func TestRefresh_WritesNoTrackAtThePathOfASourceThatIsTheFile(t *testing.T) {
	s := newTwoSources(t)
	s.add(t, s.a)

	s.scanRemovingBeforeTheWalk(t, s.music, s.music)

	if s.trackID(t, s.a) != 0 {
		t.Error("a.mp3, which only the source at its own path names, is in the library")
	}
}

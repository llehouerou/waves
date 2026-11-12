package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/state"
)

// addLibraryTrack puts a track in the library table without going through a
// scan: the file at path does not have to exist.
func addLibraryTrack(t *testing.T, mgr *state.Manager, path, artist, title string) {
	t.Helper()
	_, err := mgr.DB().Exec(`
		INSERT INTO library_tracks
			(path, mtime, artist, album_artist, album, title, disc_number, track_number, year, added_at, updated_at)
		VALUES (?, 1000, ?, ?, 'Album', ?, 1, 1, 2024, 1000, 1000)
	`, path, artist, artist, title)
	if err != nil {
		t.Fatalf("insert track: %v", err)
	}
}

// waitScanEnd waits for the scan to end without reading a single event: no
// client makes it advance.
func waitScanEnd(t *testing.T, c *Core) {
	t.Helper()
	eventually(t, "the scan ends", func() bool { return !c.Library().Scanning() })
}

func TestScan_RunsToItsEndWithNoClientReadingItsProgress(t *testing.T) {
	mgr := openTestState(t)
	source := t.TempDir()
	// Two tracks the library knows, whose files are gone.
	addLibraryTrack(t, mgr, filepath.Join(source, "one.mp3"), "Low", "Lullaby")
	addLibraryTrack(t, mgr, filepath.Join(source, "two.mp3"), "Low", "Words")

	c, _ := newTestCore(t, Options{State: mgr, LibrarySources: []string{source}})
	lib := c.Library()

	if err := lib.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	waitScanEnd(t, c)

	if e := nextEvent[ScanProgressed](t, c); e.Phase != "scanning" {
		t.Errorf("first ScanProgressed = %+v, want the scanning phase", e)
	}
	finished := nextEvent[ScanFinished](t, c)
	if finished.Stats == nil {
		t.Fatal("ScanFinished carries no stats")
	}
	removed := finished.Stats.BySource[source].Removed
	slices.Sort(removed)
	if !slices.Equal(removed, []string{"one.mp3", "two.mp3"}) {
		t.Errorf("removed = %v, want one.mp3 and two.mp3", removed)
	}

	if artists, err := lib.Artists(); err != nil || len(artists) != 0 {
		t.Errorf("artists after the scan = %v (err %v), want none", artists, err)
	}
	// The search index followed, with no call from the client.
	if results, err := lib.SearchFTS("Lullaby"); err != nil || len(results) != 0 {
		t.Errorf("search after the scan = %v (err %v), want nothing", results, err)
	}
}

func TestScan_AddsTheFilesItFindsToTheLibraryAndTheSearchIndex(t *testing.T) {
	source := t.TempDir()
	file := filepath.Join(source, "canada.flac")
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-metadata", "artist=Low", "-metadata", "album=Trust", "-metadata", "title=Canada", file)
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	c, _ := newTestCore(t, Options{LibrarySources: []string{source}})
	lib := c.Library()

	if err := lib.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	waitScanEnd(t, c)

	finished := nextEvent[ScanFinished](t, c)
	if finished.Stats == nil || !slices.Equal(finished.Stats.BySource[source].Added, []string{"canada.flac"}) {
		t.Errorf("ScanFinished = %+v, want canada.flac added", finished.Stats)
	}
	tracks, err := lib.Tracks("Low", "Trust")
	if err != nil || len(tracks) != 1 || tracks[0].Title != "Canada" {
		t.Errorf("tracks = %+v (err %v), want Canada", tracks, err)
	}
	if results, err := lib.SearchFTS("Canada"); err != nil || len(results) == 0 {
		t.Errorf("search = %v (err %v), want Canada", results, err)
	}
}

func TestScan_ALargeLibraryDoesNotDrownTheEventStream(t *testing.T) {
	source := t.TempDir()
	// Enough files for more progress reports than the event stream holds.
	for i := range 100 * (eventBuffer + 6) {
		name := filepath.Join(source, strconv.Itoa(i)+".mp3")
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	c, _ := newTestCore(t, Options{LibrarySources: []string{source}})
	if err := c.Library().Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	waitScanEnd(t, c)

	// The end of the scan is still there for a client that reads late.
	nextEvent[ScanFinished](t, c)
}

func TestScan_AFailureIsReportedAndTheScanStillFinishes(t *testing.T) {
	mgr := openTestState(t)
	c, _ := newTestCore(t, Options{State: mgr, LibrarySources: []string{t.TempDir()}})
	// The scan reads this table first.
	if _, err := mgr.DB().Exec(`DROP TABLE library_tracks`); err != nil {
		t.Fatalf("drop table: %v", err)
	}

	if err := c.Library().Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if e := waitFailed(t, c, errmsg.OpLibraryScan); e.Err == nil {
		t.Error("Failed carries no error")
	}
	if e := nextEvent[ScanFinished](t, c); e.Stats != nil {
		t.Errorf("ScanFinished = %+v, want no stats after a failure", e)
	}
	if c.Library().Scanning() {
		t.Error("still scanning after the scan finished")
	}
}

func TestAddSource_ScansTheNewSource(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	lib := c.Library()
	source := t.TempDir()

	if err := lib.AddSource(source); err != nil {
		t.Fatalf("add source: %v", err)
	}

	finished := nextEvent[ScanFinished](t, c)
	if finished.Stats == nil || finished.Stats.BySource[source] == nil {
		t.Errorf("ScanFinished = %+v, want the stats of the new source", finished.Stats)
	}
	if sources, err := lib.Sources(); err != nil || !slices.Equal(sources, []string{source}) {
		t.Errorf("sources = %v (err %v), want the new source", sources, err)
	}
}

func TestNew_BuildsTheSearchIndexOfALibraryThatHasNone(t *testing.T) {
	mgr := openTestState(t)
	addLibraryTrack(t, mgr, "/lib/low/lullaby.mp3", "Low", "Lullaby")

	c, _ := newTestCore(t, Options{State: mgr})

	eventually(t, "the track is found by a search", func() bool {
		results, err := c.Library().SearchFTS("Lullaby")
		return err == nil && len(results) > 0
	})
}

func TestNew_TakesTheConfigSourcesOnlyWhenTheLibraryHasNone(t *testing.T) {
	mgr := openTestState(t)

	first, _ := newTestCore(t, Options{State: mgr, LibrarySources: []string{"/music"}})
	if sources, err := first.Library().Sources(); err != nil || !slices.Equal(sources, []string{"/music"}) {
		t.Fatalf("sources = %v (err %v), want /music", sources, err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, _ := newTestCore(t, Options{State: mgr, LibrarySources: []string{"/other"}})
	if sources, err := second.Library().Sources(); err != nil || !slices.Equal(sources, []string{"/music"}) {
		t.Errorf("sources = %v (err %v), want /music kept", sources, err)
	}
}

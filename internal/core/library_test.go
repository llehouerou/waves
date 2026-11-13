package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags/tagstest"
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

	c, _ := newTestCore(t, Options{state: mgr, LibrarySources: []string{source}})
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
	c, _ := newTestCore(t, Options{state: mgr, LibrarySources: []string{t.TempDir()}})
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

	c, _ := newTestCore(t, Options{state: mgr})

	eventually(t, "the track is found by a search", func() bool {
		results, err := c.Library().SearchFTS("Lullaby")
		return err == nil && len(results) > 0
	})
}

// The index is built in the database: Close waits for it, as for anything
// that writes there, rather than leave it to end on a database being closed.
func TestClose_WaitsForTheSearchIndexBeingBuilt(t *testing.T) {
	mgr := openTestState(t)
	for i := range 500 {
		n := strconv.Itoa(i)
		addLibraryTrack(t, mgr, "/lib/slowdive/"+n+".mp3", "Slowdive", "Track "+n)
	}
	c, _ := newTestCore(t, Options{state: mgr})

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	var indexed int
	if err := mgr.DB().QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&indexed); err != nil {
		t.Fatalf("read the index: %v", err)
	}
	if indexed == 0 {
		t.Error("Close returned with the search index still being built")
	}
}

// trustTracks are the titles of the tracks of Low - Trust in the library.
func trustTracks(t *testing.T, c *Core) []string {
	t.Helper()
	tracks, err := c.Library().Tracks("Low", "Trust")
	if err != nil {
		t.Fatalf("tracks: %v", err)
	}
	titles := make([]string, 0, len(tracks))
	for i := range tracks {
		titles = append(titles, tracks[i].Title)
	}
	slices.Sort(titles)
	return titles
}

func TestClose_EndsAScanAfterTheFilesItIsReading(t *testing.T) {
	source := t.TempDir()
	mp3 := func(name, title string) string {
		path := filepath.Join(source, name)
		tagstest.WriteMP3(t, path, FileTags{Artist: "Low", AlbumArtist: "Low", Album: "Trust", Title: title})
		return path
	}
	// On a database the core opens and closes itself, as a client's does.
	database := filepath.Join(t.TempDir(), "waves.db")
	opts := Options{Database: database, LibrarySources: []string{source}}

	// A first scan to its end: on an empty search index, the next run would
	// rebuild it and hide what a scan cut short leaves there.
	gone := mp3("a.mp3", "Gone")
	c, _ := newTestCore(t, opts)
	if err := c.Library().Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	waitScanEnd(t, c)
	nextEvent[ScanFinished](t, c)

	// Read in this order: b, then as many files that hold their reader as
	// the scan has, then z, which no reader reaches before Close.
	mp3("b.mp3", "Written")
	slow := make([]string, 0, 8)
	for i := range 8 {
		path := filepath.Join(source, "f"+strconv.Itoa(i)+".mp3")
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}
		slow = append(slow, path)
	}
	// Each opening and closing of the other end ends a read.
	release := func() {
		for _, path := range slow {
			if w, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				w.Close()
			}
		}
		time.Sleep(time.Millisecond)
	}
	// Registered after the core's: a failed test must not leave Close
	// waiting for reads nobody ends.
	t.Cleanup(func() {
		for i := 0; i < 5000 && c.Library().Scanning(); i++ {
			release()
		}
	})
	mp3("z.mp3", "Unread")
	if err := c.Library().Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	eventually(t, "the scan writes b", func() bool { return slices.Contains(trustTracks(t, c), "Written") })

	closed := make(chan error, 1)
	go func() { closed <- c.Close() }()
	eventually(t, "Close has begun", c.stopping)
	deadline := time.After(5 * time.Second)
	for done := false; !done; {
		select {
		case err := <-closed:
			if err != nil {
				t.Fatalf("close: %v", err)
			}
			done = true
		case <-deadline:
			t.Fatal("Close still waits for the scan")
		default:
			release()
		}
	}

	if c.Library().Scanning() {
		t.Error("still scanning after Close")
	}
	for e := range c.Events() {
		switch e.(type) {
		case Failed, ScanFinished:
			t.Errorf("Close reported %+v", e)
		}
	}

	// The next run: what the scan wrote is searched, with no scan in between.
	next, _ := newTestCore(t, opts)
	if got := trustTracks(t, next); !slices.Equal(got, []string{"Gone", "Written"}) {
		t.Errorf("tracks after Close = %v, want Gone and Written: z not read, nothing removed", got)
	}
	if results, err := next.Library().SearchFTS("Written"); err != nil || len(results) == 0 {
		t.Errorf("search for the track written before Close = %v (err %v), want it", results, err)
	}

	// A scan of the next run does what the one cut short left.
	for _, path := range append(slow, gone) {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove: %v", err)
		}
	}
	if err := next.Library().Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	waitScanEnd(t, next)
	if got := trustTracks(t, next); !slices.Equal(got, []string{"Unread", "Written"}) {
		t.Errorf("tracks after the next scan = %v, want Unread and Written", got)
	}
}

func TestNew_TakesTheConfigSourcesOnlyWhenTheLibraryHasNone(t *testing.T) {
	mgr := openTestState(t)

	first, _ := newTestCore(t, Options{state: mgr, LibrarySources: []string{"/music"}})
	if sources, err := first.Library().Sources(); err != nil || !slices.Equal(sources, []string{"/music"}) {
		t.Fatalf("sources = %v (err %v), want /music", sources, err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, _ := newTestCore(t, Options{state: mgr, LibrarySources: []string{"/other"}})
	if sources, err := second.Library().Sources(); err != nil || !slices.Equal(sources, []string{"/music"}) {
		t.Errorf("sources = %v (err %v), want /music kept", sources, err)
	}
}

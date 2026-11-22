package core

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// newRetagCore builds a core whose library holds one album, "Trust" by "low"
// as a careless tagger left it: three files in one library source, numbered
// and titled in lower case. It returns their paths in track order.
func newRetagCore(t *testing.T) (c *Core, mock *player.Mock, paths []string) {
	t.Helper()
	source := t.TempDir()
	return newRetagCoreIn(t, source, source)
}

// newRetagCoreIn is newRetagCore with the album in the folder dir and the
// library sources given.
func newRetagCoreIn(t *testing.T, dir string, sources ...string) (c *Core, mock *player.Mock, paths []string) {
	t.Helper()
	return newRetagCoreOn(t, dir, Options{LibrarySources: sources})
}

// newRetagCoreOn is newRetagCore with the album in the folder dir and the
// core built on opts.
func newRetagCoreOn(t *testing.T, dir string, opts Options) (c *Core, mock *player.Mock, paths []string) {
	t.Helper()
	for i, title := range []string{"amazing grace", "canada", "candy girl"} {
		path := filepath.Join(dir, "low", "trust", title+".mp3")
		tagstest.WriteMP3(t, path, FileTags{Artist: "low", AlbumArtist: "low", Album: "trust", Title: title, TrackNumber: i + 1})
		paths = append(paths, path)
	}
	c, mock = newTestCore(t, opts)
	if _, err := c.library.lib.AddTracks(paths); err != nil {
		t.Fatalf("add tracks: %v", err)
	}
	return c, mock, paths
}

// asLowTrust retags the album of newRetagCore as lowTrust.
func asLowTrust() RetagRequest {
	return RetagRequest{
		AlbumArtist:  "low",
		Album:        "trust",
		Release:      lowTrust(),
		ReleaseGroup: &MBReleaseGroup{ID: "rg", PrimaryType: "Album", FirstRelease: "2002"},
	}
}

func TestRetag_RunsToItsEndWithNoClientDrivingIt(t *testing.T) {
	c, _, paths := newRetagCore(t)
	lib := c.Library()

	if err := lib.Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}

	// Not an event is read: the core retags on its own, and the library
	// follows the files.
	eventually(t, "the library holds the album under its new name", func() bool {
		tracks, err := lib.Tracks("Low", "Trust")
		return err == nil && len(tracks) == 3
	})
	tracks, err := lib.Tracks("Low", "Trust")
	if err != nil {
		t.Fatalf("tracks: %v", err)
	}
	for i, title := range []string{"Amazing Grace", "Canada", "Candy Girl"} {
		if tracks[i].Title != title || tracks[i].Path != paths[i] {
			t.Errorf("track %d = %q at %s, want %q at %s: retagged in place", i+1, tracks[i].Title, tracks[i].Path, title, paths[i])
		}
	}
	if old, err := lib.Tracks("low", "trust"); err != nil || len(old) != 0 {
		t.Errorf("the album is still in the library under its old name: %+v (err %v)", old, err)
	}
}

// playFile starts playing the file at path.
func playFile(t *testing.T, c *Core, path string) {
	t.Helper()
	c.Playback().AddTracks(Track{Path: path})
	c.Playback().QueueMoveTo(0)
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
}

// The player holds the file it plays open: a retag that is about to write it
// stops playback first. It was the client's to do.
func TestRetag_StopsPlaybackOfATrackItIsAboutToWrite(t *testing.T) {
	c, mock, paths := newRetagCore(t)
	playFile(t, c, paths[1])

	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}

	if mock.State() != player.Stopped {
		t.Errorf("player state = %v once the retag is started, want stopped", mock.State())
	}
}

func TestRetag_LeavesATrackOfAnotherAlbumPlaying(t *testing.T) {
	c, mock, _ := newRetagCore(t)
	playFile(t, c, "/music/another/album/track.mp3")

	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}

	if mock.State() != player.Playing {
		t.Errorf("player state = %v, want still playing", mock.State())
	}
}

func TestRetag_ReportsWhereItIsThenHowItEnded(t *testing.T) {
	c, _, paths := newRetagCore(t)

	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}

	progress := nextEvent[RetagProgressed](t, c)
	if want := []FileState{FileWorking, FilePending, FilePending}; progress.AlbumArtist != "low" || progress.Album != "trust" || !slices.Equal(fileStates(progress.Files), want) {
		t.Errorf("first RetagProgressed = %+v, want the album asked for with its first file being retagged", progress)
	}

	finished := nextEvent[RetagFinished](t, c)
	if finished.AlbumArtist != "low" || finished.Album != "trust" || finished.Err != nil {
		t.Errorf("RetagFinished = %+v, want the album asked for and no failure", finished)
	}
	if !AllDone(finished.Files) || len(finished.Files) != 3 {
		t.Fatalf("files ended %v, want the three done", fileStates(finished.Files))
	}
	for i, f := range finished.Files {
		if f.Path != paths[i] {
			t.Errorf("file %d is %s, want %s", i, f.Path, paths[i])
		}
	}
	// The library is up to date when a client is told.
	if tracks, err := c.Library().Tracks("Low", "Trust"); err != nil || len(tracks) != 3 {
		t.Errorf("library tracks = %+v (err %v), want the three retagged", tracks, err)
	}
}

func TestRetag_OneRunsAtATime(t *testing.T) {
	c, _, _ := newRetagCore(t)

	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	// The first one has three files to tag: it is still running.
	if err := c.Library().Retag(asLowTrust()); !errors.Is(err, ErrRetagRunning) {
		t.Errorf("a second Retag = %v, want ErrRetagRunning", err)
	}

	if finished := nextEvent[RetagFinished](t, c); !AllDone(finished.Files) {
		t.Errorf("the first retag ended %v, want it untouched by the second", fileStates(finished.Files))
	}
	// A client told the retag is over can start another: of the album under
	// its new name.
	again := asLowTrust()
	again.AlbumArtist, again.Album = "Low", "Trust"
	if err := c.Library().Retag(again); err != nil {
		t.Errorf("a retag after RetagFinished = %v, want it started", err)
	}
}

func TestRetag_RefusesWhatItCannotStart(t *testing.T) {
	c, mock, paths := newRetagCore(t)
	playFile(t, c, paths[0])

	unknown := asLowTrust()
	unknown.Album = "secret name"
	if err := c.Library().Retag(unknown); err == nil {
		t.Error("the retag of an album the library doesn't hold was accepted")
	}
	noRelease := asLowTrust()
	noRelease.Release = nil
	if err := c.Library().Retag(noRelease); err == nil {
		t.Error("a retag with no release was accepted")
	}

	expectNoEvent[RetagProgressed](t, c)
	if mock.State() != player.Playing {
		t.Errorf("player state = %v, want a retag that did not start to leave it playing", mock.State())
	}
	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Errorf("a retag after refused ones = %v, want it started", err)
	}
}

func TestAlbumTags_ReadsTheTagsOfTheAlbumsFiles(t *testing.T) {
	c, _, paths := newRetagCore(t)

	fileTags, err := c.Library().AlbumTags("low", "trust")
	if err != nil {
		t.Fatalf("album tags: %v", err)
	}

	if len(fileTags) != 3 {
		t.Fatalf("%d files, want the three of the album", len(fileTags))
	}
	for i, title := range []string{"amazing grace", "canada", "candy girl"} {
		if fileTags[i].Path != paths[i] || fileTags[i].Title != title || fileTags[i].TrackNumber != i+1 {
			t.Errorf("file %d = %q, track %d, at %s; want %q, track %d, at %s",
				i, fileTags[i].Title, fileTags[i].TrackNumber, fileTags[i].Path, title, i+1, paths[i])
		}
	}

	if _, err := c.Library().AlbumTags("low", "secret name"); err == nil {
		t.Error("the tags of an album the library doesn't hold were read")
	}
}

// A source removed while a retag of one of its albums runs: the retag writes
// every file, and the library, which no longer has the source, reads none
// back. Whether the removal comes before the retag's write or after it.
func TestRetag_OfAnAlbumWhoseSourceIsRemovedLeavesNoTrackOfIt(t *testing.T) {
	source := t.TempDir()
	c, _, paths := newRetagCoreIn(t, source, source)
	lib := c.Library()

	if err := lib.Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	if err := lib.RemoveSource(source); err != nil {
		t.Fatalf("remove source: %v", err)
	}

	finished := nextEvent[RetagFinished](t, c)
	if finished.Err != nil || !AllDone(finished.Files) || len(finished.Files) != 3 {
		t.Errorf("RetagFinished = %+v, want the three files done and no failure", finished)
	}
	if sources, err := lib.Sources(); err != nil || len(sources) != 0 {
		t.Errorf("sources = %v (err %v), want none", sources, err)
	}
	if artists, err := lib.Artists(); err != nil || len(artists) != 0 {
		t.Errorf("artists = %v (err %v), want none", artists, err)
	}
	for _, title := range []string{"Amazing Grace", "Canada", "Candy Girl", "amazing grace"} {
		if results, err := lib.SearchFTS(title); err != nil || len(results) != 0 {
			t.Errorf("search %q = %+v (err %v), want nothing", title, results, err)
		}
	}
	for i, title := range []string{"Amazing Grace", "Canada", "Candy Girl"} {
		if got, err := tags.Read(paths[i]); err != nil || got.Title != title || got.Artist != "Low" {
			t.Errorf("file %d tagged %+v (err %v), want %q by Low", i+1, got, err, title)
		}
	}
}

// music/sub removed while music is a source too: music still holds the
// album, which the library reads back under its new names.
func TestRetag_OfAnAlbumAnOuterSourceStillHoldsReadsItBack(t *testing.T) {
	music := t.TempDir()
	sub := filepath.Join(music, "sub")
	c, _, paths := newRetagCoreIn(t, sub, music, sub)
	lib := c.Library()

	if err := lib.Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	if err := lib.RemoveSource(sub); err != nil {
		t.Fatalf("remove source: %v", err)
	}

	if finished := nextEvent[RetagFinished](t, c); finished.Err != nil || !AllDone(finished.Files) {
		t.Errorf("RetagFinished = %+v, want every file done and no failure", finished)
	}
	tracks, err := lib.Tracks("Low", "Trust")
	if err != nil || len(tracks) != 3 {
		t.Fatalf("library tracks = %+v (err %v), want the three retagged", tracks, err)
	}
	for i, title := range []string{"Amazing Grace", "Canada", "Candy Girl"} {
		if tracks[i].Title != title || tracks[i].Path != paths[i] {
			t.Errorf("track %d = %q at %s, want %q at %s", i+1, tracks[i].Title, tracks[i].Path, title, paths[i])
		}
	}
}

// spoilRow makes the library row of the file at path one TrackByID cannot
// read: AlbumTrackIDs still lists it. It returns what puts the row right.
func spoilRow(t *testing.T, c *Core, path string) (repair func()) {
	t.Helper()
	if _, err := c.state.DB().Exec(`UPDATE library_tracks SET year = 'not a year' WHERE path = ?`, path); err != nil {
		t.Fatalf("spoil row: %v", err)
	}
	return func() {
		t.Helper()
		if _, err := c.state.DB().Exec(`UPDATE library_tracks SET year = NULL WHERE path = ?`, path); err != nil {
			t.Fatalf("repair row: %v", err)
		}
	}
}

func TestAlbumTags_FailsWhenATrackOfTheAlbumCannotBeRead(t *testing.T) {
	c, _, paths := newRetagCore(t)
	spoilRow(t, c, paths[1])

	if fileTags, err := c.Library().AlbumTags("low", "trust"); err == nil {
		t.Errorf("album tags = %d files and no error, want the failed read", len(fileTags))
	}
}

func TestRetag_OfAnAlbumWithATrackThatCannotBeReadStartsNothing(t *testing.T) {
	c, mock, paths := newRetagCore(t)
	playFile(t, c, paths[0])
	repair := spoilRow(t, c, paths[1])

	if err := c.Library().Retag(asLowTrust()); err == nil {
		t.Fatal("the retag of an album with an unreadable track was accepted")
	}

	for _, e := range eventsDuring(c) {
		switch e.(type) {
		case RetagProgressed, RetagFinished:
			t.Errorf("unexpected %T: %+v", e, e)
		}
	}
	if mock.State() != player.Playing {
		t.Errorf("player state = %v, want a retag that did not start to leave it playing", mock.State())
	}
	for i, title := range []string{"amazing grace", "canada", "candy girl"} {
		if got, err := tags.Read(paths[i]); err != nil || got.Title != title || got.Artist != "low" {
			t.Errorf("file %d tagged %+v (err %v), want %q by low, untouched", i+1, got, err, title)
		}
	}
	repair()
	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Errorf("a retag once the track reads again = %v, want it started", err)
	}
}

// The album is renamed: the event names it as it was asked for, which the
// popup knows its retag by, and as the library now has it.
func TestRetag_FinishedNamesTheAlbumAsTheLibraryNowHasIt(t *testing.T) {
	c, _, _ := newRetagCore(t)

	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}

	finished := nextEvent[RetagFinished](t, c)
	if finished.AlbumArtist != "low" || finished.Album != "trust" {
		t.Errorf("album asked for = %q / %q, want low / trust", finished.AlbumArtist, finished.Album)
	}
	if finished.LibraryAlbumArtist != "Low" || finished.LibraryAlbum != "Trust" {
		t.Errorf("album in the library = %q / %q, want Low / Trust", finished.LibraryAlbumArtist, finished.LibraryAlbum)
	}
}

// A release with no track for any file: the retag writes none, and names no
// album the library has.
func TestRetag_ThatWritesNoFileNamesNoAlbumInTheLibrary(t *testing.T) {
	c, _, _ := newRetagCore(t)
	req := asLowTrust()
	req.Release.Tracks = nil

	if err := c.Library().Retag(req); err != nil {
		t.Fatalf("retag: %v", err)
	}

	finished := nextEvent[RetagFinished](t, c)
	if done := fileStates(finished.Files); slices.Contains(done, FileDone) {
		t.Fatalf("files ended %v, want none written", done)
	}
	if finished.LibraryAlbumArtist != "" || finished.LibraryAlbum != "" {
		t.Errorf("album in the library = %q / %q, want none", finished.LibraryAlbumArtist, finished.LibraryAlbum)
	}
}

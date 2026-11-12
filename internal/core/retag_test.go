package core

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// newRetagCore builds a core whose library holds one album, "Trust" by "low"
// as a careless tagger left it: three files in one library source, numbered
// and titled in lower case. It returns their paths in track order.
func newRetagCore(t *testing.T) (c *Core, mock *player.Mock, paths []string) {
	t.Helper()
	source := t.TempDir()
	for i, title := range []string{"amazing grace", "canada", "candy girl"} {
		path := filepath.Join(source, "low", "trust", title+".mp3")
		tagstest.WriteMP3(t, path, FileTags{Artist: "low", AlbumArtist: "low", Album: "trust", Title: title, TrackNumber: i + 1})
		paths = append(paths, path)
	}
	c, mock = newTestCore(t, Options{LibrarySources: []string{source}})
	if err := c.library.lib.AddTracks(paths); err != nil {
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

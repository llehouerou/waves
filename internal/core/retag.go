package core

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/llehouerou/waves/internal/importer"
)

// RetagRequest is what a retag writes on which album.
type RetagRequest struct {
	AlbumArtist, Album string            // the library album to retag
	Release            *MBReleaseDetails // what it is tagged as
	ReleaseGroup       *MBReleaseGroup   // the release group of Release
	Cover              []byte            // embedded in each file; nil for none
}

// AlbumTags reads the tags of the files of a library album, in track order: a
// client compares them with the release it is about to retag the album as.
// A file whose tags can't be read has none but its Path.
//
// It waits for the disk: a client calls it off its UI goroutine.
func (l *Library) AlbumTags(albumArtist, album string) ([]FileTags, error) {
	paths, err := l.albumPaths(albumArtist, album)
	if err != nil {
		return nil, err
	}
	return importer.ReadTags(paths), nil
}

// RetagTrack is the index in release.Tracks of the track a retag tags the i-th
// file of a library album as, in the order Library.AlbumTags gives them: the
// one its track and disc numbers name, else the i-th. -1 when the release has
// no track for it, and the retag leaves it as it is.
func RetagTrack(file *FileTags, i int, release *MBReleaseDetails) int {
	return importer.RetagTrack(file, i, release)
}

// ErrRetagRunning is returned by Retag while another retag runs.
var ErrRetagRunning = errors.New("another retag is running")

// Retag starts writing the tags of a release on the files of a library album
// and returns at once. The core tags each file in place as the track its
// numbers name, then reads again into the library the files a library source
// still holds: a RemoveSource while the retag runs does not cut it short, and
// leaves no track of the source once it ends. RetagProgressed follows it and
// RetagFinished ends it.
//
// One retag runs at a time: a second one is refused with ErrRetagRunning.
func (l *Library) Retag(req RetagRequest) error {
	if req.Release == nil || req.ReleaseGroup == nil {
		return errors.New("retag: no release to tag the album as")
	}
	paths, err := l.albumPaths(req.AlbumArtist, req.Album)
	if err != nil {
		return err
	}

	if !l.retagging.CompareAndSwap(false, true) {
		return ErrRetagRunning
	}

	// The player holds the file it plays open: it lets go before it is written.
	if current := l.core.playback.CurrentTrack(); current != nil && slices.Contains(paths, current.Path) {
		_ = l.core.playback.Stop() //nolint:errcheck // Stop never fails; the signature is the service's
	}

	l.core.work.Go(func() {
		files := importer.Retag(paths, req.Release, req.ReleaseGroup, req.Cover, throttled(func(files []FileProgress) {
			l.core.emit(RetagProgressed{AlbumArtist: req.AlbumArtist, Album: req.Album, Files: files})
		}), l.core.stopping)
		// Every file, even one that failed: the library says what the files
		// say. Those of a source removed meanwhile are left out, and that is
		// not a failure.
		_, err := l.lib.AddTracks(paths)
		finished := RetagFinished{AlbumArtist: req.AlbumArtist, Album: req.Album, Files: files, Err: err}
		if err == nil {
			finished.LibraryAlbumArtist, finished.LibraryAlbum = l.retaggedAlbum(files)
		}

		l.retagging.Store(false)
		if l.core.stopping() {
			return // cut short by Close: not a retag that failed
		}
		l.core.emit(finished)
	})
	return nil
}

// retaggedAlbum names the album the library has the files a retag wrote
// under: one release gives them all the same names, so the first the library
// holds names it. Empty when it holds none of them, or when looking one up
// fails.
func (l *Library) retaggedAlbum(files []FileProgress) (albumArtist, album string) {
	for i := range files {
		if files[i].State != FileDone {
			continue
		}
		t, err := l.lib.TrackByPath(files[i].Path)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue // not in the library: its source was removed
		case err != nil:
			// The files were retagged: without a name, a client leaves its
			// view as the refresh left it.
			return "", ""
		}
		return t.AlbumArtist, t.Album
	}
	return "", ""
}

// albumPaths are the files of a library album, in track order. A track that
// cannot be read fails it: a retag is of the whole album or nothing.
func (l *Library) albumPaths(albumArtist, album string) ([]string, error) {
	ids, err := l.lib.AlbumTrackIDs(albumArtist, album)
	if err != nil {
		return nil, err
	}
	tracks, err := l.tracksByID(ids)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no track of %s - %s in the library", albumArtist, album)
	}
	paths := make([]string, len(tracks))
	for i, t := range tracks {
		paths[i] = t.Path
	}
	return paths, nil
}

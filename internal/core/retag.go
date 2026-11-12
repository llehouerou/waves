package core

import (
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

// ErrRetagRunning is returned by Retag while another retag runs.
var ErrRetagRunning = errors.New("another retag is running")

// Retag starts writing the tags of a release on the files of a library album
// and returns at once. The core tags each file in place as the track its
// numbers name, then reads the files again into the library. RetagProgressed
// follows it and RetagFinished ends it.
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
		// Every file, even one that failed: the library says what the files say.
		err := l.lib.AddTracks(paths)

		l.retagging.Store(false)
		if l.core.stopping() {
			return // cut short by Close: not a retag that failed
		}
		l.core.emit(RetagFinished{AlbumArtist: req.AlbumArtist, Album: req.Album, Files: files, Err: err})
	})
	return nil
}

// albumPaths are the files of a library album, in track order.
func (l *Library) albumPaths(albumArtist, album string) ([]string, error) {
	ids, err := l.lib.AlbumTrackIDs(albumArtist, album)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		if track, err := l.lib.TrackByID(id); err == nil {
			paths = append(paths, track.Path)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no track of %s - %s in the library", albumArtist, album)
	}
	return paths, nil
}

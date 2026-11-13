package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/importer"
	"github.com/llehouerou/waves/internal/tags"
)

// What an import and a retag hand a client, so that it imports the core only.
type (
	// ImportPlan is what importing a download will do: PrepareImport makes
	// it, a client shows it, Import carries it out.
	ImportPlan = importer.Plan
	ImportFile = importer.PlannedFile

	// FileTags are the tags of an audio file.
	FileTags = tags.Tag

	// FileProgress is one file of an import or of a retag, and where it is.
	FileProgress = importer.FileProgress
	FileState    = importer.FileState
)

// Where a file of an import or of a retag is.
const (
	FilePending         = importer.FilePending         // not reached yet
	FileWorking         = importer.FileWorking         // being tagged, and moved for an import
	FileDone            = importer.FileDone            // tagged, and in the library for an import
	FileAlreadyImported = importer.FileAlreadyImported // moved by an earlier import: a success
	FileFailed          = importer.FileFailed
)

// ErrImportRunning is returned by Import while another import runs.
var ErrImportRunning = errors.New("another import is running")

// BuildGenreString is the genre an import or a retag writes for a release:
// its own genres, or its release group's when it has none.
func BuildGenreString(releaseGenres, releaseGroupGenres []string) string {
	return importer.BuildGenreString(releaseGenres, releaseGroupGenres)
}

// ParseTrackNumber reads the track number a file name starts with, 0 when it
// has none.
func ParseTrackNumber(filename string) int {
	return downloads.ParseTrackNumber(filename)
}

// AllDone reports whether every file of an import or of a retag made it:
// none failed, none was left out.
func AllDone(files []FileProgress) bool {
	for i := range files {
		if files[i].State != FileDone && files[i].State != FileAlreadyImported {
			return false
		}
	}
	return len(files) > 0
}

// PrepareImport plans the import of a download: it reads the tags of its
// files, settles the release they are imported as (their own when their tags
// name another than the one queued), fetches its cover and matches each file
// to a track. Nothing is written.
//
// It waits for the disk and for MusicBrainz: a client calls it off its UI
// goroutine.
func (d *Downloads) PrepareImport(id int64) (ImportPlan, error) {
	dl, err := d.mgr.Get(id)
	if err != nil {
		return ImportPlan{}, err
	}
	return importer.Prepare(dl, d.completedPath, d.core.musicbrainz, d.rename)
}

// Import starts importing a plan into the library source and returns at once.
// The core tags and moves each file, writes the cover, adds the tracks to the
// library and, when every file made it, finishes the import of the download.
// ImportProgressed follows it and ImportFinished ends it.
//
// One import runs at a time: a second one is refused with ErrImportRunning.
func (d *Downloads) Import(plan ImportPlan, source string) error {
	if plan.Release == nil {
		return importer.ErrNoRelease
	}
	lib := d.core.library.lib
	isSource, err := lib.SourceExists(source)
	if err != nil {
		return err
	}
	if !isSource {
		return fmt.Errorf("%s is not a library source", source)
	}
	if !d.importing.CompareAndSwap(false, true) {
		return ErrImportRunning
	}

	d.core.work.Go(func() {
		files := importer.Run(plan, source, d.rename, throttled(func(files []FileProgress) {
			d.core.emit(ImportProgressed{DownloadID: plan.DownloadID, Files: files})
		}), d.core.stopping)

		// Whatever was moved is in the library folder: the library must know,
		// even when the import failed on other files or Close cut it short.
		var moved []string
		for i := range files {
			if files[i].Dest != "" {
				moved = append(moved, files[i].Dest)
			}
		}
		err := lib.AddTracks(moved)
		if err == nil && AllDone(files) {
			if err := d.mgr.FinishImport(plan.DownloadID); err != nil && !d.core.stopping() {
				// The tracks are in the library: the import itself succeeded.
				d.core.emit(Failed{Op: errmsg.OpDownloadCleanup, Err: err})
			}
		}

		d.importing.Store(false)
		if d.core.stopping() {
			return // cut short by Close: not an import that failed
		}
		if err == nil && AllDone(files) {
			d.core.notifications.imported(plan.Release.Artist, plan.Release.Title)
		}
		d.core.emit(ImportFinished{
			DownloadID: plan.DownloadID,
			Artist:     plan.Release.Artist,
			Album:      plan.Release.Title,
			Files:      files,
			Err:        err,
		})
	})
	return nil
}

// throttled returns report limited to one call per progressEvery; the first
// goes through. What it is given must be a whole state rather than a step, so
// that a call left out loses nothing.
func throttled[T any](report func(T)) func(T) {
	var last time.Time
	return func(state T) {
		if time.Since(last) < progressEvery {
			return
		}
		last = time.Now()
		report(state)
	}
}

package library

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/llehouerou/waves/internal/tags"
)

// processFiles processes files in parallel and updates the database and stats.
// Once stop says so, it reads no new file.
//
// It returns the error of the first write that fails: it then writes nothing
// more, reads no new file and returns without waiting for the files being read.
//
// Its progress reporter has ended when it returns, whichever way: nothing it
// started sends on progress afterwards.
func (l *Library) processFiles(
	filesToProcess []fileInfo,
	fileIsNew map[string]bool,
	stats *ScanStats,
	progress chan<- ScanProgress,
	stop func() bool,
) error {
	total := len(filesToProcess)
	var processed atomic.Int64
	var failed atomic.Bool

	// Create work channel and results channel
	workCh := make(chan fileInfo, total)
	resultCh := make(chan trackResult, total)

	// Start workers
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for f := range workCh {
				if failed.Load() || stop() {
					return
				}
				// Extract metadata (without duration for speed)
				info, err := tags.Read(f.path)
				if err != nil {
					processed.Add(1)
					continue
				}

				// Skip files without artist or album
				if info.Artist == "" || info.Album == "" {
					processed.Add(1)
					continue
				}

				resultCh <- trackResult{
					path:   f.path,
					mtime:  f.mtime,
					info:   info,
					source: f.source,
					isNew:  fileIsNew[f.path],
				}
				processed.Add(1)
			}
		})
	}

	// Send work to workers
	go func() {
		for _, f := range filesToProcess {
			workCh <- f
		}
		close(workCh)
	}()

	endReports := reportProcessing(progress, &processed, total)

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results and insert into DB (sequential to avoid SQLite issues)
	for result := range resultCh {
		if err := l.upsertTrack(result.path, result.mtime, result.info); err != nil {
			// The workers end after the files they are reading: resultCh
			// holds every result, none of them blocks on it.
			failed.Store(true)
			endReports()
			return fmt.Errorf("%s %s: %w", writeVerb(result.isNew), result.path, err)
		}

		// Record stats
		relPath := relativePath(result.source, result.path)
		if sourceStats, ok := stats.BySource[result.source]; ok {
			if result.isNew {
				sourceStats.Added = append(sourceStats.Added, relPath)
			} else {
				sourceStats.Updated = append(sourceStats.Updated, relPath)
			}
		}
	}

	endReports()
	progress <- ScanProgress{Phase: "processing", Current: total, Total: total}
	return nil
}

// processingTick is the interval of the progress reports of the processing
// phase.
const processingTick = 100 * time.Millisecond

// reportProcessing sends the count of processed files out of total on
// progress at each tick, until the returned function is called: that function
// returns once the reporter has ended. A report the reader has not taken by
// then is given up: it is out of date, and the phase does not wait on the
// reader to end.
func reportProcessing(progress chan<- ScanProgress, processed *atomic.Int64, total int) (end func()) {
	done := make(chan struct{})
	var reporter sync.WaitGroup
	reporter.Go(func() {
		ticker := time.NewTicker(processingTick)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				report := ScanProgress{Phase: "processing", Current: int(processed.Load()), Total: total}
				select {
				case progress <- report:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	})
	return func() {
		close(done)
		reporter.Wait()
	}
}

// writeVerb names the write of a scanned file, for its error.
func writeVerb(isNew bool) string {
	if isNew {
		return "add track"
	}
	return "update track"
}

// getExistingTracks returns a map of path->mtime for the tracks at or under
// one of the given sources.
func (l *Library) getExistingTracks(sources []string) (map[string]int64, error) {
	rows, err := l.db.Query(`SELECT path, mtime FROM library_tracks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make(map[string]int64)
	for rows.Next() {
		var path string
		var mtime int64
		if err := rows.Scan(&path, &mtime); err != nil {
			return nil, err
		}
		if _, ok := sourceOf(path, sources); ok {
			tracks[path] = mtime
		}
	}
	return tracks, rows.Err()
}

// upsertTrack inserts or updates a track in the database.
// Uses file mtime for added_at on new tracks (preserved across copies).
func (l *Library) upsertTrack(path string, mtime int64, info *tags.Tag) error {
	return upsertTrackWithExecutor(l.db, path, mtime, info)
}

// upsertTrackWithExecutor is the internal implementation that accepts an executor.
func upsertTrackWithExecutor(ex executor, path string, mtime int64, info *tags.Tag) error {
	now := time.Now().Unix()
	_, err := ex.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre, original_date, release_date, label, added_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			mtime = excluded.mtime,
			artist = excluded.artist,
			album_artist = excluded.album_artist,
			album = excluded.album,
			title = excluded.title,
			disc_number = excluded.disc_number,
			track_number = excluded.track_number,
			year = excluded.year,
			genre = excluded.genre,
			original_date = excluded.original_date,
			release_date = excluded.release_date,
			label = excluded.label,
			updated_at = excluded.updated_at
	`, path, mtime, info.Artist, info.AlbumArtist, info.Album, info.Title, info.DiscNumber, info.TrackNumber, info.Year(), info.Genre, info.OriginalDate, info.Date, info.Label, mtime, now)
	return err
}

// deleteTrackByPath removes a track from the library by its path.
// Also updates the FTS index incrementally within a transaction.
func (l *Library) deleteTrackByPath(path string) error {
	// First fetch track info for FTS cleanup
	track, err := l.TrackByPath(path)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // no such track, nothing to delete
	}
	if err != nil {
		return err
	}

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Delete from library_tracks
	if _, err := tx.Exec(`DELETE FROM library_tracks WHERE path = ?`, path); err != nil {
		return err
	}
	if err := closePlaylistGaps(tx); err != nil {
		return err
	}

	// Remove from FTS index
	if err := removeTrackFromFTS(tx, track); err != nil {
		return err
	}

	return tx.Commit()
}

// AddTracks adds specific files to the library without doing a full scan.
// This is useful when importing files where we know exactly which files were added.
// Also incrementally updates the FTS index within a transaction.
func (l *Library) AddTracks(paths []string) error {
	// Collect track info before starting transaction (file I/O)
	type trackData struct {
		path     string
		mtime    int64
		info     *tags.Tag
		oldTrack *Track // nil if new track
	}
	tracks := make([]trackData, 0, len(paths))

	for _, path := range paths {
		info, err := tags.Read(path)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Skip files without artist or album
		if info.Artist == "" || info.Album == "" {
			continue
		}

		// Get file mtime - used for both change detection and added_at
		fileInfo, statErr := os.Stat(path)
		if statErr != nil {
			continue // Skip files we can't stat
		}
		mtime := fileInfo.ModTime().Unix()

		// Check if track already exists (for FTS update handling)
		oldTrack, _ := l.TrackByPath(path)

		tracks = append(tracks, trackData{
			path:     path,
			mtime:    mtime,
			info:     info,
			oldTrack: oldTrack,
		})
	}

	if len(tracks) == 0 {
		return nil
	}

	// Start transaction for all database operations
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	for _, t := range tracks {
		if err := upsertTrackWithExecutor(tx, t.path, t.mtime, t.info); err != nil {
			return err
		}

		// Get the new/updated track for FTS
		newTrack, err := trackByPathWithExecutor(tx, t.path)
		if err != nil {
			continue // Shouldn't happen, but skip FTS update if it does
		}

		// Update FTS index
		if t.oldTrack != nil {
			// Track existed, update FTS
			if err := updateTrackInFTS(tx, t.oldTrack, newTrack); err != nil {
				return err
			}
		} else {
			// New track, add to FTS
			if err := addTrackToFTS(tx, newTrack); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

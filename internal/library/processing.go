package library

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/llehouerou/waves/internal/player"
)

// processFiles processes files in parallel and updates the database and stats.
func (l *Library) processFiles(
	filesToProcess []fileInfo,
	fileIsNew map[string]bool,
	stats *ScanStats,
	progress chan<- ScanProgress,
) {
	total := len(filesToProcess)
	var processed atomic.Int64

	// Create work channel and results channel
	workCh := make(chan fileInfo, total)
	resultCh := make(chan trackResult, total)

	// Start workers
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for f := range workCh {
				// Skip .ogg files that are Vorbis (not Opus)
				ext := strings.ToLower(filepath.Ext(f.path))
				if ext == ".ogg" && !player.IsValidOpusFile(f.path) {
					processed.Add(1)
					continue
				}

				// Extract metadata (without duration for speed)
				info, err := player.ReadTrackInfo(f.path)
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

	// Progress reporter
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				current := int(processed.Load())
				progress <- ScanProgress{
					Phase:   "processing",
					Current: current,
					Total:   total,
				}
			case <-done:
				return
			}
		}
	}()

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results and insert into DB (sequential to avoid SQLite issues)
	for result := range resultCh {
		_ = l.upsertTrack(result.path, result.mtime, result.info)

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

	close(done)
	progress <- ScanProgress{Phase: "processing", Current: total, Total: total}
}

// getExistingTracks returns a map of path->mtime for all tracks in the given sources.
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
		// Only include tracks that belong to the sources being scanned
		for _, src := range sources {
			if strings.HasPrefix(path, src) {
				tracks[path] = mtime
				break
			}
		}
	}
	return tracks, rows.Err()
}

// upsertTrack inserts or updates a track in the database.
// Uses file mtime for added_at on new tracks (preserved across copies).
func (l *Library) upsertTrack(path string, mtime int64, info *player.TrackInfo) error {
	return upsertTrackWithExecutor(l.db, path, mtime, info)
}

// upsertTrackWithExecutor is the internal implementation that accepts an executor.
func upsertTrackWithExecutor(ex executor, path string, mtime int64, info *player.TrackInfo) error {
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
	`, path, mtime, info.Artist, info.AlbumArtist, info.Album, info.Title, info.Disc, info.Track, info.Year, info.Genre, info.OriginalDate, info.Date, info.Label, mtime, now)
	return err
}

// deleteTrackByPath removes a track from the library by its path.
// Also updates the FTS index incrementally within a transaction.
func (l *Library) deleteTrackByPath(path string) error {
	// First fetch track info for FTS cleanup
	track, err := l.TrackByPath(path)
	if err != nil {
		//nolint:nilerr // Track not found means nothing to delete, which is not an error
		return nil
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
		info     *player.TrackInfo
		oldTrack *Track // nil if new track
	}
	tracks := make([]trackData, 0, len(paths))

	for _, path := range paths {
		// Skip .ogg files that are Vorbis (not Opus)
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ogg" && !player.IsValidOpusFile(path) {
			continue
		}

		info, err := player.ReadTrackInfo(path)
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

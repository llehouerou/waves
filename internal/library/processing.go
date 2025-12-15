package library

import (
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
func (l *Library) upsertTrack(path string, mtime int64, info *player.TrackInfo) error {
	now := time.Now().Unix()
	_, err := l.db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre, added_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			updated_at = excluded.updated_at
	`, path, mtime, info.Artist, info.AlbumArtist, info.Album, info.Title, info.Disc, info.Track, info.Year, info.Genre, now, now)
	return err
}

// deleteTrackByPath removes a track from the library by its path.
func (l *Library) deleteTrackByPath(path string) error {
	_, err := l.db.Exec(`DELETE FROM library_tracks WHERE path = ?`, path)
	return err
}

// AddTracks adds specific files to the library without doing a full scan.
// This is useful when importing files where we know exactly which files were added.
func (l *Library) AddTracks(paths []string) error {
	for _, path := range paths {
		info, err := player.ReadTrackInfo(path)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Skip files without artist or album
		if info.Artist == "" || info.Album == "" {
			continue
		}

		// Get file mtime
		mtime := time.Now().Unix() // Default to now if we can't get mtime

		if err := l.upsertTrack(path, mtime, info); err != nil {
			return err
		}
	}
	return nil
}

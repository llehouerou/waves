package library

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/llehouerou/waves/internal/player"
)

const numWorkers = 8

type ScanProgress struct {
	Phase       string // "scanning", "processing", "cleaning", "done"
	Current     int
	Total       int
	CurrentFile string
}

type fileInfo struct {
	path  string
	mtime int64
}

type trackResult struct {
	path  string
	mtime int64
	info  *player.TrackInfo
}

func (l *Library) Refresh(sources []string, progress chan<- ScanProgress) error {
	return l.refresh(sources, progress, false)
}

// FullRefresh rescans all files, ignoring modification times.
// Use this to pick up metadata changes (like disc numbers) without file modifications.
func (l *Library) FullRefresh(sources []string, progress chan<- ScanProgress) error {
	return l.refresh(sources, progress, true)
}

// RefreshSource scans a single source path. Used when adding a new source.
func (l *Library) RefreshSource(source string, progress chan<- ScanProgress) error {
	return l.refresh([]string{source}, progress, false)
}

func (l *Library) refresh(sources []string, progress chan<- ScanProgress, forceRescan bool) error {
	defer close(progress)

	// Phase 1: Scan directories for music files
	progress <- ScanProgress{Phase: "scanning", Current: 0, Total: 0}

	var files []fileInfo
	for _, src := range sources {
		err := filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
			// Skip any walk errors - intentionally continuing to scan other paths
			if walkErr != nil {
				return nil //nolint:nilerr // intentionally skipping errors
			}
			if d.IsDir() {
				return nil
			}
			if !player.IsMusicFile(path) {
				return nil
			}

			info, infoErr := d.Info()
			// Skip files we can't stat - intentionally continuing to scan other files
			if infoErr != nil {
				return nil //nolint:nilerr // intentionally skipping errors
			}

			files = append(files, fileInfo{
				path:  path,
				mtime: info.ModTime().Unix(),
			})

			if len(files)%100 == 0 {
				progress <- ScanProgress{Phase: "scanning", Current: len(files), Total: 0}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Build set of discovered paths for deletion phase
	discoveredPaths := make(map[string]struct{}, len(files))
	for _, f := range files {
		discoveredPaths[f.path] = struct{}{}
	}

	// Phase 2: Get existing tracks from DB
	existingTracks, err := l.getExistingTracks()
	if err != nil {
		return err
	}

	// Filter to only new/modified files (or all files if forceRescan)
	filesToProcess := make([]fileInfo, 0, len(files))
	for _, f := range files {
		if !forceRescan {
			if existing, ok := existingTracks[f.path]; ok && existing == f.mtime {
				continue // unchanged, skip
			}
		}
		filesToProcess = append(filesToProcess, f)
	}

	// Phase 3: Process new/modified files in parallel
	total := len(filesToProcess)
	if total > 0 {
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
						path:  f.path,
						mtime: f.mtime,
						info:  info,
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
		}

		close(done)
		progress <- ScanProgress{Phase: "processing", Current: total, Total: total}
	}

	// Phase 4: Clean up deleted files
	progress <- ScanProgress{Phase: "cleaning", Current: 0, Total: 0}

	for path := range existingTracks {
		if _, exists := discoveredPaths[path]; !exists {
			_ = l.deleteTrack(path)
		}
	}

	progress <- ScanProgress{Phase: "done", Current: len(files), Total: len(files)}
	return nil
}

func (l *Library) getExistingTracks() (map[string]int64, error) {
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
		tracks[path] = mtime
	}
	return tracks, rows.Err()
}

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

func (l *Library) deleteTrack(path string) error {
	_, err := l.db.Exec(`DELETE FROM library_tracks WHERE path = ?`, path)
	return err
}

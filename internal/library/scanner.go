package library

import (
	"strings"

	"github.com/llehouerou/waves/internal/tags"
)

const numWorkers = 8

// ScanProgress reports the progress of a library scan.
type ScanProgress struct {
	Phase       string // "scanning", "processing", "cleaning", "done"
	Current     int
	Total       int
	CurrentFile string
	Stats       *ScanStats // Only populated when Phase == "done"
}

// ScanStats holds statistics for a completed scan.
type ScanStats struct {
	BySource map[string]*SourceStats // keyed by source path
}

// SourceStats holds per-source scan statistics.
type SourceStats struct {
	Added   []string // relative paths of added tracks
	Removed []string // relative paths of removed tracks
	Updated []string // relative paths of updated tracks (mtime changed)
}

// fileInfo holds information about a discovered music file.
type fileInfo struct {
	path   string
	mtime  int64
	source string // source path this file belongs to
}

// trackResult holds the result of processing a music file.
type trackResult struct {
	path   string
	mtime  int64
	info   *tags.Tag
	source string // source path this file belongs to
	isNew  bool   // true if new track, false if updated
}

// Refresh performs an incremental scan of the given source directories.
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

	// Initialize stats
	stats := &ScanStats{
		BySource: make(map[string]*SourceStats),
	}
	for _, src := range sources {
		stats.BySource[src] = &SourceStats{}
	}

	// Phase 1: Scan directories for music files
	progress <- ScanProgress{Phase: "scanning", Current: 0, Total: 0}
	files, discoveredPaths := discoverFiles(sources, progress)

	// Phase 2: Get existing tracks from DB (only from sources being scanned)
	existingTracks, err := l.getExistingTracks(sources)
	if err != nil {
		return err
	}

	// Filter to only new/modified files (or all files if forceRescan)
	// Also track which are new vs updated
	filesToProcess := make([]fileInfo, 0, len(files))
	fileIsNew := make(map[string]bool) // track if file is new or updated
	for _, f := range files {
		if !forceRescan {
			if existing, ok := existingTracks[f.path]; ok && existing == f.mtime {
				continue // unchanged, skip
			}
		}
		_, existed := existingTracks[f.path]
		fileIsNew[f.path] = !existed
		filesToProcess = append(filesToProcess, f)
	}

	// Phase 3: Process new/modified files in parallel
	if len(filesToProcess) > 0 {
		l.processFiles(filesToProcess, fileIsNew, stats, progress)
	}

	// Phase 4: Clean up deleted files
	progress <- ScanProgress{Phase: "cleaning", Current: 0, Total: 0}

	for path := range existingTracks {
		if _, exists := discoveredPaths[path]; !exists {
			_ = l.deleteTrackByPath(path)

			// Find the source this path belonged to and record removal
			for src := range stats.BySource {
				if strings.HasPrefix(path, src) {
					relPath := relativePath(src, path)
					stats.BySource[src].Removed = append(stats.BySource[src].Removed, relPath)
					break
				}
			}
		}
	}

	progress <- ScanProgress{Phase: "done", Current: len(files), Total: len(files), Stats: stats}
	return nil
}

package playlist

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/tags"
)

// FromLibraryTrack converts a library track to a playlist track.
func FromLibraryTrack(t library.Track) Track {
	return Track{
		ID:          t.ID,
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		TrackNumber: t.TrackNumber,
		DiscNumber:  t.DiscNumber,
		Genre:       t.Genre,
		Year:        t.Year,
	}
}

// FromLibraryTracks converts a slice of library tracks to playlist tracks.
func FromLibraryTracks(tracks []library.Track) []Track {
	result := make([]Track, len(tracks))
	for i := range tracks {
		result[i] = FromLibraryTrack(tracks[i])
	}
	return result
}

// FromPath creates a playlist track from a file path by reading its metadata.
func FromPath(path string) Track {
	info, err := tags.Read(path)
	if err != nil {
		// Fallback to basic info from filename
		return Track{
			Path:  path,
			Title: filepath.Base(path),
		}
	}

	return Track{
		Path:        path,
		Title:       info.Title,
		Artist:      info.Artist,
		Album:       info.Album,
		TrackNumber: info.TrackNumber,
		DiscNumber:  info.DiscNumber,
		Genre:       info.Genre,
		Year:        info.Year(),
	}
}

// isDir reports whether path is a directory. A path that cannot be read is
// taken for a file: it then fails when played, like any missing file.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// CollectFromPath collects all tracks at a path.
// For directories: recursively collects all music files
// For files: just that file
func CollectFromPath(root string) ([]Track, error) {
	if !isDir(root) {
		// Single file
		if !tags.IsMusicFile(root) {
			return nil, nil
		}
		track := FromPath(root)
		return []Track{track}, nil
	}

	// Directory - collect all music files recursively
	var tracks []Track
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip directories/files with errors, continue walking
			return nil //nolint:nilerr // intentionally skipping errors
		}
		if d.IsDir() {
			return nil
		}
		if !tags.IsMusicFile(path) {
			return nil
		}

		track := FromPath(path)
		tracks = append(tracks, track)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by path for consistent ordering
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Path < tracks[j].Path
	})

	return tracks, nil
}

// CollectFolderFromPath collects all tracks from the folder containing the given file,
// returning the tracks and the index of the selected file within that folder.
// Returns (tracks, selectedIndex, error).
func CollectFolderFromPath(filePath string) ([]Track, int, error) {
	if isDir(filePath) {
		// It's a directory - collect its contents, play from first track
		tracks, err := CollectFromPath(filePath)
		return tracks, 0, err
	}

	// It's a file - collect from parent directory
	if !tags.IsMusicFile(filePath) {
		return nil, 0, nil
	}

	parentDir := filepath.Dir(filePath)

	// Read all files in the parent directory (non-recursive)
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, 0, err
	}

	tracks := make([]Track, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryPath := filepath.Join(parentDir, entry.Name())
		if !tags.IsMusicFile(entryPath) {
			continue
		}
		track := FromPath(entryPath)
		tracks = append(tracks, track)
	}

	// Sort by path for consistent ordering
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Path < tracks[j].Path
	})

	// Find the index of the selected file
	selectedIdx := 0
	for i := range tracks {
		if tracks[i].Path == filePath {
			selectedIdx = i
			break
		}
	}

	return tracks, selectedIdx, nil
}

// WithDuration reads the duration for a track (expensive - decodes audio).
func WithDuration(t Track) Track {
	info, err := tags.ReadWithAudio(t.Path)
	if err != nil {
		return t
	}
	t.Duration = info.Duration
	return t
}

// FormatDuration formats a duration as MM:SS.
func FormatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return formatDuration(m, s)
}

func formatDuration(m, s int) string {
	return padInt(m) + ":" + padInt(s)
}

func padInt(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

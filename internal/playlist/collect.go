package playlist

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/linkfs"
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

// isMusicFile reports whether the entry at path, from a listing or a walk of
// its folder, is a music file: a symbolic link is what it points to.
func isMusicFile(path string, entry os.DirEntry) bool {
	return tags.IsMusicFile(path) && linkfs.KindOf(path, entry) == linkfs.File
}

// CollectFromPath collects all tracks at a path.
// For directories: recursively collects all music files, as the Files view
// lists them: no hidden entry (linkfs.WalkShown) below root is a track or
// walked, whatever the name of root and of the folders above it.
// For files: just that file, hidden or not.
// A directory that is a symbolic link is walked as the directory it points
// to, its tracks at paths under the link. Below it, a symbolic link is what
// it points to: a link to a music file gives its track at the link's path, a
// link to a folder is not followed and gives none, nor does a link to nothing.
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
	err := linkfs.WalkShown(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip directories/files with errors, continue walking
			return nil //nolint:nilerr // intentionally skipping errors
		}
		if !isMusicFile(path, d) {
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
// It takes the entries of that folder as CollectFromPath does those below its
// root: a hidden one (linkfs.Hidden) is left out, but for the given file, kept
// whatever its name. The name of the folder does not count.
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

	// Compared to the entries of its folder, which are clean paths.
	filePath = filepath.Clean(filePath)
	parentDir := filepath.Dir(filePath)

	// Read all files in the parent directory (non-recursive)
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, 0, err
	}

	tracks := make([]Track, 0, len(entries))
	for _, entry := range entries {
		entryPath := filepath.Join(parentDir, entry.Name())
		// The selected file is kept whatever it points to: one that cannot
		// be read fails when played.
		if entryPath != filePath && (linkfs.Hidden(entry.Name()) || !isMusicFile(entryPath, entry)) {
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

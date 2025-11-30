package playlist

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
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

// CollectFromLibraryNode collects all tracks for a library node.
// For artists: all tracks across all albums (sorted by album year, track number)
// For albums: all tracks (sorted by track number)
// For tracks: just that track
func CollectFromLibraryNode(lib *library.Library, node library.Node) ([]Track, error) {
	switch node.Level() {
	case library.LevelRoot:
		// Root level - no tracks to collect
		return nil, nil
	case library.LevelArtist:
		tracks, err := lib.ArtistTracks(node.Artist())
		if err != nil {
			return nil, err
		}
		return FromLibraryTracks(tracks), nil

	case library.LevelAlbum:
		tracks, err := lib.Tracks(node.Artist(), node.Album())
		if err != nil {
			return nil, err
		}
		return FromLibraryTracks(tracks), nil

	case library.LevelTrack:
		if t := node.Track(); t != nil {
			return []Track{FromLibraryTrack(*t)}, nil
		}
		return nil, nil

	default:
		return nil, nil
	}
}

// FromPath creates a playlist track from a file path by reading its metadata.
func FromPath(path string) Track {
	info, err := player.ReadTrackInfo(path)
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
		TrackNumber: info.Track,
		Duration:    info.Duration,
	}
}

// CollectFromFileNode collects all tracks for a file node.
// For directories: recursively collects all music files
// For files: just that file
func CollectFromFileNode(node navigator.FileNode) ([]Track, error) {
	if !node.IsContainer() {
		// Single file
		if !player.IsMusicFile(node.ID()) {
			return nil, nil
		}
		track := FromPath(node.ID())
		return []Track{track}, nil
	}

	// Directory - collect all music files recursively
	var tracks []Track
	err := filepath.WalkDir(node.ID(), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip directories/files with errors, continue walking
			return nil //nolint:nilerr // intentionally skipping errors
		}
		if d.IsDir() {
			return nil
		}
		if !player.IsMusicFile(path) {
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

// WithDuration reads the duration for a track (expensive - decodes audio).
func WithDuration(t Track) Track {
	info, err := player.ExtractFullMetadata(t.Path)
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

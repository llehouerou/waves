package playback

import (
	"time"

	"github.com/llehouerou/waves/internal/playlist"
)

// Track represents a track in the queue.
// This is a copy of the data, not a reference to playlist.Track.
type Track struct {
	ID          int64
	Path        string
	Title       string
	Artist      string
	Album       string
	TrackNumber int
	Duration    time.Duration
}

// TrackFromPlaylist converts a playlist.Track to a playback.Track.
func TrackFromPlaylist(t playlist.Track) Track {
	return Track{
		ID:          t.ID,
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		TrackNumber: t.TrackNumber,
		Duration:    t.Duration,
	}
}

// ToPlaylist converts a playback.Track to a playlist.Track.
func (t Track) ToPlaylist() playlist.Track {
	return playlist.Track{
		ID:          t.ID,
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		TrackNumber: t.TrackNumber,
		Duration:    t.Duration,
	}
}

// TracksFromPlaylist converts a slice of playlist.Track to playback.Track.
func TracksFromPlaylist(tracks []playlist.Track) []Track {
	result := make([]Track, len(tracks))
	for i, t := range tracks {
		result[i] = TrackFromPlaylist(t)
	}
	return result
}

// TracksToPlaylist converts a slice of playback.Track to playlist.Track.
func TracksToPlaylist(tracks []Track) []playlist.Track {
	result := make([]playlist.Track, len(tracks))
	for i, t := range tracks {
		result[i] = t.ToPlaylist()
	}
	return result
}

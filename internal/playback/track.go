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
	DiscNumber  int
	Genre       string
	Year        int
	Duration    time.Duration
}

// EntryID identifies one entry of the queue: a track queued twice is two
// entries. An entry keeps its ID for as long as it exists, through undo and
// redo; a replace makes new entries. The zero EntryID is no entry.
type EntryID uint64

// QueueEntry is one entry of the queue: its ID and its track.
type QueueEntry struct {
	ID    EntryID
	Track Track
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
		DiscNumber:  t.DiscNumber,
		Genre:       t.Genre,
		Year:        t.Year,
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
		DiscNumber:  t.DiscNumber,
		Genre:       t.Genre,
		Year:        t.Year,
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

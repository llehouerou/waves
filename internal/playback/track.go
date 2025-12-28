package playback

import "time"

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

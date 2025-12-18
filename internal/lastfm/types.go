package lastfm

import "time"

// ScrobbleTrack contains track metadata for scrobbling.
type ScrobbleTrack struct {
	Artist        string
	Track         string
	Album         string
	AlbumArtist   string
	Duration      time.Duration
	Timestamp     time.Time // When playback started
	MBRecordingID string    // Optional MusicBrainz recording ID
}

// ScrobbleState tracks the scrobbling status of the current track.
type ScrobbleState struct {
	TrackPath      string    // Path of current track (for dedup)
	StartedAt      time.Time // When playback started
	Scrobbled      bool      // Whether this track has been scrobbled
	NowPlayingSent bool      // Whether now playing was sent
}

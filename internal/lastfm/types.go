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

// SimilarArtist represents a similar artist from Last.fm.
type SimilarArtist struct {
	Name       string
	MatchScore float64 // 0.0-1.0 similarity score
}

// TopTrack represents a top track for an artist from Last.fm.
type TopTrack struct {
	Name      string
	Playcount int
	Rank      int
}

// UserTrack represents a track the user has scrobbled for an artist.
type UserTrack struct {
	Name      string
	Playcount int
}

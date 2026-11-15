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

// UserTrack is a track the user scrobbled, and how many times.
type UserTrack struct {
	Artist    string
	Name      string
	Playcount int
}

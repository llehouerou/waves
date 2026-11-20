package lastfm

import "time"

// PlayedTrack is a track played, as a scrobble or a "now playing" update
// tells Last.fm of it.
type PlayedTrack struct {
	Artist        string
	Track         string
	Album         string
	AlbumArtist   string
	Duration      time.Duration
	Timestamp     time.Time // When playback started; a scrobble only
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

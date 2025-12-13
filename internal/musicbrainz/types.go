// Package musicbrainz provides a client for the MusicBrainz API.
package musicbrainz

// Release represents a MusicBrainz release (album).
type Release struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Artist      string // Extracted from artist-credit
	Date        string `json:"date"`
	Country     string `json:"country"`
	TrackCount  int    // Sum of track counts from media
	Score       int    `json:"score"` // Search relevance score (0-100)
	ReleaseType string // album, single, ep, etc.
	Formats     string // CD, Vinyl, Digital, etc.
}

// Track represents a track on a release.
type Track struct {
	Position int    `json:"position"`
	Title    string `json:"title"`
	Length   int    `json:"length"` // Duration in milliseconds
}

// ReleaseDetails contains full release information including tracks.
type ReleaseDetails struct {
	Release
	Tracks []Track
}

// searchResponse is the raw response from MusicBrainz release search.
type searchResponse struct {
	Releases []releaseResult `json:"releases"`
}

// releaseResult is a single release from search results.
type releaseResult struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Score        int            `json:"score"`
	Date         string         `json:"date"`
	Country      string         `json:"country"`
	ArtistCredit []artistCredit `json:"artist-credit"`
	ReleaseGroup *releaseGroup  `json:"release-group"`
	Media        []medium       `json:"media"`
}

// artistCredit represents an artist contribution.
type artistCredit struct {
	Name   string `json:"name"`
	Artist struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
	JoinPhrase string `json:"joinphrase"`
}

// releaseGroup contains release type info.
type releaseGroup struct {
	PrimaryType string `json:"primary-type"`
}

// medium represents a disc/medium in a release.
type medium struct {
	Format     string  `json:"format"`
	TrackCount int     `json:"track-count"`
	Tracks     []track `json:"tracks"`
}

// track is a raw track from the API.
type track struct {
	Position int    `json:"position"`
	Title    string `json:"title"`
	Length   int    `json:"length"`
}

// releaseDetailsResponse is the response when fetching a single release.
type releaseDetailsResponse struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Date         string         `json:"date"`
	Country      string         `json:"country"`
	ArtistCredit []artistCredit `json:"artist-credit"`
	ReleaseGroup *releaseGroup  `json:"release-group"`
	Media        []medium       `json:"media"`
}

// Package musicbrainz provides a client for the MusicBrainz API.
package musicbrainz

// Artist represents a MusicBrainz artist.
type Artist struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Type           string `json:"type"` // Person, Group, etc.
	Country        string `json:"country"`
	Score          int    `json:"score"` // Search relevance score (0-100)
	Disambiguation string `json:"disambiguation"`
	BeginYear      string // Extracted from life-span
	EndYear        string // Extracted from life-span
}

// ReleaseGroup represents a MusicBrainz release group (album concept).
type ReleaseGroup struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	PrimaryType    string `json:"primary-type"` // Album, Single, EP, etc.
	FirstRelease   string `json:"first-release-date"`
	Artist         string // Extracted from artist-credit
	ReleaseCount   int    // Number of releases in this group
	SecondaryTypes []string
	Genres         []string // Extracted from genres array
}

// Release represents a MusicBrainz release (album).
type Release struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artist      string   // Extracted from artist-credit
	Date        string   `json:"date"`
	Country     string   `json:"country"`
	TrackCount  int      // Sum of track counts from media
	Score       int      `json:"score"` // Search relevance score (0-100)
	ReleaseType string   // album, single, ep, etc.
	Formats     string   // CD, Vinyl, Digital, etc.
	Genres      []string // Extracted from genres array
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
	Genres       []genre        `json:"genres"`
}

// genre represents a MusicBrainz genre tag.
type genre struct {
	Name string `json:"name"`
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
	Genres       []genre        `json:"genres"`
}

// artistSearchResponse is the raw response from MusicBrainz artist search.
type artistSearchResponse struct {
	Artists []artistResult `json:"artists"`
}

// artistResult is a single artist from search results.
type artistResult struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Type           string `json:"type"`
	Country        string `json:"country"`
	Score          int    `json:"score"`
	Disambiguation string `json:"disambiguation"`
	LifeSpan       *struct {
		Begin string `json:"begin"`
		End   string `json:"end"`
	} `json:"life-span"`
}

// releaseGroupBrowseResponse is the response when browsing release groups.
type releaseGroupBrowseResponse struct {
	ReleaseGroups []releaseGroupResult `json:"release-groups"`
}

// releaseGroupResult is a single release group from results.
type releaseGroupResult struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	PrimaryType    string         `json:"primary-type"`
	SecondaryTypes []string       `json:"secondary-types"`
	FirstRelease   string         `json:"first-release-date"`
	ArtistCredit   []artistCredit `json:"artist-credit"`
	Genres         []genre        `json:"genres"`
}

// releaseBrowseResponse is the response when browsing releases.
type releaseBrowseResponse struct {
	Releases []releaseResult `json:"releases"`
}

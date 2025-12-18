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
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Artist         string   // Extracted from artist-credit
	ArtistID       string   // MusicBrainz artist ID
	ArtistSortName string   // Artist sort name
	Date           string   `json:"date"`
	Country        string   `json:"country"`
	TrackCount     int      // Sum of track counts from media
	DiscCount      int      // Number of discs/media
	Score          int      `json:"score"` // Search relevance score (0-100)
	ReleaseType    string   // album, single, ep, etc.
	Status         string   // official, promotional, bootleg
	Formats        string   // CD, Vinyl, Digital, etc.
	Genres         []string // Extracted from genres array
	Label          string   // Record label name
	CatalogNumber  string   // Catalog number
	Barcode        string   // Barcode (UPC/EAN)
	Script         string   // Script (Latn, etc.)
}

// Track represents a track on a release.
type Track struct {
	Position    int    `json:"position"`
	Title       string `json:"title"`
	Length      int    `json:"length"` // Duration in milliseconds
	DiscNumber  int    // Disc number (1-based)
	RecordingID string // MusicBrainz recording ID
	TrackID     string // MusicBrainz track ID
	ISRC        string // International Standard Recording Code (first one if multiple)
	Artist      string // Track artist (if different from album artist, e.g., featuring artists)
	ArtistID    string // MusicBrainz artist ID(s), semicolon-separated for multiple artists
}

// ReleaseDetails contains full release information including tracks.
type ReleaseDetails struct {
	Release
	Tracks         []Track
	ReleaseGroupID string // MusicBrainz release group ID
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
		ID       string `json:"id"`
		Name     string `json:"name"`
		SortName string `json:"sort-name"`
	} `json:"artist"`
	JoinPhrase string `json:"joinphrase"`
}

// releaseGroup contains release type info.
type releaseGroup struct {
	ID          string `json:"id"`
	PrimaryType string `json:"primary-type"`
}

// medium represents a disc/medium in a release.
type medium struct {
	Position   int     `json:"position"`
	Format     string  `json:"format"`
	TrackCount int     `json:"track-count"`
	Tracks     []track `json:"tracks"`
}

// track is a raw track from the API.
type track struct {
	ID           string         `json:"id"`
	Position     int            `json:"position"`
	Title        string         `json:"title"`
	Length       int            `json:"length"`
	Recording    *recording     `json:"recording"`
	ArtistCredit []artistCredit `json:"artist-credit"`
}

// recording represents a MusicBrainz recording (linked from track).
type recording struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	ISRCs []string `json:"isrcs"`
}

// releaseDetailsResponse is the response when fetching a single release.
type releaseDetailsResponse struct {
	ID                 string              `json:"id"`
	Title              string              `json:"title"`
	Date               string              `json:"date"`
	Country            string              `json:"country"`
	Status             string              `json:"status"`
	Barcode            string              `json:"barcode"`
	ArtistCredit       []artistCredit      `json:"artist-credit"`
	ReleaseGroup       *releaseGroup       `json:"release-group"`
	Media              []medium            `json:"media"`
	Genres             []genre             `json:"genres"`
	LabelInfo          []labelInfo         `json:"label-info"`
	TextRepresentation *textRepresentation `json:"text-representation"`
}

// labelInfo contains label and catalog number for a release.
type labelInfo struct {
	CatalogNumber string `json:"catalog-number"`
	Label         *label `json:"label"`
}

// label represents a record label.
type label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// textRepresentation contains script info for a release.
type textRepresentation struct {
	Language string `json:"language"`
	Script   string `json:"script"`
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

// releaseGroupSearchResponse is the raw response from MusicBrainz release group search.
type releaseGroupSearchResponse struct {
	ReleaseGroups []releaseGroupResult `json:"release-groups"`
}

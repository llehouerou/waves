package library

// SearchResultType indicates the type of search result.
type SearchResultType int

const (
	ResultArtist SearchResultType = iota
	ResultAlbum
	ResultTrack
)

// SearchResult represents a search result from the library.
type SearchResult struct {
	Type        SearchResultType
	Artist      string // album_artist for navigation
	Album       string
	AlbumYear   int
	TrackID     int64
	TrackTitle  string
	TrackArtist string // actual track artist (may differ from album_artist)
	TrackNumber int
	DiscNumber  int
	Path        string
}

package library

import (
	"database/sql"

	dbutil "github.com/llehouerou/waves/internal/db"
	"github.com/llehouerou/waves/internal/search"
)

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

// RefreshSearchCache reloads the search items cache from the database.
// Call this after library scans complete or on initial load.
func (l *Library) RefreshSearchCache() error {
	results, err := l.loadSearchItems()
	if err != nil {
		return err
	}
	l.searchCache = results

	// Build search.Item slice and trigram matcher
	items := make([]search.Item, len(results))
	for i, r := range results {
		items[i] = SearchItem{Result: r}
	}
	l.searchItems = items
	l.searchMatcher = search.NewTrigramMatcher(items)
	l.searchCacheValid = true
	return nil
}

// InvalidateSearchCache marks the cache as needing refresh.
func (l *Library) InvalidateSearchCache() {
	l.searchCacheValid = false
}

// SearchItemsAndMatcher returns the cached search items and trigram matcher.
// If the cache is not valid, it refreshes it first.
func (l *Library) SearchItemsAndMatcher() ([]search.Item, *search.TrigramMatcher, error) {
	if !l.searchCacheValid {
		if err := l.RefreshSearchCache(); err != nil {
			return nil, nil, err
		}
	}
	return l.searchItems, l.searchMatcher, nil
}

// AllSearchItems returns cached searchable items from the library.
// If the cache is not valid, it refreshes it first.
func (l *Library) AllSearchItems() ([]SearchResult, error) {
	if !l.searchCacheValid {
		if err := l.RefreshSearchCache(); err != nil {
			return nil, err
		}
	}
	return l.searchCache, nil
}

// loadSearchItems loads all searchable items from the database.
func (l *Library) loadSearchItems() ([]SearchResult, error) {
	var results []SearchResult

	// Get all artists
	artists, err := l.searchArtists()
	if err != nil {
		return nil, err
	}
	results = append(results, artists...)

	// Get all albums
	albums, err := l.searchAlbums()
	if err != nil {
		return nil, err
	}
	results = append(results, albums...)

	// Get all tracks
	tracks, err := l.searchTracks()
	if err != nil {
		return nil, err
	}
	results = append(results, tracks...)

	return results, nil
}

func (l *Library) searchArtists() ([]SearchResult, error) {
	rows, err := l.db.Query(`
		SELECT DISTINCT album_artist
		FROM library_tracks
		ORDER BY album_artist COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var artist string
		if err := rows.Scan(&artist); err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			Type:   ResultArtist,
			Artist: artist,
		})
	}
	return results, rows.Err()
}

func (l *Library) searchAlbums() ([]SearchResult, error) {
	rows, err := l.db.Query(`
		SELECT album_artist, album, MAX(year) as year
		FROM library_tracks
		GROUP BY album_artist, album
		ORDER BY album COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var artist, album string
		var year sql.NullInt64
		if err := rows.Scan(&artist, &album, &year); err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			Type:      ResultAlbum,
			Artist:    artist,
			Album:     album,
			AlbumYear: int(dbutil.NullInt64Value(year)),
		})
	}
	return results, rows.Err()
}

func (l *Library) searchTracks() ([]SearchResult, error) {
	rows, err := l.db.Query(`
		SELECT id, album_artist, album, year, title, artist, track_number, disc_number, path
		FROM library_tracks
		ORDER BY title COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var trackNum, discNum *int
		r.Type = ResultTrack
		if err := rows.Scan(&r.TrackID, &r.Artist, &r.Album, &r.AlbumYear, &r.TrackTitle, &r.TrackArtist, &trackNum, &discNum, &r.Path); err != nil {
			return nil, err
		}
		if trackNum != nil {
			r.TrackNumber = *trackNum
		}
		if discNum != nil {
			r.DiscNumber = *discNum
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

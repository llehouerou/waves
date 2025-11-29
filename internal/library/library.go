package library

import (
	"database/sql"
)

type Track struct {
	ID          int64
	Path        string
	Mtime       int64
	Artist      string
	AlbumArtist string
	Album       string
	Title       string
	TrackNumber int
	Year        int
	Genre       string
}

type Album struct {
	Name string
	Year int
}

type Library struct {
	db *sql.DB
}

func New(db *sql.DB) *Library {
	return &Library{db: db}
}

func (l *Library) Artists() ([]string, error) {
	rows, err := l.db.Query(`
		SELECT DISTINCT album_artist FROM library_tracks ORDER BY album_artist COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var artist string
		if err := rows.Scan(&artist); err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}
	return artists, rows.Err()
}

func (l *Library) Albums(albumArtist string) ([]Album, error) {
	rows, err := l.db.Query(`
		SELECT album, MAX(year) as year
		FROM library_tracks
		WHERE album_artist = ?
		GROUP BY album
		ORDER BY (year IS NULL OR year = 0), year, album COLLATE NOCASE
	`, albumArtist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var a Album
		var year sql.NullInt64
		if err := rows.Scan(&a.Name, &year); err != nil {
			return nil, err
		}
		a.Year = int(year.Int64)
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (l *Library) Tracks(albumArtist, album string) ([]Track, error) {
	rows, err := l.db.Query(`
		SELECT id, path, mtime, artist, album_artist, album, title, track_number, year, genre
		FROM library_tracks
		WHERE album_artist = ? AND album = ?
		ORDER BY track_number, title COLLATE NOCASE
	`, albumArtist, album)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var t Track
		var trackNum, year sql.NullInt64
		var genre sql.NullString

		if err := rows.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
			&trackNum, &year, &genre); err != nil {
			return nil, err
		}
		t.TrackNumber = int(trackNum.Int64)
		t.Year = int(year.Int64)
		t.Genre = genre.String
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (l *Library) TrackCount() (int, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM library_tracks`).Scan(&count)
	return count, err
}

// TrackByID returns a track by its ID.
func (l *Library) TrackByID(id int64) (*Track, error) {
	row := l.db.QueryRow(`
		SELECT id, path, mtime, artist, album_artist, album, title, track_number, year, genre
		FROM library_tracks
		WHERE id = ?
	`, id)

	var t Track
	var trackNum, year sql.NullInt64
	var genre sql.NullString

	err := row.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
		&trackNum, &year, &genre)
	if err != nil {
		return nil, err
	}
	t.TrackNumber = int(trackNum.Int64)
	t.Year = int(year.Int64)
	t.Genre = genre.String
	return &t, nil
}

func (l *Library) ArtistCount() (int, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(DISTINCT album_artist) FROM library_tracks`).Scan(&count)
	return count, err
}

func (l *Library) AlbumCount() (int, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(DISTINCT album_artist || album) FROM library_tracks`).Scan(&count)
	return count, err
}

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
	Path        string
}

// AllSearchItems returns all searchable items from the library for fuzzy search.
func (l *Library) AllSearchItems() ([]SearchResult, error) {
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
			AlbumYear: int(year.Int64),
		})
	}
	return results, rows.Err()
}

func (l *Library) searchTracks() ([]SearchResult, error) {
	rows, err := l.db.Query(`
		SELECT id, album_artist, album, title, artist, path
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
		r.Type = ResultTrack
		if err := rows.Scan(&r.TrackID, &r.Artist, &r.Album, &r.TrackTitle, &r.TrackArtist, &r.Path); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

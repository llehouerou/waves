package library

import "strings"

// EnsureFTSIndex rebuilds the FTS index only if it's empty.
// Call this on startup to populate the index for existing databases.
func (l *Library) EnsureFTSIndex() error {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return l.RebuildFTSIndex()
	}
	return nil
}

// RebuildFTSIndex rebuilds the full-text search index from library_tracks.
// This should be called after library scans complete.
func (l *Library) RebuildFTSIndex() error {
	// Clear existing FTS data
	if _, err := l.db.Exec(`DELETE FROM library_search_fts`); err != nil {
		return err
	}

	// Insert artists
	if err := l.insertFTSArtists(); err != nil {
		return err
	}

	// Insert albums
	if err := l.insertFTSAlbums(); err != nil {
		return err
	}

	// Insert tracks
	if err := l.insertFTSTracks(); err != nil {
		return err
	}

	return nil
}

// insertFTSArtists adds all unique artists to the FTS index.
func (l *Library) insertFTSArtists() error {
	//nolint:dupword // SQL NULL values
	_, err := l.db.Exec(`
		INSERT INTO library_search_fts (search_text, result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path)
		SELECT DISTINCT
			album_artist,
			'artist',
			album_artist,
			NULL,
			NULL,
			NULL,
			NULL,
			NULL,
			NULL,
			NULL,
			NULL
		FROM library_tracks
		ORDER BY album_artist COLLATE NOCASE
	`)
	return err
}

// insertFTSAlbums adds all unique albums to the FTS index.
func (l *Library) insertFTSAlbums() error {
	//nolint:dupword // SQL NULL values
	_, err := l.db.Exec(`
		INSERT INTO library_search_fts (search_text, result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path)
		SELECT
			album_artist || ' ' || album,
			'album',
			album_artist,
			album,
			NULL,
			MAX(year),
			NULL,
			NULL,
			NULL,
			NULL,
			NULL
		FROM library_tracks
		GROUP BY album_artist, album
		ORDER BY album COLLATE NOCASE
	`)
	return err
}

// insertFTSTracks adds all tracks to the FTS index.
func (l *Library) insertFTSTracks() error {
	_, err := l.db.Exec(`
		INSERT INTO library_search_fts (search_text, result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path)
		SELECT
			album_artist || ' ' || album || ' ' || title || CASE WHEN artist != album_artist THEN ' ' || artist ELSE '' END,
			'track',
			album_artist,
			album,
			id,
			year,
			title,
			artist,
			track_number,
			disc_number,
			path
		FROM library_tracks
		ORDER BY title COLLATE NOCASE
	`)
	return err
}

// SearchFTS performs a full-text search using the FTS5 trigram index.
// Returns matching SearchResult items sorted by relevance.
func (l *Library) SearchFTS(query string) ([]SearchResult, error) {
	if query == "" {
		return l.getAllSearchResults()
	}

	// Escape special FTS5 characters and prepare query for trigram search
	escaped := escapeFTSQuery(query)

	rows, err := l.db.Query(`
		SELECT result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path
		FROM library_search_fts
		WHERE search_text MATCH ?
		ORDER BY rank
	`, escaped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return l.scanSearchResults(rows)
}

// getAllSearchResults returns all searchable items (used when query is empty).
func (l *Library) getAllSearchResults() ([]SearchResult, error) {
	rows, err := l.db.Query(`
		SELECT result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path
		FROM library_search_fts
		ORDER BY result_type, artist COLLATE NOCASE, album COLLATE NOCASE, track_number
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return l.scanSearchResults(rows)
}

// scanSearchResults scans rows into SearchResult slice.
func (l *Library) scanSearchResults(rows interface {
	Scan(...any) error
	Next() bool
	Err() error
}) ([]SearchResult, error) {
	var results []SearchResult

	for rows.Next() {
		var r SearchResult
		var resultType string
		var artist, album, trackTitle, trackArtist, path *string
		var trackID *int64
		var year, trackNum, discNum *int

		err := rows.Scan(&resultType, &artist, &album, &trackID, &year, &trackTitle, &trackArtist, &trackNum, &discNum, &path)
		if err != nil {
			return nil, err
		}

		switch resultType {
		case "artist":
			r.Type = ResultArtist
		case "album":
			r.Type = ResultAlbum
		case "track":
			r.Type = ResultTrack
		}

		if artist != nil {
			r.Artist = *artist
		}
		if album != nil {
			r.Album = *album
		}
		if trackID != nil {
			r.TrackID = *trackID
		}
		if year != nil {
			r.AlbumYear = *year
		}
		if trackTitle != nil {
			r.TrackTitle = *trackTitle
		}
		if trackArtist != nil {
			r.TrackArtist = *trackArtist
		}
		if trackNum != nil {
			r.TrackNumber = *trackNum
		}
		if discNum != nil {
			r.DiscNumber = *discNum
		}
		if path != nil {
			r.Path = *path
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

// escapeFTSQuery escapes a query string for FTS5 trigram search.
// Each word is wrapped in quotes for substring matching, with implicit AND between words.
func escapeFTSQuery(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return `""`
	}

	// Wrap each word in quotes for trigram substring matching
	quoted := make([]string, len(words))
	for i, word := range words {
		// Escape any double quotes within the word
		escaped := strings.ReplaceAll(word, `"`, `""`)
		quoted[i] = `"` + escaped + `"`
	}

	// Join with space (implicit AND in FTS5)
	return strings.Join(quoted, " ")
}

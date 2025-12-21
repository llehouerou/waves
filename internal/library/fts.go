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
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	if err := rebuildFTSIndex(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// rebuildFTSIndex is the internal implementation that accepts an executor.
func rebuildFTSIndex(ex executor) error {
	// Clear existing FTS data
	if _, err := ex.Exec(`DELETE FROM library_search_fts`); err != nil {
		return err
	}

	// Insert artists
	if err := insertFTSArtists(ex); err != nil {
		return err
	}

	// Insert albums
	if err := insertFTSAlbums(ex); err != nil {
		return err
	}

	// Insert tracks
	if err := insertFTSTracks(ex); err != nil {
		return err
	}

	return nil
}

// insertFTSArtists adds all unique artists to the FTS index.
func insertFTSArtists(ex executor) error {
	//nolint:dupword // SQL NULL values
	_, err := ex.Exec(`
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
func insertFTSAlbums(ex executor) error {
	//nolint:dupword // SQL NULL values
	_, err := ex.Exec(`
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
func insertFTSTracks(ex executor) error {
	_, err := ex.Exec(`
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

// SearchAlbumsFTS performs a full-text search for albums only.
// Used by album view search to only show album results.
func (l *Library) SearchAlbumsFTS(query string) ([]SearchResult, error) {
	if query == "" {
		return l.getAllAlbumResults()
	}

	// Escape special FTS5 characters and prepare query for trigram search
	escaped := escapeFTSQuery(query)

	rows, err := l.db.Query(`
		SELECT result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path
		FROM library_search_fts
		WHERE search_text MATCH ? AND result_type = 'album'
		ORDER BY rank
	`, escaped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return l.scanSearchResults(rows)
}

// getAllAlbumResults returns all albums (used when query is empty in album view).
func (l *Library) getAllAlbumResults() ([]SearchResult, error) {
	rows, err := l.db.Query(`
		SELECT result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path
		FROM library_search_fts
		WHERE result_type = 'album'
		ORDER BY artist COLLATE NOCASE, album COLLATE NOCASE
	`)
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

// AddTrackToFTS adds a single track to the FTS index.
// Also adds artist/album entries if they don't already exist.
func (l *Library) AddTrackToFTS(t *Track) error {
	return addTrackToFTS(l.db, t)
}

// addTrackToFTS is the internal implementation that accepts an executor.
func addTrackToFTS(ex executor, t *Track) error {
	// Build track search text
	searchText := t.AlbumArtist + " " + t.Album + " " + t.Title
	if t.Artist != t.AlbumArtist {
		searchText += " " + t.Artist
	}

	// Insert track
	if _, err := ex.Exec(`
		INSERT INTO library_search_fts (search_text, result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path)
		VALUES (?, 'track', ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, searchText, t.AlbumArtist, t.Album, t.ID, t.Year, t.Title, t.Artist, t.TrackNumber, t.DiscNumber, t.Path); err != nil {
		return err
	}

	// Insert artist if not exists
	//nolint:dupword // SQL NULL values
	if _, err := ex.Exec(`
		INSERT INTO library_search_fts (search_text, result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path)
		SELECT ?, 'artist', ?, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL
		WHERE NOT EXISTS (SELECT 1 FROM library_search_fts WHERE result_type = 'artist' AND artist = ?)
	`, t.AlbumArtist, t.AlbumArtist, t.AlbumArtist); err != nil {
		return err
	}

	// Insert album if not exists
	albumSearchText := t.AlbumArtist + " " + t.Album
	//nolint:dupword // SQL NULL values
	if _, err := ex.Exec(`
		INSERT INTO library_search_fts (search_text, result_type, artist, album, track_id, year, track_title, track_artist, track_number, disc_number, path)
		SELECT ?, 'album', ?, ?, NULL, ?, NULL, NULL, NULL, NULL, NULL
		WHERE NOT EXISTS (SELECT 1 FROM library_search_fts WHERE result_type = 'album' AND artist = ? AND album = ?)
	`, albumSearchText, t.AlbumArtist, t.Album, t.Year, t.AlbumArtist, t.Album); err != nil {
		return err
	}

	return nil
}

// UpdateTrackInFTS updates a track's FTS entry.
// If the artist/album changed, it handles orphaned entries.
func (l *Library) UpdateTrackInFTS(oldTrack, newTrack *Track) error {
	return updateTrackInFTS(l.db, oldTrack, newTrack)
}

// updateTrackInFTS is the internal implementation that accepts an executor.
func updateTrackInFTS(ex executor, oldTrack, newTrack *Track) error {
	// Delete old track entry
	if _, err := ex.Exec(`DELETE FROM library_search_fts WHERE result_type = 'track' AND track_id = ?`, oldTrack.ID); err != nil {
		return err
	}

	// Add new track entry (also ensures artist/album exist)
	if err := addTrackToFTS(ex, newTrack); err != nil {
		return err
	}

	// If artist changed, clean up orphaned artist entry
	if oldTrack.AlbumArtist != newTrack.AlbumArtist {
		if _, err := ex.Exec(`
			DELETE FROM library_search_fts
			WHERE result_type = 'artist' AND artist = ?
			AND NOT EXISTS (SELECT 1 FROM library_tracks WHERE album_artist = ?)
		`, oldTrack.AlbumArtist, oldTrack.AlbumArtist); err != nil {
			return err
		}
	}

	// If album changed, clean up orphaned album entry
	if oldTrack.AlbumArtist != newTrack.AlbumArtist || oldTrack.Album != newTrack.Album {
		if _, err := ex.Exec(`
			DELETE FROM library_search_fts
			WHERE result_type = 'album' AND artist = ? AND album = ?
			AND NOT EXISTS (SELECT 1 FROM library_tracks WHERE album_artist = ? AND album = ?)
		`, oldTrack.AlbumArtist, oldTrack.Album, oldTrack.AlbumArtist, oldTrack.Album); err != nil {
			return err
		}
	}

	return nil
}

// RemoveTrackFromFTS removes a track from the FTS index.
// Also removes orphaned artist/album entries.
// The track must still exist in library_tracks when this is called
// (or pass the track info directly).
func (l *Library) RemoveTrackFromFTS(t *Track) error {
	return removeTrackFromFTS(l.db, t)
}

// removeTrackFromFTS is the internal implementation that accepts an executor.
func removeTrackFromFTS(ex executor, t *Track) error {
	// Delete track from FTS
	if _, err := ex.Exec(`DELETE FROM library_search_fts WHERE result_type = 'track' AND track_id = ?`, t.ID); err != nil {
		return err
	}

	// Delete album from FTS if no more tracks exist for it
	if _, err := ex.Exec(`
		DELETE FROM library_search_fts
		WHERE result_type = 'album' AND artist = ? AND album = ?
		AND NOT EXISTS (SELECT 1 FROM library_tracks WHERE album_artist = ? AND album = ?)
	`, t.AlbumArtist, t.Album, t.AlbumArtist, t.Album); err != nil {
		return err
	}

	// Delete artist from FTS if no more tracks exist for them
	if _, err := ex.Exec(`
		DELETE FROM library_search_fts
		WHERE result_type = 'artist' AND artist = ?
		AND NOT EXISTS (SELECT 1 FROM library_tracks WHERE album_artist = ?)
	`, t.AlbumArtist, t.AlbumArtist); err != nil {
		return err
	}

	return nil
}

// RemoveTracksFromFTSByPrefix removes all FTS entries for tracks matching a path prefix.
// Used when removing a library source.
func (l *Library) RemoveTracksFromFTSByPrefix(pathPrefix string) error {
	return removeTracksFromFTSByPrefix(l.db, pathPrefix)
}

// removeTracksFromFTSByPrefix is the internal implementation that accepts an executor.
func removeTracksFromFTSByPrefix(ex executor, pathPrefix string) error {
	// Delete track entries matching the prefix
	if _, err := ex.Exec(`
		DELETE FROM library_search_fts
		WHERE result_type = 'track' AND path LIKE ?
	`, pathPrefix+"%"); err != nil {
		return err
	}

	// Clean up orphaned albums
	if _, err := ex.Exec(`
		DELETE FROM library_search_fts
		WHERE result_type = 'album'
		AND NOT EXISTS (
			SELECT 1 FROM library_tracks
			WHERE album_artist = library_search_fts.artist
			AND album = library_search_fts.album
		)
	`); err != nil {
		return err
	}

	// Clean up orphaned artists
	if _, err := ex.Exec(`
		DELETE FROM library_search_fts
		WHERE result_type = 'artist'
		AND NOT EXISTS (
			SELECT 1 FROM library_tracks
			WHERE album_artist = library_search_fts.artist
		)
	`); err != nil {
		return err
	}

	return nil
}

package radio

import (
	"database/sql"
	"time"

	"github.com/llehouerou/waves/internal/lastfm"
)

// Cache manages the Last.fm data cache in SQLite.
type Cache struct {
	db      *sql.DB
	ttlDays int
}

// NewCache creates a new Cache instance.
func NewCache(db *sql.DB, ttlDays int) *Cache {
	return &Cache{
		db:      db,
		ttlDays: ttlDays,
	}
}

// isExpired checks if a cached entry is expired.
func (c *Cache) isExpired(fetchedAt int64) bool {
	expiry := time.Now().AddDate(0, 0, -c.ttlDays).Unix()
	return fetchedAt < expiry
}

// GetSimilarArtists returns cached similar artists if not expired.
func (c *Cache) GetSimilarArtists(artist string) ([]lastfm.SimilarArtist, error) {
	rows, err := c.db.Query(`
		SELECT similar_artist, match_score, fetched_at
		FROM lastfm_similar_artists
		WHERE artist = ?
		ORDER BY match_score DESC
	`, artist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []lastfm.SimilarArtist
	var fetchedAt int64
	hasData := false

	for rows.Next() {
		var similar lastfm.SimilarArtist
		if err := rows.Scan(&similar.Name, &similar.MatchScore, &fetchedAt); err != nil {
			return nil, err
		}
		hasData = true

		// Check if any entry is expired (they should all have same timestamp)
		if c.isExpired(fetchedAt) {
			return nil, nil // Return empty to trigger refresh
		}

		result = append(result, similar)
	}

	if !hasData {
		return nil, nil
	}

	return result, rows.Err()
}

// SetSimilarArtists caches similar artists for an artist.
func (c *Cache) SetSimilarArtists(artist string, similar []lastfm.SimilarArtist) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck // rollback on error path, result doesn't matter
		}
	}()

	// Delete existing entries for this artist
	_, err = tx.Exec(`DELETE FROM lastfm_similar_artists WHERE artist = ?`, artist)
	if err != nil {
		return err
	}

	// Insert new entries
	now := time.Now().Unix()
	stmt, err := tx.Prepare(`
		INSERT INTO lastfm_similar_artists (artist, similar_artist, match_score, fetched_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range similar {
		_, err = stmt.Exec(artist, s.Name, s.MatchScore, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetArtistTopTracks returns cached top tracks if not expired.
func (c *Cache) GetArtistTopTracks(artist string) ([]lastfm.TopTrack, error) {
	rows, err := c.db.Query(`
		SELECT track_name, playcount, rank, fetched_at
		FROM lastfm_artist_top_tracks
		WHERE artist = ?
		ORDER BY rank ASC
	`, artist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []lastfm.TopTrack
	var fetchedAt int64
	hasData := false

	for rows.Next() {
		var track lastfm.TopTrack
		if err := rows.Scan(&track.Name, &track.Playcount, &track.Rank, &fetchedAt); err != nil {
			return nil, err
		}
		hasData = true

		if c.isExpired(fetchedAt) {
			return nil, nil
		}

		result = append(result, track)
	}

	if !hasData {
		return nil, nil
	}

	return result, rows.Err()
}

// SetArtistTopTracks caches top tracks for an artist.
func (c *Cache) SetArtistTopTracks(artist string, tracks []lastfm.TopTrack) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck // rollback on error path, result doesn't matter
		}
	}()

	// Delete existing entries for this artist
	_, err = tx.Exec(`DELETE FROM lastfm_artist_top_tracks WHERE artist = ?`, artist)
	if err != nil {
		return err
	}

	// Insert new entries
	now := time.Now().Unix()
	stmt, err := tx.Prepare(`
		INSERT INTO lastfm_artist_top_tracks (artist, track_name, playcount, rank, fetched_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range tracks {
		_, err = stmt.Exec(artist, t.Name, t.Playcount, t.Rank, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetUserArtistTracks returns cached user scrobbles for an artist if not expired.
func (c *Cache) GetUserArtistTracks(artist string) ([]lastfm.UserTrack, error) {
	rows, err := c.db.Query(`
		SELECT track_name, user_playcount, fetched_at
		FROM lastfm_user_artist_tracks
		WHERE artist = ?
		ORDER BY user_playcount DESC
	`, artist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []lastfm.UserTrack
	var fetchedAt int64
	hasData := false

	for rows.Next() {
		var track lastfm.UserTrack
		if err := rows.Scan(&track.Name, &track.Playcount, &fetchedAt); err != nil {
			return nil, err
		}
		hasData = true

		if c.isExpired(fetchedAt) {
			return nil, nil
		}

		result = append(result, track)
	}

	if !hasData {
		return nil, nil
	}

	return result, rows.Err()
}

// SetUserArtistTracks caches user scrobbles for an artist.
func (c *Cache) SetUserArtistTracks(artist string, tracks []lastfm.UserTrack) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck // rollback on error path, result doesn't matter
		}
	}()

	// Delete existing entries for this artist
	_, err = tx.Exec(`DELETE FROM lastfm_user_artist_tracks WHERE artist = ?`, artist)
	if err != nil {
		return err
	}

	// Insert new entries
	now := time.Now().Unix()
	stmt, err := tx.Prepare(`
		INSERT INTO lastfm_user_artist_tracks (artist, track_name, user_playcount, fetched_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range tracks {
		_, err = stmt.Exec(artist, t.Name, t.Playcount, now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CleanExpired removes all expired cache entries.
func (c *Cache) CleanExpired() error {
	expiry := time.Now().AddDate(0, 0, -c.ttlDays).Unix()

	// Delete from each table separately to avoid SQL injection risks
	_, err := c.db.Exec(`DELETE FROM lastfm_similar_artists WHERE fetched_at < ?`, expiry)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(`DELETE FROM lastfm_artist_top_tracks WHERE fetched_at < ?`, expiry)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(`DELETE FROM lastfm_user_artist_tracks WHERE fetched_at < ?`, expiry)
	if err != nil {
		return err
	}

	return nil
}

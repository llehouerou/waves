package radio

import (
	"database/sql"
	"errors"
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

// GetSimilarArtists returns the similar artists of artist Last.fm answered
// within the TTL; ok is false when there is none: never asked, or expired.
// An answer with no similar artist is ok, with none.
func (c *Cache) GetSimilarArtists(artist string) (similar []lastfm.SimilarArtist, ok bool, err error) {
	rows, err := c.db.Query(`
		SELECT similar_artist, match_score, fetched_at
		FROM lastfm_similar_artists
		WHERE artist = ?
		ORDER BY match_score DESC
	`, artist)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var fetchedAt int64
	for rows.Next() {
		var s lastfm.SimilarArtist
		if err := rows.Scan(&s.Name, &s.MatchScore, &fetchedAt); err != nil {
			return nil, false, err
		}
		similar = append(similar, s)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(similar) > 0 {
		// All the rows of an answer have the same fetched_at.
		if c.isExpired(fetchedAt) {
			return nil, false, nil
		}
		return similar, true, nil
	}

	ok, err = c.hasEmptyAnswer(`SELECT fetched_at FROM lastfm_no_similar_artists WHERE artist = ?`, artist)
	return nil, ok, err
}

// SetSimilarArtists remembers, now, similar as Last.fm's answer about
// artist, in place of the one remembered; none remembers an answer with no
// similar artist.
func (c *Cache) SetSimilarArtists(artist string, similar []lastfm.SimilarArtist) (err error) {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck // rollback on error path, result doesn't matter
		}
	}()

	if _, err = tx.Exec(`DELETE FROM lastfm_similar_artists WHERE artist = ?`, artist); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM lastfm_no_similar_artists WHERE artist = ?`, artist); err != nil {
		return err
	}

	now := time.Now().Unix()
	if len(similar) == 0 {
		_, err = tx.Exec(`INSERT INTO lastfm_no_similar_artists (artist, fetched_at) VALUES (?, ?)`, artist, now)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	stmt, err := tx.Prepare(`
		INSERT INTO lastfm_similar_artists (artist, similar_artist, match_score, fetched_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, s := range similar {
		if _, err = stmt.Exec(artist, s.Name, s.MatchScore, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetArtistTopTracks returns the top tracks of artist Last.fm answered within
// the TTL; ok is false when there are none: never asked, or expired. An
// answer with no top track is ok, with none.
func (c *Cache) GetArtistTopTracks(artist string) (tracks []lastfm.TopTrack, ok bool, err error) {
	rows, err := c.db.Query(`
		SELECT track_name, playcount, rank, fetched_at
		FROM lastfm_artist_top_tracks
		WHERE artist = ?
		ORDER BY rank ASC
	`, artist)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var fetchedAt int64
	for rows.Next() {
		var t lastfm.TopTrack
		if err := rows.Scan(&t.Name, &t.Playcount, &t.Rank, &fetchedAt); err != nil {
			return nil, false, err
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(tracks) > 0 {
		// All the rows of an answer have the same fetched_at.
		if c.isExpired(fetchedAt) {
			return nil, false, nil
		}
		return tracks, true, nil
	}

	ok, err = c.hasEmptyAnswer(`SELECT fetched_at FROM lastfm_no_artist_top_tracks WHERE artist = ?`, artist)
	return nil, ok, err
}

// SetArtistTopTracks remembers, now, tracks as Last.fm's answer about
// artist, in place of the one remembered; none remembers an answer with no
// top track.
func (c *Cache) SetArtistTopTracks(artist string, tracks []lastfm.TopTrack) (err error) {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck // rollback on error path, result doesn't matter
		}
	}()

	if _, err = tx.Exec(`DELETE FROM lastfm_artist_top_tracks WHERE artist = ?`, artist); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM lastfm_no_artist_top_tracks WHERE artist = ?`, artist); err != nil {
		return err
	}

	now := time.Now().Unix()
	if len(tracks) == 0 {
		_, err = tx.Exec(`INSERT INTO lastfm_no_artist_top_tracks (artist, fetched_at) VALUES (?, ?)`, artist, now)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	stmt, err := tx.Prepare(`
		INSERT INTO lastfm_artist_top_tracks (artist, track_name, playcount, rank, fetched_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, t := range tracks {
		if _, err = stmt.Exec(artist, t.Name, t.Playcount, t.Rank, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// hasEmptyAnswer reports whether query, reading the fetched_at of an empty
// answer about artist, finds one within the TTL.
func (c *Cache) hasEmptyAnswer(query, artist string) (bool, error) {
	var fetchedAt int64
	err := c.db.QueryRow(query, artist).Scan(&fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !c.isExpired(fetchedAt), nil
}

// GetUserChart returns the user's plays from the chart read for account
// within the TTL; ok is false when there is none: never read, read for
// another account, or expired. A chart read empty is ok, with no plays.
func (c *Cache) GetUserChart(account string) (plays map[TrackKey]int, ok bool, err error) {
	var readFor string
	var fetchedAt int64
	err = c.db.QueryRow(`SELECT account, fetched_at FROM lastfm_user_chart WHERE id = 1`).Scan(&readFor, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if readFor != account || c.isExpired(fetchedAt) {
		return nil, false, nil
	}

	rows, err := c.db.Query(`SELECT artist, title, playcount FROM lastfm_user_chart_tracks`)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	plays = make(map[TrackKey]int)
	for rows.Next() {
		var key TrackKey
		var playcount int
		if err := rows.Scan(&key.Artist, &key.Title, &playcount); err != nil {
			return nil, false, err
		}
		plays[key] = playcount
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return plays, true, nil
}

// SetUserChart replaces the chart with the user's plays read for account,
// now; none remembers a chart read empty.
func (c *Cache) SetUserChart(account string, plays map[TrackKey]int) (err error) {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck // rollback on error path, result doesn't matter
		}
	}()

	if _, err = tx.Exec(`DELETE FROM lastfm_user_chart_tracks`); err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO lastfm_user_chart (id, account, fetched_at) VALUES (1, ?, ?)
	`, account, time.Now().Unix())
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO lastfm_user_chart_tracks (artist, title, playcount) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for key, playcount := range plays {
		if _, err = stmt.Exec(key.Artist, key.Title, playcount); err != nil {
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

	_, err = c.db.Exec(`DELETE FROM lastfm_no_similar_artists WHERE fetched_at < ?`, expiry)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(`DELETE FROM lastfm_no_artist_top_tracks WHERE fetched_at < ?`, expiry)
	if err != nil {
		return err
	}

	return nil
}

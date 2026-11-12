// Package releases caches the ListenBrainz fresh-releases payload in SQLite and
// matches it against the library and the similar-artists set.
package releases

import (
	"database/sql"
	"time"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/listenbrainz"
)

// TTL is how long a cached payload stays fresh. Every refresh re-downloads the
// full ~10 MB payload: ListenBrainz exposes no ETag or Last-Modified.
const TTL = 24 * time.Hour

// Cache stores the fresh-releases payload verbatim.
type Cache struct {
	db *sql.DB
}

// NewCache creates a Cache over the given database.
func NewCache(db *sql.DB) *Cache {
	return &Cache{db: db}
}

// Replace swaps the whole cache for a new payload in one transaction, all rows
// sharing one fetched_at. An empty payload is ignored: it must not empty the cache.
// Entries that vanished from the payload vanish from the cache, no purge needed.
func (c *Cache) Replace(payload []listenbrainz.Release) error {
	if len(payload) == 0 {
		return nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback() //nolint:errcheck // rollback on error path, result doesn't matter
		}
	}()

	if _, err = tx.Exec(`DELETE FROM fresh_releases`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO fresh_releases (
			release_group_mbid, artist_credit_name, norm_artist, release_name,
			release_date, primary_type, secondary_type, listen_count, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, r := range payload {
		_, err = stmt.Exec(
			r.ReleaseGroupMBID,
			r.ArtistCreditName,
			library.NormalizeTitle(r.ArtistCreditName),
			r.ReleaseName,
			r.ReleaseDate,
			nullable(r.PrimaryType),
			nullable(r.SecondaryType),
			r.ListenCount,
			now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// lastFetch returns the timestamp of the cached payload, zero if the cache is empty.
func (c *Cache) lastFetch() (time.Time, error) {
	var fetchedAt sql.NullInt64
	if err := c.db.QueryRow(`SELECT MAX(fetched_at) FROM fresh_releases`).Scan(&fetchedAt); err != nil {
		return time.Time{}, err
	}
	if !fetchedAt.Valid {
		return time.Time{}, nil
	}
	return time.Unix(fetchedAt.Int64, 0), nil
}

// IsStale reports whether the cache is empty or older than TTL.
func (c *Cache) IsStale() (bool, error) {
	last, err := c.lastFetch()
	if err != nil {
		return false, err
	}
	return time.Since(last) > TTL, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

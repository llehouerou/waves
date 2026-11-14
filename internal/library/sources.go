package library

import (
	"path/filepath"
	"time"
)

// Sources returns all configured library source paths.
func (l *Library) Sources() ([]string, error) {
	rows, err := l.db.Query(`SELECT path FROM library_sources ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		sources = append(sources, path)
	}
	return sources, rows.Err()
}

// AddSource adds a new library source path.
func (l *Library) AddSource(path string) error {
	_, err := l.db.Exec(`
		INSERT INTO library_sources (path, added_at) VALUES (?, ?)
	`, path, time.Now().Unix())
	return err
}

// RemoveSource removes a library source path and all tracks under it.
// Also cleans up the FTS index within a transaction.
func (l *Library) RemoveSource(path string) error {
	bounds := sourceBounds(path)

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Clean up FTS index FIRST (needs library_tracks to find matching track_ids)
	if err := removeSourceFromFTS(tx, path); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM library_tracks WHERE `+underSource, bounds...); err != nil {
		return err
	}
	if err := closePlaylistGaps(tx); err != nil {
		return err
	}

	// Delete the source
	if _, err := tx.Exec(`DELETE FROM library_sources WHERE path = ?`, path); err != nil {
		return err
	}

	return tx.Commit()
}

// TrackCountBySource returns the number of tracks under a source path.
func (l *Library) TrackCountBySource(path string) (int, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM library_tracks WHERE `+underSource, sourceBounds(path)...).Scan(&count)
	return count, err
}

// underSource is the SQL condition on library_tracks.path of a track under a
// source. Its two arguments are the bounds sourceBounds returns for the source.
const underSource = `(path >= ? AND path < ?)`

// sourceBounds returns the arguments of underSource for source: the range of
// the BINARY collation of path from the source with its separator, included,
// to the source followed by the byte after the separator, excluded. A path is
// in it exactly when it starts with withSeparator(source), byte for byte, as
// isUnder says. No character of source is special in it, and the unique index
// on path serves it.
func sourceBounds(source string) []any {
	from := withSeparator(source)
	return []any{from, from[:len(from)-1] + string(filepath.Separator+1)}
}

// SourceExists checks if a source path already exists.
func (l *Library) SourceExists(path string) (bool, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM library_sources WHERE path = ?`, path).Scan(&count)
	return count > 0, err
}

// MigrateSources adds source paths if the library_sources table is empty.
// This is used to migrate from config file to database storage.
func (l *Library) MigrateSources(sources []string) error {
	// Check if table is empty
	var count int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM library_sources`).Scan(&count)
	if err != nil {
		return err
	}

	// Only migrate if empty
	if count > 0 {
		return nil
	}

	// Add each source
	for _, source := range sources {
		if source != "" {
			if err := l.AddSource(source); err != nil {
				return err
			}
		}
	}
	return nil
}

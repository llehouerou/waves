package library

import (
	"strings"
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
	// Ensure we match the directory prefix properly
	prefix := path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Clean up FTS index FIRST (needs library_tracks to find matching track_ids)
	if err := removeTracksFromFTSByPrefix(tx, prefix); err != nil {
		return err
	}

	// Delete all tracks with paths starting with this source
	if _, err := tx.Exec(`
		DELETE FROM library_tracks WHERE path LIKE ? OR path LIKE ?
	`, path+"/%", prefix+"%"); err != nil {
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
	prefix := path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	var count int
	err := l.db.QueryRow(`
		SELECT COUNT(*) FROM library_tracks WHERE path LIKE ?
	`, prefix+"%").Scan(&count)
	return count, err
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

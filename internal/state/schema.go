package state

import (
	"database/sql"
)

const currentSchemaVersion = 6

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		);

		CREATE TABLE IF NOT EXISTS navigation_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			current_path TEXT NOT NULL,
			selected_name TEXT,
			view_mode TEXT DEFAULT 'library',
			library_selected_id TEXT
		);

		CREATE TABLE IF NOT EXISTS library_tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			mtime INTEGER NOT NULL,
			artist TEXT NOT NULL,
			album_artist TEXT NOT NULL,
			album TEXT NOT NULL,
			title TEXT NOT NULL,
			track_number INTEGER,
			year INTEGER,
			genre TEXT,
			added_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_tracks_album_artist ON library_tracks(album_artist);
		CREATE INDEX IF NOT EXISTS idx_tracks_album_artist_album ON library_tracks(album_artist, album);
		CREATE INDEX IF NOT EXISTS idx_tracks_added_at ON library_tracks(added_at);

		CREATE TABLE IF NOT EXISTS queue_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			current_index INTEGER NOT NULL DEFAULT -1,
			repeat_mode INTEGER NOT NULL DEFAULT 0,
			shuffle INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS queue_tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			position INTEGER NOT NULL,
			track_id INTEGER,
			path TEXT NOT NULL,
			title TEXT NOT NULL,
			artist TEXT,
			album TEXT,
			track_number INTEGER,
			UNIQUE(position)
		);

		CREATE INDEX IF NOT EXISTS idx_queue_tracks_position ON queue_tracks(position);
	`)
	if err != nil {
		return err
	}

	// Set initial version if not exists
	_, err = db.Exec(`
		INSERT OR IGNORE INTO schema_version (version) VALUES (?)
	`, currentSchemaVersion)
	if err != nil {
		return err
	}

	// Migration: add repeat_mode and shuffle columns if missing
	_, _ = db.Exec(`ALTER TABLE queue_state ADD COLUMN repeat_mode INTEGER NOT NULL DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE queue_state ADD COLUMN shuffle INTEGER NOT NULL DEFAULT 0`)

	return nil
}

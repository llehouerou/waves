package state

import (
	"database/sql"
	"time"
)

const currentSchemaVersion = 18

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
			library_selected_id TEXT,
			playlists_selected_id TEXT
		);

		CREATE TABLE IF NOT EXISTS library_tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			mtime INTEGER NOT NULL,
			artist TEXT NOT NULL,
			album_artist TEXT NOT NULL,
			album TEXT NOT NULL,
			title TEXT NOT NULL,
			disc_number INTEGER,
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

		-- Playlist tables
		CREATE TABLE IF NOT EXISTS playlist_folders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			parent_id INTEGER REFERENCES playlist_folders(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			UNIQUE(parent_id, name)
		);

		CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_id INTEGER REFERENCES playlist_folders(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			last_used_at INTEGER NOT NULL,
			UNIQUE(folder_id, name)
		);

		CREATE TABLE IF NOT EXISTS playlist_tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			library_track_id INTEGER REFERENCES library_tracks(id) ON DELETE CASCADE,
			UNIQUE(playlist_id, position)
		);

		CREATE INDEX IF NOT EXISTS idx_playlist_tracks_playlist ON playlist_tracks(playlist_id, position);
		CREATE INDEX IF NOT EXISTS idx_playlists_last_used ON playlists(last_used_at DESC);

		-- Library source paths
		CREATE TABLE IF NOT EXISTS library_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			added_at INTEGER NOT NULL
		);

		-- FTS5 full-text search with trigram tokenizer for fast substring matching
		-- Stores denormalized search data for artists, albums, and tracks
		CREATE VIRTUAL TABLE IF NOT EXISTS library_search_fts USING fts5(
			search_text,
			result_type UNINDEXED,
			artist UNINDEXED,
			album UNINDEXED,
			track_id UNINDEXED,
			year UNINDEXED,
			track_title UNINDEXED,
			track_artist UNINDEXED,
			track_number UNINDEXED,
			disc_number UNINDEXED,
			path UNINDEXED,
			tokenize='trigram'
		);

		-- Download tracking tables
		CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mb_release_group_id TEXT NOT NULL,
			mb_release_id TEXT,
			mb_artist_name TEXT NOT NULL,
			mb_album_title TEXT NOT NULL,
			mb_release_year TEXT,
			mb_release_group_json TEXT,
			mb_release_details_json TEXT,
			slskd_username TEXT NOT NULL,
			slskd_directory TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
		CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at);

		CREATE TABLE IF NOT EXISTS download_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			download_id INTEGER NOT NULL REFERENCES downloads(id) ON DELETE CASCADE,
			filename TEXT NOT NULL,
			size INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			bytes_read INTEGER NOT NULL DEFAULT 0,
			verified_on_disk INTEGER NOT NULL DEFAULT 0,
			UNIQUE(download_id, filename)
		);

		CREATE INDEX IF NOT EXISTS idx_download_files_download_id ON download_files(download_id);

		-- Last.fm scrobbling tables
		CREATE TABLE IF NOT EXISTS lastfm_session (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			username TEXT NOT NULL,
			session_key TEXT NOT NULL,
			linked_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS lastfm_pending_scrobbles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			artist TEXT NOT NULL,
			track TEXT NOT NULL,
			album TEXT,
			duration_seconds INTEGER NOT NULL,
			timestamp INTEGER NOT NULL,
			mb_recording_id TEXT,
			attempts INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			created_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_lastfm_pending_created ON lastfm_pending_scrobbles(created_at);

		-- Last.fm radio cache tables
		CREATE TABLE IF NOT EXISTS lastfm_similar_artists (
			artist TEXT NOT NULL,
			similar_artist TEXT NOT NULL,
			match_score REAL NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, similar_artist)
		);

		CREATE TABLE IF NOT EXISTS lastfm_artist_top_tracks (
			artist TEXT NOT NULL,
			track_name TEXT NOT NULL,
			playcount INTEGER NOT NULL,
			rank INTEGER NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, track_name)
		);

		CREATE TABLE IF NOT EXISTS lastfm_user_artist_tracks (
			artist TEXT NOT NULL,
			track_name TEXT NOT NULL,
			user_playcount INTEGER NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, track_name)
		);

		CREATE INDEX IF NOT EXISTS idx_lastfm_similar_artist ON lastfm_similar_artists(artist);
		CREATE INDEX IF NOT EXISTS idx_lastfm_top_tracks_artist ON lastfm_artist_top_tracks(artist);
		CREATE INDEX IF NOT EXISTS idx_lastfm_user_tracks_artist ON lastfm_user_artist_tracks(artist);
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

	// Migration: add playlists_selected_id column if missing
	_, _ = db.Exec(`ALTER TABLE navigation_state ADD COLUMN playlists_selected_id TEXT`)

	// Migration: add disc_number column if missing
	_, _ = db.Exec(`ALTER TABLE library_tracks ADD COLUMN disc_number INTEGER`)

	// Ensure Favorites playlist exists (id=1, always at root level)
	now := time.Now().Unix()
	_, _ = db.Exec(`
		INSERT OR IGNORE INTO playlists (id, folder_id, name, created_at, last_used_at)
		VALUES (1, NULL, 'Favorites', ?, ?)
	`, now, now)

	// Migration: create FTS5 search table if not exists (for existing databases)
	_, _ = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS library_search_fts USING fts5(
			search_text,
			result_type UNINDEXED,
			artist UNINDEXED,
			album UNINDEXED,
			track_id UNINDEXED,
			year UNINDEXED,
			track_title UNINDEXED,
			track_artist UNINDEXED,
			track_number UNINDEXED,
			disc_number UNINDEXED,
			path UNINDEXED,
			tokenize='trigram'
		)
	`)

	// Migration: create downloads tables if not exists (for existing databases)
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mb_release_group_id TEXT NOT NULL,
			mb_release_id TEXT,
			mb_artist_name TEXT NOT NULL,
			mb_album_title TEXT NOT NULL,
			mb_release_year TEXT,
			mb_release_group_json TEXT,
			mb_release_details_json TEXT,
			slskd_username TEXT NOT NULL,
			slskd_directory TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)

	// Migration: add mb_release_id column if missing
	_, _ = db.Exec(`ALTER TABLE downloads ADD COLUMN mb_release_id TEXT`)
	// Migration: add JSON columns for full MusicBrainz data
	_, _ = db.Exec(`ALTER TABLE downloads ADD COLUMN mb_release_group_json TEXT`)
	_, _ = db.Exec(`ALTER TABLE downloads ADD COLUMN mb_release_details_json TEXT`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at)`)

	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS download_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			download_id INTEGER NOT NULL REFERENCES downloads(id) ON DELETE CASCADE,
			filename TEXT NOT NULL,
			size INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			bytes_read INTEGER NOT NULL DEFAULT 0,
			UNIQUE(download_id, filename)
		)
	`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_download_files_download_id ON download_files(download_id)`)

	// Migration: add verified_on_disk column if missing
	_, _ = db.Exec(`ALTER TABLE download_files ADD COLUMN verified_on_disk INTEGER NOT NULL DEFAULT 0`)

	// Migration: add original_date and release_date columns for album view
	_, _ = db.Exec(`ALTER TABLE library_tracks ADD COLUMN original_date TEXT`)
	_, _ = db.Exec(`ALTER TABLE library_tracks ADD COLUMN release_date TEXT`)

	// Migration: add library_sub_mode and album_selected_id columns for album view persistence
	_, _ = db.Exec(`ALTER TABLE navigation_state ADD COLUMN library_sub_mode TEXT`)
	_, _ = db.Exec(`ALTER TABLE navigation_state ADD COLUMN album_selected_id TEXT`)

	// Migration: add label column to library_tracks for album view grouping
	_, _ = db.Exec(`ALTER TABLE library_tracks ADD COLUMN label TEXT`)

	// Migration: add album view settings columns for multi-layer grouping/sorting persistence
	_, _ = db.Exec(`ALTER TABLE navigation_state ADD COLUMN album_group_fields TEXT`)
	_, _ = db.Exec(`ALTER TABLE navigation_state ADD COLUMN album_sort_criteria TEXT`)

	// Migration: create album_view_presets table for saved grouping/sorting configurations
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS album_view_presets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			group_fields TEXT NOT NULL,
			sort_criteria TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)

	// Insert default presets (only if they don't exist)
	now = time.Now().Unix()
	_, _ = db.Exec(`
		INSERT OR IGNORE INTO album_view_presets (name, group_fields, sort_criteria, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`,
		"Newly added",
		`{"groupFields":[4],"groupSortOrder":0,"groupDateField":3,"sortCriteria":[{"field":2,"order":0}]}`,
		`[{"field":2,"order":0}]`,
		now, now,
	)
	_, _ = db.Exec(`
		INSERT OR IGNORE INTO album_view_presets (name, group_fields, sort_criteria, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`,
		"Newly released",
		`{"groupFields":[4],"groupSortOrder":0,"groupDateField":0,"sortCriteria":[{"field":1,"order":0}]}`,
		`[{"field":1,"order":0}]`,
		now, now,
	)

	// Migration: create Last.fm tables if not exists (for existing databases)
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS lastfm_session (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			username TEXT NOT NULL,
			session_key TEXT NOT NULL,
			linked_at INTEGER NOT NULL
		)
	`)
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS lastfm_pending_scrobbles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			artist TEXT NOT NULL,
			track TEXT NOT NULL,
			album TEXT,
			duration_seconds INTEGER NOT NULL,
			timestamp INTEGER NOT NULL,
			mb_recording_id TEXT,
			attempts INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			created_at INTEGER NOT NULL
		)
	`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_lastfm_pending_created ON lastfm_pending_scrobbles(created_at)`)

	// Migration: create Last.fm radio cache tables if not exists (for existing databases)
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS lastfm_similar_artists (
			artist TEXT NOT NULL,
			similar_artist TEXT NOT NULL,
			match_score REAL NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, similar_artist)
		)
	`)
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS lastfm_artist_top_tracks (
			artist TEXT NOT NULL,
			track_name TEXT NOT NULL,
			playcount INTEGER NOT NULL,
			rank INTEGER NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, track_name)
		)
	`)
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS lastfm_user_artist_tracks (
			artist TEXT NOT NULL,
			track_name TEXT NOT NULL,
			user_playcount INTEGER NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, track_name)
		)
	`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_lastfm_similar_artist ON lastfm_similar_artists(artist)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_lastfm_top_tracks_artist ON lastfm_artist_top_tracks(artist)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_lastfm_user_tracks_artist ON lastfm_user_artist_tracks(artist)`)

	return nil
}

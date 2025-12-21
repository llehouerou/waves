package library

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the required schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create tables
	schema := `
		CREATE TABLE library_tracks (
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
			original_date TEXT,
			release_date TEXT,
			label TEXT,
			added_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE library_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			added_at INTEGER NOT NULL
		);

		CREATE VIRTUAL TABLE library_search_fts USING fts5(
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
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestAddTrackToFTS(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a track into library_tracks first
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES ('/music/track1.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	track, err := lib.TrackByPath("/music/track1.mp3")
	if err != nil {
		t.Fatalf("failed to get track: %v", err)
	}

	// Add to FTS
	if err := lib.AddTrackToFTS(track); err != nil {
		t.Fatalf("AddTrackToFTS failed: %v", err)
	}

	// Verify track entry exists
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'track' AND track_id = ?`, track.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query FTS: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 track in FTS, got %d", count)
	}

	// Verify artist entry exists
	err = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'artist' AND artist = ?`, track.AlbumArtist).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query FTS: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 artist in FTS, got %d", count)
	}

	// Verify album entry exists
	err = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'album' AND artist = ? AND album = ?`, track.AlbumArtist, track.Album).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query FTS: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 album in FTS, got %d", count)
	}
}

func TestAddTrackToFTS_SameArtistAlbum_NoDuplicates(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert two tracks from same album
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/track1.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/track2.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	track1, _ := lib.TrackByPath("/music/track1.mp3")
	track2, _ := lib.TrackByPath("/music/track2.mp3")

	// Add both to FTS
	if err := lib.AddTrackToFTS(track1); err != nil {
		t.Fatalf("AddTrackToFTS failed: %v", err)
	}
	if err := lib.AddTrackToFTS(track2); err != nil {
		t.Fatalf("AddTrackToFTS failed: %v", err)
	}

	// Should have 2 track entries
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'track'`).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 tracks in FTS, got %d", count)
	}

	// Should have only 1 artist entry
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'artist'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 artist in FTS, got %d", count)
	}

	// Should have only 1 album entry
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'album'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 album in FTS, got %d", count)
	}
}

func TestRemoveTrackFromFTS_CleansOrphanedAlbumAndArtist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a single track
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES ('/music/track1.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	track, _ := lib.TrackByPath("/music/track1.mp3")
	_ = lib.AddTrackToFTS(track)

	// Delete from library_tracks (simulating what DeleteTrack does before calling RemoveTrackFromFTS)
	_, _ = db.Exec(`DELETE FROM library_tracks WHERE id = ?`, track.ID)

	// Remove from FTS
	if err := lib.RemoveTrackFromFTS(track); err != nil {
		t.Fatalf("RemoveTrackFromFTS failed: %v", err)
	}

	// Verify all entries are gone
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 entries in FTS, got %d", count)
	}
}

func TestRemoveTrackFromFTS_KeepsSharedAlbumAndArtist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert two tracks from same album
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/track1.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/track2.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	track1, _ := lib.TrackByPath("/music/track1.mp3")
	track2, _ := lib.TrackByPath("/music/track2.mp3")

	_ = lib.AddTrackToFTS(track1)
	_ = lib.AddTrackToFTS(track2)

	// Delete track1 from library_tracks
	_, _ = db.Exec(`DELETE FROM library_tracks WHERE id = ?`, track1.ID)

	// Remove track1 from FTS
	_ = lib.RemoveTrackFromFTS(track1)

	// Should have 1 track entry left
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'track'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 track in FTS, got %d", count)
	}

	// Should still have artist (track2 uses it)
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'artist'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 artist in FTS, got %d", count)
	}

	// Should still have album (track2 uses it)
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'album'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 album in FTS, got %d", count)
	}

	// Verify it's track2 that remains
	var trackID int64
	_ = db.QueryRow(`SELECT track_id FROM library_search_fts WHERE result_type = 'track'`).Scan(&trackID)
	if trackID != track2.ID {
		t.Errorf("expected track2 (ID %d) to remain, got %d", track2.ID, trackID)
	}
}

func TestDeleteTrack_UpdatesFTS(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a track
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES ('/music/track1.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	track, _ := lib.TrackByPath("/music/track1.mp3")
	_ = lib.AddTrackToFTS(track)

	// Delete using DeleteTrack (which should handle both library_tracks and FTS)
	if err := lib.DeleteTrack(track.ID); err != nil {
		t.Fatalf("DeleteTrack failed: %v", err)
	}

	// Verify track is gone from library_tracks
	_, err = lib.TrackByID(track.ID)
	if err == nil {
		t.Error("expected track to be deleted from library_tracks")
	}

	// Verify FTS is cleaned up
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 entries in FTS after DeleteTrack, got %d", count)
	}
}

func TestUpdateTrackInFTS_ChangesArtist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a track
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES ('/music/track1.mp3', 1000, 'Artist', 'Old Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	oldTrack, _ := lib.TrackByPath("/music/track1.mp3")
	_ = lib.AddTrackToFTS(oldTrack)

	// Update the track in library_tracks
	_, _ = db.Exec(`UPDATE library_tracks SET album_artist = 'New Artist' WHERE id = ?`, oldTrack.ID)
	newTrack, _ := lib.TrackByPath("/music/track1.mp3")

	// Update FTS
	if err := lib.UpdateTrackInFTS(oldTrack, newTrack); err != nil {
		t.Fatalf("UpdateTrackInFTS failed: %v", err)
	}

	// Verify old artist is gone
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'artist' AND artist = 'Old Artist'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected old artist to be removed, got %d", count)
	}

	// Verify new artist exists
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'artist' AND artist = 'New Artist'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 new artist in FTS, got %d", count)
	}
}

func TestRebuildFTSIndex(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert some tracks
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/artist1/album1/track1.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/artist1/album1/track2.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 2', 2, 1, 2023, 1000, 1000),
			('/music/artist2/album2/track1.mp3', 1000, 'Artist 2', 'Artist 2', 'Album 2', 'Track 1', 1, 1, 2024, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	// Rebuild FTS
	if err := lib.RebuildFTSIndex(); err != nil {
		t.Fatalf("RebuildFTSIndex failed: %v", err)
	}

	var count int

	// Should have 3 track entries
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'track'`).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 tracks in FTS, got %d", count)
	}

	// Should have 2 artist entries
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'artist'`).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 artists in FTS, got %d", count)
	}

	// Should have 2 album entries
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'album'`).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 albums in FTS, got %d", count)
	}
}

func TestSearchFTS(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert tracks and rebuild FTS
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/beatles/abbey/come.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Come Together', 1, 1, 1969, 1000, 1000),
			('/music/beatles/abbey/something.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Something', 2, 1, 1969, 1000, 1000),
			('/music/stones/exile/rocks.mp3', 1000, 'Rolling Stones', 'Rolling Stones', 'Exile on Main St', 'Rocks Off', 1, 1, 1972, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	_ = lib.RebuildFTSIndex()

	// Search for "beatles"
	results, err := lib.SearchFTS("beatles")
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}

	// Should find artist, album, and 2 tracks
	if len(results) < 3 {
		t.Errorf("expected at least 3 results for 'beatles', got %d", len(results))
	}

	// Search for "abbey"
	results, err = lib.SearchFTS("abbey")
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}

	if len(results) < 1 {
		t.Errorf("expected at least 1 result for 'abbey', got %d", len(results))
	}
}

func TestRemoveTracksFromFTSByPrefix(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert tracks
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/source1/track1.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/source1/track2.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	_ = lib.RebuildFTSIndex()

	// Verify FTS was populated
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 4 { // 2 tracks + 1 album + 1 artist
		t.Fatalf("expected 4 entries in FTS, got %d", count)
	}

	// Call removeTracksFromFTSByPrefix BEFORE deleting from library_tracks
	// (FTS5 UNINDEXED columns don't support LIKE, so we need library_tracks for the join)
	if err := lib.RemoveTracksFromFTSByPrefix("/music/source1/"); err != nil {
		t.Fatalf("RemoveTracksFromFTSByPrefix failed: %v", err)
	}

	// Check what remains in FTS
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 entries in FTS, got %d", count)
	}

	// Now we can delete from library_tracks
	_, _ = db.Exec(`DELETE FROM library_tracks WHERE path LIKE '/music/source1/%'`)
}

func TestRemoveSource_CleansFTS(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add a source
	_, _ = db.Exec(`INSERT INTO library_sources (path, added_at) VALUES ('/music/source1', 1000)`)

	// Insert tracks from that source
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/source1/track1.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/source1/track2.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	_ = lib.RebuildFTSIndex()

	// Verify FTS was populated
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 4 { // 2 tracks + 1 album + 1 artist
		t.Fatalf("expected 4 entries in FTS before RemoveSource, got %d", count)
	}

	// Remove the source
	if err := lib.RemoveSource("/music/source1"); err != nil {
		t.Fatalf("RemoveSource failed: %v", err)
	}

	// Verify tracks are gone from library_tracks
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_tracks`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 tracks after RemoveSource, got %d", count)
	}

	// Debug: show what's left in FTS
	rows, _ := db.Query(`SELECT result_type, artist, album, path FROM library_search_fts`)
	defer rows.Close()
	var remaining []string
	for rows.Next() {
		var resultType, artist string
		var album, path *string
		_ = rows.Scan(&resultType, &artist, &album, &path)
		albumStr := "<nil>"
		if album != nil {
			albumStr = *album
		}
		pathStr := "<nil>"
		if path != nil {
			pathStr = *path
		}
		remaining = append(remaining, resultType+":"+artist+"/"+albumStr+" path="+pathStr)
	}

	// Verify all FTS entries are gone
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 entries in FTS after RemoveSource, got %d: %v", count, remaining)
	}
}

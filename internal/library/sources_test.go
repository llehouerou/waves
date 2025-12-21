package library

import (
	"testing"
)

func TestSources_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	sources, err := lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestAddSource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add a source
	if err := lib.AddSource("/music/library1"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	sources, err := lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0] != "/music/library1" {
		t.Errorf("expected source '/music/library1', got %s", sources[0])
	}

	// Add another source
	if err := lib.AddSource("/music/library2"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	sources, err = lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
}

func TestAddSource_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add a source
	if err := lib.AddSource("/music/library"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	// Adding duplicate should fail (unique constraint)
	err := lib.AddSource("/music/library")
	if err == nil {
		t.Error("expected error when adding duplicate source")
	}
}

func TestSources_OrderedByAddedAt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add sources in specific order
	sources := []string{"/first", "/second", "/third"}
	for _, src := range sources {
		if err := lib.AddSource(src); err != nil {
			t.Fatalf("AddSource failed: %v", err)
		}
	}

	result, err := lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}

	// Should be in order of added_at
	for i, src := range result {
		if src != sources[i] {
			t.Errorf("source[%d] = %s, expected %s", i, src, sources[i])
		}
	}
}

func TestSourceExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Non-existent source
	exists, err := lib.SourceExists("/nonexistent")
	if err != nil {
		t.Fatalf("SourceExists failed: %v", err)
	}
	if exists {
		t.Error("expected non-existent source to return false")
	}

	// Add a source
	if err := lib.AddSource("/music/library"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	// Now it should exist
	exists, err = lib.SourceExists("/music/library")
	if err != nil {
		t.Fatalf("SourceExists failed: %v", err)
	}
	if !exists {
		t.Error("expected existing source to return true")
	}

	// Slightly different path should not exist
	exists, err = lib.SourceExists("/music/library/")
	if err != nil {
		t.Fatalf("SourceExists failed: %v", err)
	}
	if exists {
		t.Error("expected '/music/library/' to not match '/music/library'")
	}
}

func TestTrackCountBySource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add source and tracks
	if err := lib.AddSource("/music/source1"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/source1/album/track1.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/source1/album/track2.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 2', 2, 1, 2023, 1000, 1000),
			('/music/source2/album/track1.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 3', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	// Count tracks in source1
	count, err := lib.TrackCountBySource("/music/source1")
	if err != nil {
		t.Fatalf("TrackCountBySource failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 tracks in source1, got %d", count)
	}

	// Count tracks in source2
	count, err = lib.TrackCountBySource("/music/source2")
	if err != nil {
		t.Fatalf("TrackCountBySource failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 track in source2, got %d", count)
	}

	// Non-existent source should return 0
	count, err = lib.TrackCountBySource("/music/nonexistent")
	if err != nil {
		t.Fatalf("TrackCountBySource failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tracks in nonexistent source, got %d", count)
	}
}

func TestTrackCountBySource_TrailingSlash(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/source/album/track1.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/source/album/track2.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	// Should work with or without trailing slash
	count1, _ := lib.TrackCountBySource("/music/source")
	count2, _ := lib.TrackCountBySource("/music/source/")

	if count1 != count2 {
		t.Errorf("trailing slash should not affect count: %d vs %d", count1, count2)
	}
	if count1 != 2 {
		t.Errorf("expected 2 tracks, got %d", count1)
	}
}

func TestRemoveSource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add source and tracks
	if err := lib.AddSource("/music/source1"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/source1/album/track1.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/source1/album/track2.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 2', 2, 1, 2023, 1000, 1000),
			('/music/other/album/track1.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 3', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	// Build FTS index
	if err := lib.RebuildFTSIndex(); err != nil {
		t.Fatalf("RebuildFTSIndex failed: %v", err)
	}

	// Remove source1
	if err := lib.RemoveSource("/music/source1"); err != nil {
		t.Fatalf("RemoveSource failed: %v", err)
	}

	// Verify source is removed
	exists, _ := lib.SourceExists("/music/source1")
	if exists {
		t.Error("source should no longer exist after removal")
	}

	// Verify tracks under source1 are removed
	count, _ := lib.TrackCount()
	if count != 1 {
		t.Errorf("expected 1 remaining track, got %d", count)
	}

	// Verify track from other source remains
	track, err := lib.TrackByPath("/music/other/album/track1.mp3")
	if err != nil {
		t.Errorf("track from other source should still exist: %v", err)
	}
	if track == nil {
		t.Error("track from other source should not be nil")
	}
}

func TestRemoveSource_CleansUpFTS(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add source and tracks
	if err := lib.AddSource("/music/source1"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/source1/album/track1.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/source1/album/track2.mp3', 1000, 'Artist 1', 'Artist 1', 'Album 1', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	// Build FTS index
	if err := lib.RebuildFTSIndex(); err != nil {
		t.Fatalf("RebuildFTSIndex failed: %v", err)
	}

	// Verify FTS has entries
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count == 0 {
		t.Fatal("expected FTS entries before removal")
	}

	// Remove source
	if err := lib.RemoveSource("/music/source1"); err != nil {
		t.Fatalf("RemoveSource failed: %v", err)
	}

	// Verify FTS is cleaned up
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 FTS entries after removal, got %d", count)
	}
}

func TestRemoveSource_KeepsSharedArtistsAlbums(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add two sources
	if err := lib.AddSource("/music/source1"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}
	if err := lib.AddSource("/music/source2"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	// Insert tracks - same artist in both sources
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/source1/album/track1.mp3', 1000, 'Shared Artist', 'Shared Artist', 'Album A', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/source2/album/track1.mp3', 1000, 'Shared Artist', 'Shared Artist', 'Album B', 'Track 1', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	// Build FTS index
	if err := lib.RebuildFTSIndex(); err != nil {
		t.Fatalf("RebuildFTSIndex failed: %v", err)
	}

	// Remove source1
	if err := lib.RemoveSource("/music/source1"); err != nil {
		t.Fatalf("RemoveSource failed: %v", err)
	}

	// Artist should still exist in FTS (has tracks in source2)
	var artistCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'artist' AND artist = 'Shared Artist'`).Scan(&artistCount)
	if artistCount != 1 {
		t.Errorf("expected shared artist to remain in FTS, got count %d", artistCount)
	}

	// Album A should be gone, Album B should remain
	var albumACount, albumBCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'album' AND album = 'Album A'`).Scan(&albumACount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts WHERE result_type = 'album' AND album = 'Album B'`).Scan(&albumBCount)

	if albumACount != 0 {
		t.Errorf("expected Album A to be removed from FTS, got count %d", albumACount)
	}
	if albumBCount != 1 {
		t.Errorf("expected Album B to remain in FTS, got count %d", albumBCount)
	}
}

func TestRemoveSource_NonExistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Removing non-existent source should not error
	err := lib.RemoveSource("/nonexistent/source")
	if err != nil {
		t.Errorf("RemoveSource on non-existent should not error, got: %v", err)
	}
}

func TestMigrateSources_EmptyTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Migrate sources to empty table
	sources := []string{"/music/lib1", "/music/lib2", "/music/lib3"}
	if err := lib.MigrateSources(sources); err != nil {
		t.Fatalf("MigrateSources failed: %v", err)
	}

	// Verify all sources were added
	result, err := lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 sources, got %d", len(result))
	}
}

func TestMigrateSources_NonEmptyTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Add existing source
	if err := lib.AddSource("/existing/source"); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	// Try to migrate - should be no-op when table is not empty
	sources := []string{"/new/lib1", "/new/lib2"}
	if err := lib.MigrateSources(sources); err != nil {
		t.Fatalf("MigrateSources failed: %v", err)
	}

	// Verify only existing source remains
	result, err := lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 source (no migration), got %d", len(result))
	}
	if result[0] != "/existing/source" {
		t.Errorf("expected existing source, got %s", result[0])
	}
}

func TestMigrateSources_SkipsEmptyStrings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Migrate with some empty strings
	sources := []string{"/valid/path", "", "/another/path", ""}
	if err := lib.MigrateSources(sources); err != nil {
		t.Fatalf("MigrateSources failed: %v", err)
	}

	// Verify only non-empty sources were added
	result, err := lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 sources (skipping empty), got %d", len(result))
	}
}

func TestMigrateSources_EmptyList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Migrate empty list
	if err := lib.MigrateSources([]string{}); err != nil {
		t.Fatalf("MigrateSources failed: %v", err)
	}

	// Verify no sources
	result, err := lib.Sources()
	if err != nil {
		t.Fatalf("Sources failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 sources, got %d", len(result))
	}
}

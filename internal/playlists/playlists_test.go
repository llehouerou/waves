//nolint:goconst // test files commonly repeat strings for test data
package playlists

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/llehouerou/waves/internal/library"
)

// setupTestDB creates an in-memory SQLite database with the schema initialized.
// Uses shared cache to ensure all connections see the same database.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Configure SQLite
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		t.Fatalf("failed to set pragma: %v", err)
	}

	// Create required tables
	_, err = db.Exec(`
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
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	return db
}

// insertTestTracks inserts some test tracks into the database.
func insertTestTracks(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	ids := make([]int64, 0, 5)

	tracks := []struct {
		path   string
		artist string
		album  string
		title  string
		num    int
	}{
		{"/music/track1.mp3", "Artist 1", "Album 1", "Track 1", 1},
		{"/music/track2.mp3", "Artist 1", "Album 1", "Track 2", 2},
		{"/music/track3.mp3", "Artist 1", "Album 1", "Track 3", 3},
		{"/music/track4.mp3", "Artist 2", "Album 2", "Track 4", 1},
		{"/music/track5.mp3", "Artist 2", "Album 2", "Track 5", 2},
	}

	for _, tr := range tracks {
		result, err := db.Exec(`
			INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
			VALUES (?, 1000, ?, ?, ?, ?, ?, 1, 2023, 1000, 1000)
		`, tr.path, tr.artist, tr.artist, tr.album, tr.title, tr.num)
		if err != nil {
			t.Fatalf("failed to insert track: %v", err)
		}
		id, _ := result.LastInsertId()
		ids = append(ids, id)
	}

	return ids
}

// Playlist CRUD tests

func TestPlaylist_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	id, err := p.Create(nil, "My Playlist")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Verify created
	pl, err := p.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if pl.Name != "My Playlist" {
		t.Errorf("Name = %q, want %q", pl.Name, "My Playlist")
	}
	if pl.FolderID != nil {
		t.Errorf("FolderID should be nil for root playlist")
	}
}

func TestPlaylist_CreateInFolder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	// Create folder first
	folderID, _ := p.CreateFolder(nil, "My Folder")

	// Create playlist in folder
	id, err := p.Create(&folderID, "Folder Playlist")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	pl, _ := p.Get(id)
	if pl.FolderID == nil || *pl.FolderID != folderID {
		t.Errorf("FolderID = %v, want %d", pl.FolderID, folderID)
	}
}

func TestPlaylist_Rename(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	id, _ := p.Create(nil, "Original Name")

	if err := p.Rename(id, "New Name"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	pl, _ := p.Get(id)
	if pl.Name != "New Name" {
		t.Errorf("Name = %q, want %q", pl.Name, "New Name")
	}
}

func TestPlaylist_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	id, _ := p.Create(nil, "To Delete")
	_ = p.AddTracks(id, trackIDs[:2])

	if err := p.Delete(id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err := p.Get(id)
	if err == nil {
		t.Error("expected error when getting deleted playlist")
	}

	// Tracks should also be deleted (cascade)
	count, _ := p.TrackCount(id)
	if count != 0 {
		t.Errorf("expected 0 tracks after delete, got %d", count)
	}
}

func TestPlaylist_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	// Create playlists
	_, _ = p.Create(nil, "Playlist A")
	_, _ = p.Create(nil, "Playlist B")

	playlists, err := p.List(nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Note: Favorites playlist is created by default in real schema,
	// but not in our test setup
	if len(playlists) != 2 {
		t.Errorf("expected 2 playlists, got %d", len(playlists))
	}

	// Should be sorted by name
	if playlists[0].Name != "Playlist A" {
		t.Errorf("first playlist = %q, want %q", playlists[0].Name, "Playlist A")
	}
}

func TestPlaylist_ListInFolder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	folderID, _ := p.CreateFolder(nil, "Folder")
	_, _ = p.Create(nil, "Root Playlist")
	_, _ = p.Create(&folderID, "Folder Playlist")

	// List root playlists
	rootPlaylists, _ := p.List(nil)
	if len(rootPlaylists) != 1 {
		t.Errorf("expected 1 root playlist, got %d", len(rootPlaylists))
	}

	// List folder playlists
	folderPlaylists, _ := p.List(&folderID)
	if len(folderPlaylists) != 1 {
		t.Errorf("expected 1 folder playlist, got %d", len(folderPlaylists))
	}
	if folderPlaylists[0].Name != "Folder Playlist" {
		t.Errorf("folder playlist = %q, want %q", folderPlaylists[0].Name, "Folder Playlist")
	}
}

func TestPlaylist_UpdateLastUsed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	id, _ := p.Create(nil, "Test")

	// Set last_used_at to an old value
	_, _ = db.Exec(`UPDATE playlists SET last_used_at = 1000 WHERE id = ?`, id)

	pl1, _ := p.Get(id)
	if pl1.LastUsedAt != 1000 {
		t.Fatalf("expected last_used_at = 1000, got %d", pl1.LastUsedAt)
	}

	// Update should set to current time
	if err := p.UpdateLastUsed(id); err != nil {
		t.Fatalf("UpdateLastUsed failed: %v", err)
	}

	pl2, _ := p.Get(id)
	if pl2.LastUsedAt <= pl1.LastUsedAt {
		t.Error("LastUsedAt should be updated to a more recent value")
	}
}

func TestPlaylist_IsPlaylistEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	id, _ := p.Create(nil, "Test")

	// Initially empty
	empty, err := p.IsPlaylistEmpty(id)
	if err != nil {
		t.Fatalf("IsPlaylistEmpty failed: %v", err)
	}
	if !empty {
		t.Error("expected empty playlist")
	}

	// Add tracks
	_ = p.AddTracks(id, trackIDs[:1])

	empty, _ = p.IsPlaylistEmpty(id)
	if empty {
		t.Error("expected non-empty playlist after adding tracks")
	}
}

// Folder tests

func TestFolder_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	id, err := p.CreateFolder(nil, "My Folder")
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	folder, err := p.FolderByID(id)
	if err != nil {
		t.Fatalf("FolderByID failed: %v", err)
	}
	if folder.Name != "My Folder" {
		t.Errorf("Name = %q, want %q", folder.Name, "My Folder")
	}
}

func TestFolder_Nested(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	parentID, _ := p.CreateFolder(nil, "Parent")
	childID, _ := p.CreateFolder(&parentID, "Child")

	child, _ := p.FolderByID(childID)
	if child.ParentID == nil || *child.ParentID != parentID {
		t.Errorf("ParentID = %v, want %d", child.ParentID, parentID)
	}
}

func TestFolder_Rename(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	id, _ := p.CreateFolder(nil, "Original")
	if err := p.RenameFolder(id, "Renamed"); err != nil {
		t.Fatalf("RenameFolder failed: %v", err)
	}

	folder, _ := p.FolderByID(id)
	if folder.Name != "Renamed" {
		t.Errorf("Name = %q, want %q", folder.Name, "Renamed")
	}
}

func TestFolder_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	id, _ := p.CreateFolder(nil, "To Delete")

	if err := p.DeleteFolder(id); err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}

	_, err := p.FolderByID(id)
	if err == nil {
		t.Error("expected error when getting deleted folder")
	}
}

func TestFolder_DeleteCascade(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	// Create folder with subfolder and playlist
	folderID, _ := p.CreateFolder(nil, "Parent")
	subfolderID, _ := p.CreateFolder(&folderID, "Child")
	playlistID, _ := p.Create(&folderID, "Playlist")

	// Delete parent folder
	if err := p.DeleteFolder(folderID); err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}

	// Verify subfolder is deleted
	_, err := p.FolderByID(subfolderID)
	if err == nil {
		t.Error("subfolder should be deleted via cascade")
	}

	// Verify playlist is deleted
	_, err = p.Get(playlistID)
	if err == nil {
		t.Error("playlist should be deleted via cascade")
	}
}

func TestFolder_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	_, _ = p.CreateFolder(nil, "Folder B")
	_, _ = p.CreateFolder(nil, "Folder A")

	folders, err := p.Folders(nil)
	if err != nil {
		t.Fatalf("Folders failed: %v", err)
	}
	if len(folders) != 2 {
		t.Errorf("expected 2 folders, got %d", len(folders))
	}

	// Should be sorted by name
	if folders[0].Name != "Folder A" {
		t.Errorf("first folder = %q, want %q", folders[0].Name, "Folder A")
	}
}

func TestFolder_IsFolderEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	folderID, _ := p.CreateFolder(nil, "Folder")

	// Initially empty
	empty, err := p.IsFolderEmpty(folderID)
	if err != nil {
		t.Fatalf("IsFolderEmpty failed: %v", err)
	}
	if !empty {
		t.Error("expected empty folder")
	}

	// Add subfolder
	_, _ = p.CreateFolder(&folderID, "Subfolder")
	empty, _ = p.IsFolderEmpty(folderID)
	if empty {
		t.Error("expected non-empty folder with subfolder")
	}
}

// Track management tests

func TestTracks_AddAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")

	// Add tracks
	if err := p.AddTracks(playlistID, trackIDs[:3]); err != nil {
		t.Fatalf("AddTracks failed: %v", err)
	}

	// Get tracks
	tracks, err := p.Tracks(playlistID)
	if err != nil {
		t.Fatalf("Tracks failed: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(tracks))
	}

	// Verify track data
	if tracks[0].Title != "Track 1" {
		t.Errorf("first track = %q, want %q", tracks[0].Title, "Track 1")
	}
	if tracks[0].Path != "/music/track1.mp3" {
		t.Errorf("first track path = %q, want %q", tracks[0].Path, "/music/track1.mp3")
	}
}

func TestTracks_AddPreservesOrder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")

	// Add in specific order (3, 1, 2)
	_ = p.AddTracks(playlistID, []int64{trackIDs[2], trackIDs[0], trackIDs[1]})

	tracks, _ := p.Tracks(playlistID)
	if tracks[0].Title != "Track 3" {
		t.Errorf("first track = %q, want Track 3", tracks[0].Title)
	}
	if tracks[1].Title != "Track 1" {
		t.Errorf("second track = %q, want Track 1", tracks[1].Title)
	}
	if tracks[2].Title != "Track 2" {
		t.Errorf("third track = %q, want Track 2", tracks[2].Title)
	}
}

func TestTracks_AddToExisting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")

	// Add first batch
	_ = p.AddTracks(playlistID, trackIDs[:2])

	// Add second batch
	_ = p.AddTracks(playlistID, trackIDs[2:4])

	tracks, _ := p.Tracks(playlistID)
	if len(tracks) != 4 {
		t.Errorf("expected 4 tracks, got %d", len(tracks))
	}

	// Verify order
	if tracks[2].Title != "Track 3" {
		t.Errorf("third track = %q, want Track 3", tracks[2].Title)
	}
}

func TestTracks_TrackCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")

	count, _ := p.TrackCount(playlistID)
	if count != 0 {
		t.Errorf("expected 0 tracks, got %d", count)
	}

	_ = p.AddTracks(playlistID, trackIDs)

	count, _ = p.TrackCount(playlistID)
	if count != 5 {
		t.Errorf("expected 5 tracks, got %d", count)
	}
}

func TestTracks_RemoveTrack(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")
	_ = p.AddTracks(playlistID, trackIDs[:3])

	// Remove middle track (position 1)
	if err := p.RemoveTrack(playlistID, 1); err != nil {
		t.Fatalf("RemoveTrack failed: %v", err)
	}

	tracks, _ := p.Tracks(playlistID)
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks after remove, got %d", len(tracks))
	}

	// Verify positions are compacted
	if tracks[0].Title != "Track 1" {
		t.Errorf("first track = %q, want Track 1", tracks[0].Title)
	}
	if tracks[1].Title != "Track 3" {
		t.Errorf("second track = %q, want Track 3", tracks[1].Title)
	}
}

func TestTracks_RemoveTracks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")
	_ = p.AddTracks(playlistID, trackIDs)

	// Remove positions 1 and 3
	if err := p.RemoveTracks(playlistID, []int{1, 3}); err != nil {
		t.Fatalf("RemoveTracks failed: %v", err)
	}

	tracks, _ := p.Tracks(playlistID)
	if len(tracks) != 3 {
		t.Fatalf("expected 3 tracks after remove, got %d", len(tracks))
	}

	// Verify remaining tracks
	if tracks[0].Title != "Track 1" {
		t.Errorf("first track = %q, want Track 1", tracks[0].Title)
	}
	if tracks[1].Title != "Track 3" {
		t.Errorf("second track = %q, want Track 3", tracks[1].Title)
	}
	if tracks[2].Title != "Track 5" {
		t.Errorf("third track = %q, want Track 5", tracks[2].Title)
	}
}

func TestTracks_ClearTracks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")
	_ = p.AddTracks(playlistID, trackIDs)

	if err := p.ClearTracks(playlistID); err != nil {
		t.Fatalf("ClearTracks failed: %v", err)
	}

	count, _ := p.TrackCount(playlistID)
	if count != 0 {
		t.Errorf("expected 0 tracks after clear, got %d", count)
	}
}

func TestTracks_SetTracks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")
	_ = p.AddTracks(playlistID, trackIDs[:2])

	// Replace with different tracks
	if err := p.SetTracks(playlistID, trackIDs[2:]); err != nil {
		t.Fatalf("SetTracks failed: %v", err)
	}

	tracks, _ := p.Tracks(playlistID)
	if len(tracks) != 3 {
		t.Fatalf("expected 3 tracks after set, got %d", len(tracks))
	}

	if tracks[0].Title != "Track 3" {
		t.Errorf("first track = %q, want Track 3", tracks[0].Title)
	}
}

func TestTracks_MoveIndicesDown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")
	_ = p.AddTracks(playlistID, trackIDs)

	// Move track at position 0 down by 2
	newPositions, err := p.MoveIndices(playlistID, []int{0}, 2)
	if err != nil {
		t.Fatalf("MoveIndices failed: %v", err)
	}

	if len(newPositions) != 1 || newPositions[0] != 2 {
		t.Errorf("newPositions = %v, want [2]", newPositions)
	}

	tracks, _ := p.Tracks(playlistID)
	// Original order: 1, 2, 3, 4, 5
	// After moving position 0 down by 2: 2, 3, 1, 4, 5
	if tracks[0].Title != "Track 2" {
		t.Errorf("position 0 = %q, want Track 2", tracks[0].Title)
	}
	if tracks[2].Title != "Track 1" {
		t.Errorf("position 2 = %q, want Track 1", tracks[2].Title)
	}
}

func TestTracks_MoveIndicesUp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")
	_ = p.AddTracks(playlistID, trackIDs)

	// Move track at position 1 up by 1
	newPositions, err := p.MoveIndices(playlistID, []int{1}, -1)
	if err != nil {
		t.Fatalf("MoveIndices failed: %v", err)
	}

	if len(newPositions) != 1 || newPositions[0] != 0 {
		t.Errorf("newPositions = %v, want [0]", newPositions)
	}

	tracks, _ := p.Tracks(playlistID)
	// Original order: 1, 2, 3, 4, 5
	// After moving position 1 up by 1: 2, 1, 3, 4, 5
	if tracks[0].Title != "Track 2" {
		t.Errorf("position 0 = %q, want Track 2", tracks[0].Title)
	}
	if tracks[1].Title != "Track 1" {
		t.Errorf("position 1 = %q, want Track 1", tracks[1].Title)
	}
}

func TestTracks_MoveIndicesNoop(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	playlistID, _ := p.Create(nil, "Test")
	_ = p.AddTracks(playlistID, trackIDs)

	// Delta 0 should be noop
	positions, _ := p.MoveIndices(playlistID, []int{2}, 0)
	if positions[0] != 2 {
		t.Errorf("expected position unchanged, got %d", positions[0])
	}

	// Empty positions should be noop
	positions, _ = p.MoveIndices(playlistID, []int{}, 5)
	if len(positions) != 0 {
		t.Errorf("expected empty positions, got %v", positions)
	}
}

// Favorites tests

func TestFavorites_IsFavorite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create Favorites playlist (ID=1)
	_, _ = db.Exec(`INSERT INTO playlists (id, folder_id, name, created_at, last_used_at) VALUES (1, NULL, 'Favorites', 1000, 1000)`)

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	// Not a favorite initially
	isFav, err := p.IsFavorite(trackIDs[0])
	if err != nil {
		t.Fatalf("IsFavorite failed: %v", err)
	}
	if isFav {
		t.Error("expected track to not be a favorite initially")
	}

	// Add to favorites
	_ = p.AddTracks(FavoritesPlaylistID, []int64{trackIDs[0]})

	isFav, _ = p.IsFavorite(trackIDs[0])
	if !isFav {
		t.Error("expected track to be a favorite after adding")
	}
}

func TestFavorites_Toggle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create Favorites playlist
	_, _ = db.Exec(`INSERT INTO playlists (id, folder_id, name, created_at, last_used_at) VALUES (1, NULL, 'Favorites', 1000, 1000)`)

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	// Toggle on
	isFav, err := p.ToggleFavorite(trackIDs[0])
	if err != nil {
		t.Fatalf("ToggleFavorite failed: %v", err)
	}
	if !isFav {
		t.Error("expected true after toggling on")
	}

	// Verify it's now a favorite
	check, _ := p.IsFavorite(trackIDs[0])
	if !check {
		t.Error("track should be favorited")
	}

	// Toggle off
	isFav, _ = p.ToggleFavorite(trackIDs[0])
	if isFav {
		t.Error("expected false after toggling off")
	}

	// Verify it's no longer a favorite
	check, _ = p.IsFavorite(trackIDs[0])
	if check {
		t.Error("track should not be favorited")
	}
}

func TestFavorites_ToggleFavorites(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create Favorites playlist
	_, _ = db.Exec(`INSERT INTO playlists (id, folder_id, name, created_at, last_used_at) VALUES (1, NULL, 'Favorites', 1000, 1000)`)

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	// Toggle multiple tracks
	result, err := p.ToggleFavorites(trackIDs[:3])
	if err != nil {
		t.Fatalf("ToggleFavorites failed: %v", err)
	}

	for _, id := range trackIDs[:3] {
		if !result[id] {
			t.Errorf("expected track %d to be favorited", id)
		}
	}
}

func TestFavorites_FavoriteTrackIDs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create Favorites playlist
	_, _ = db.Exec(`INSERT INTO playlists (id, folder_id, name, created_at, last_used_at) VALUES (1, NULL, 'Favorites', 1000, 1000)`)

	p := New(db, library.New(db))
	trackIDs := insertTestTracks(t, db)

	// Initially empty
	favorites, err := p.FavoriteTrackIDs()
	if err != nil {
		t.Fatalf("FavoriteTrackIDs failed: %v", err)
	}
	if len(favorites) != 0 {
		t.Errorf("expected 0 favorites, got %d", len(favorites))
	}

	// Add some favorites
	_ = p.AddTracks(FavoritesPlaylistID, trackIDs[:2])

	favorites, _ = p.FavoriteTrackIDs()
	if len(favorites) != 2 {
		t.Errorf("expected 2 favorites, got %d", len(favorites))
	}

	if !favorites[trackIDs[0]] {
		t.Error("first track should be in favorites")
	}
	if !favorites[trackIDs[1]] {
		t.Error("second track should be in favorites")
	}
	if favorites[trackIDs[2]] {
		t.Error("third track should not be in favorites")
	}
}

// Search tests

func TestSearch_AllForAddToPlaylist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	_, _ = p.Create(nil, "Playlist A")
	_, _ = p.Create(nil, "Playlist B")

	items, err := p.AllForAddToPlaylist()
	if err != nil {
		t.Fatalf("AllForAddToPlaylist failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestSearch_AllForAddToPlaylist_WithFolders(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := New(db, library.New(db))

	folderID, _ := p.CreateFolder(nil, "Rock")
	_, _ = p.Create(&folderID, "Classic Rock")
	_, _ = p.Create(nil, "Root Playlist")

	items, err := p.AllForAddToPlaylist()
	if err != nil {
		t.Fatalf("AllForAddToPlaylist failed: %v", err)
	}

	// Find the playlist in folder
	for _, item := range items {
		if item.Name == "Classic Rock" {
			if item.FolderPath != "Rock" {
				t.Errorf("FolderPath = %q, want %q", item.FolderPath, "Rock")
			}
		}
	}
}

package state

import (
	"database/sql"
	"testing"
	"testing/synctest"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the schema initialized.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Configure SQLite
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			t.Fatalf("failed to set pragma: %v", err)
		}
	}

	if err := initSchema(db); err != nil {
		db.Close()
		t.Fatalf("failed to init schema: %v", err)
	}

	return db
}

// TestGetNavigation_Empty tests getting navigation from empty database.
func TestGetNavigation_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	nav, err := getNavigation(db)
	if err != nil {
		t.Fatalf("getNavigation failed: %v", err)
	}
	if nav != nil {
		t.Errorf("expected nil navigation on empty db, got %+v", nav)
	}
}

// TestSaveAndGetNavigation tests saving and retrieving navigation state.
func TestSaveAndGetNavigation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Save navigation state
	state := NavigationState{
		CurrentPath:         "/music/artist",
		SelectedName:        "Some Album",
		ViewMode:            "library",
		LibrarySelectedID:   "library:artist:SomeArtist",
		PlaylistsSelectedID: "playlists:playlist:5",
		LibrarySubMode:      "miller",
		AlbumSelectedID:     "Artist:Album",
		AlbumGroupFields:    `{"groupFields":[0]}`,
		AlbumSortCriteria:   `[{"field":0,"order":0}]`,
	}

	if err := saveNavigation(db, state); err != nil {
		t.Fatalf("saveNavigation failed: %v", err)
	}

	// Retrieve navigation state
	retrieved, err := getNavigation(db)
	if err != nil {
		t.Fatalf("getNavigation failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected non-nil navigation")
	}

	// Verify all fields
	if retrieved.CurrentPath != state.CurrentPath {
		t.Errorf("CurrentPath = %q, want %q", retrieved.CurrentPath, state.CurrentPath)
	}
	if retrieved.SelectedName != state.SelectedName {
		t.Errorf("SelectedName = %q, want %q", retrieved.SelectedName, state.SelectedName)
	}
	if retrieved.ViewMode != state.ViewMode {
		t.Errorf("ViewMode = %q, want %q", retrieved.ViewMode, state.ViewMode)
	}
	if retrieved.LibrarySelectedID != state.LibrarySelectedID {
		t.Errorf("LibrarySelectedID = %q, want %q", retrieved.LibrarySelectedID, state.LibrarySelectedID)
	}
	if retrieved.PlaylistsSelectedID != state.PlaylistsSelectedID {
		t.Errorf("PlaylistsSelectedID = %q, want %q", retrieved.PlaylistsSelectedID, state.PlaylistsSelectedID)
	}
	if retrieved.LibrarySubMode != state.LibrarySubMode {
		t.Errorf("LibrarySubMode = %q, want %q", retrieved.LibrarySubMode, state.LibrarySubMode)
	}
	if retrieved.AlbumSelectedID != state.AlbumSelectedID {
		t.Errorf("AlbumSelectedID = %q, want %q", retrieved.AlbumSelectedID, state.AlbumSelectedID)
	}
	if retrieved.AlbumGroupFields != state.AlbumGroupFields {
		t.Errorf("AlbumGroupFields = %q, want %q", retrieved.AlbumGroupFields, state.AlbumGroupFields)
	}
	if retrieved.AlbumSortCriteria != state.AlbumSortCriteria {
		t.Errorf("AlbumSortCriteria = %q, want %q", retrieved.AlbumSortCriteria, state.AlbumSortCriteria)
	}
}

// TestSaveNavigation_Update tests updating existing navigation state.
func TestSaveNavigation_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Save initial state
	state1 := NavigationState{
		CurrentPath: "/initial/path",
		ViewMode:    "library",
	}
	if err := saveNavigation(db, state1); err != nil {
		t.Fatalf("saveNavigation failed: %v", err)
	}

	// Update with new state
	state2 := NavigationState{
		CurrentPath: "/updated/path",
		ViewMode:    "file",
	}
	if err := saveNavigation(db, state2); err != nil {
		t.Fatalf("saveNavigation (update) failed: %v", err)
	}

	// Verify update
	retrieved, _ := getNavigation(db)
	if retrieved.CurrentPath != "/updated/path" {
		t.Errorf("expected updated path, got %q", retrieved.CurrentPath)
	}
	if retrieved.ViewMode != "file" {
		t.Errorf("expected updated view mode, got %q", retrieved.ViewMode)
	}
}

// TestGetQueue_Empty tests getting queue from empty database.
func TestGetQueue_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	queue, err := getQueue(db)
	if err != nil {
		t.Fatalf("getQueue failed: %v", err)
	}
	if queue == nil {
		t.Fatal("expected non-nil queue")
	}
	if queue.CurrentIndex != -1 {
		t.Errorf("expected CurrentIndex -1 for empty queue, got %d", queue.CurrentIndex)
	}
	if len(queue.Tracks) != 0 {
		t.Errorf("expected 0 tracks, got %d", len(queue.Tracks))
	}
}

// TestSaveAndGetQueue tests saving and retrieving queue state.
func TestSaveAndGetQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Save queue state
	state := QueueState{
		CurrentIndex: 2,
		RepeatMode:   1,
		Shuffle:      true,
		Tracks: []QueueTrack{
			{TrackID: 1, Path: "/music/track1.mp3", Title: "Track 1", Artist: "Artist 1", Album: "Album 1", TrackNumber: 1},
			{TrackID: 2, Path: "/music/track2.mp3", Title: "Track 2", Artist: "Artist 1", Album: "Album 1", TrackNumber: 2},
			{TrackID: 3, Path: "/music/track3.mp3", Title: "Track 3", Artist: "Artist 2", Album: "Album 2", TrackNumber: 1},
		},
	}

	if err := saveQueue(db, state); err != nil {
		t.Fatalf("saveQueue failed: %v", err)
	}

	// Retrieve queue state
	retrieved, err := getQueue(db)
	if err != nil {
		t.Fatalf("getQueue failed: %v", err)
	}

	// Verify state
	if retrieved.CurrentIndex != state.CurrentIndex {
		t.Errorf("CurrentIndex = %d, want %d", retrieved.CurrentIndex, state.CurrentIndex)
	}
	if retrieved.RepeatMode != state.RepeatMode {
		t.Errorf("RepeatMode = %d, want %d", retrieved.RepeatMode, state.RepeatMode)
	}
	if retrieved.Shuffle != state.Shuffle {
		t.Errorf("Shuffle = %v, want %v", retrieved.Shuffle, state.Shuffle)
	}

	// Verify tracks
	if len(retrieved.Tracks) != len(state.Tracks) {
		t.Fatalf("expected %d tracks, got %d", len(state.Tracks), len(retrieved.Tracks))
	}

	for i, track := range retrieved.Tracks {
		expected := state.Tracks[i]
		if track.TrackID != expected.TrackID {
			t.Errorf("track[%d].TrackID = %d, want %d", i, track.TrackID, expected.TrackID)
		}
		if track.Path != expected.Path {
			t.Errorf("track[%d].Path = %q, want %q", i, track.Path, expected.Path)
		}
		if track.Title != expected.Title {
			t.Errorf("track[%d].Title = %q, want %q", i, track.Title, expected.Title)
		}
		if track.Artist != expected.Artist {
			t.Errorf("track[%d].Artist = %q, want %q", i, track.Artist, expected.Artist)
		}
		if track.Album != expected.Album {
			t.Errorf("track[%d].Album = %q, want %q", i, track.Album, expected.Album)
		}
		if track.TrackNumber != expected.TrackNumber {
			t.Errorf("track[%d].TrackNumber = %d, want %d", i, track.TrackNumber, expected.TrackNumber)
		}
	}
}

// TestSaveQueue_ClearsExisting tests that saving queue replaces existing tracks.
func TestSaveQueue_ClearsExisting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Save initial queue with 3 tracks
	state1 := QueueState{
		CurrentIndex: 0,
		Tracks: []QueueTrack{
			{Path: "/track1.mp3", Title: "Track 1"},
			{Path: "/track2.mp3", Title: "Track 2"},
			{Path: "/track3.mp3", Title: "Track 3"},
		},
	}
	if err := saveQueue(db, state1); err != nil {
		t.Fatalf("saveQueue failed: %v", err)
	}

	// Save new queue with 1 track
	state2 := QueueState{
		CurrentIndex: 0,
		Tracks: []QueueTrack{
			{Path: "/new_track.mp3", Title: "New Track"},
		},
	}
	if err := saveQueue(db, state2); err != nil {
		t.Fatalf("saveQueue (update) failed: %v", err)
	}

	// Verify only new track exists
	retrieved, _ := getQueue(db)
	if len(retrieved.Tracks) != 1 {
		t.Errorf("expected 1 track after update, got %d", len(retrieved.Tracks))
	}
	if retrieved.Tracks[0].Path != "/new_track.mp3" {
		t.Errorf("expected new track, got %q", retrieved.Tracks[0].Path)
	}
}

// TestSaveQueue_PreservesOrder tests that track order is preserved.
func TestSaveQueue_PreservesOrder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	state := QueueState{
		Tracks: []QueueTrack{
			{Path: "/z.mp3", Title: "Z"},
			{Path: "/a.mp3", Title: "A"},
			{Path: "/m.mp3", Title: "M"},
		},
	}
	if err := saveQueue(db, state); err != nil {
		t.Fatalf("saveQueue failed: %v", err)
	}

	retrieved, _ := getQueue(db)
	for i, track := range retrieved.Tracks {
		if track.Path != state.Tracks[i].Path {
			t.Errorf("track[%d].Path = %q, want %q (order not preserved)", i, track.Path, state.Tracks[i].Path)
		}
	}
}

// TestSaveQueue_NullableTrackID tests handling of zero TrackID.
func TestSaveQueue_NullableTrackID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	state := QueueState{
		Tracks: []QueueTrack{
			{TrackID: 0, Path: "/external.mp3", Title: "External Track"},
			{TrackID: 5, Path: "/library.mp3", Title: "Library Track"},
		},
	}
	if err := saveQueue(db, state); err != nil {
		t.Fatalf("saveQueue failed: %v", err)
	}

	retrieved, _ := getQueue(db)
	if retrieved.Tracks[0].TrackID != 0 {
		t.Errorf("expected TrackID 0 for external track, got %d", retrieved.Tracks[0].TrackID)
	}
	if retrieved.Tracks[1].TrackID != 5 {
		t.Errorf("expected TrackID 5 for library track, got %d", retrieved.Tracks[1].TrackID)
	}
}

// Manager tests

func TestManager_GetSaveQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Empty queue
	queue, err := m.GetQueue()
	if err != nil {
		t.Fatalf("GetQueue failed: %v", err)
	}
	if queue.CurrentIndex != -1 {
		t.Errorf("expected -1 for empty queue")
	}

	// Save and retrieve
	state := QueueState{
		CurrentIndex: 1,
		RepeatMode:   2,
		Shuffle:      true,
		Tracks: []QueueTrack{
			{Path: "/test.mp3", Title: "Test"},
		},
	}
	if err := m.SaveQueue(state); err != nil {
		t.Fatalf("SaveQueue failed: %v", err)
	}

	retrieved, _ := m.GetQueue()
	if retrieved.CurrentIndex != 1 {
		t.Errorf("CurrentIndex = %d, want 1", retrieved.CurrentIndex)
	}
}

func TestManager_GetNavigation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Empty navigation
	nav, err := m.GetNavigation()
	if err != nil {
		t.Fatalf("GetNavigation failed: %v", err)
	}
	if nav != nil {
		t.Errorf("expected nil navigation on empty db")
	}

	// Save directly and retrieve via Manager
	state := NavigationState{CurrentPath: "/test"}
	_ = saveNavigation(db, state)

	nav, err = m.GetNavigation()
	if err != nil {
		t.Fatalf("GetNavigation failed: %v", err)
	}
	if nav == nil || nav.CurrentPath != "/test" {
		t.Errorf("expected navigation with CurrentPath /test")
	}
}

func TestManager_DB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}
	if m.DB() != db {
		t.Error("DB() should return the underlying database")
	}
}

// Last.fm tests

func TestGetLastfmSession_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	session, err := m.GetLastfmSession()
	if err != nil {
		t.Fatalf("GetLastfmSession failed: %v", err)
	}
	if session != nil {
		t.Errorf("expected nil session on empty db, got %+v", session)
	}
}

func TestSaveAndGetLastfmSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Save session
	if err := m.SaveLastfmSession("testuser", "abc123sessionkey"); err != nil {
		t.Fatalf("SaveLastfmSession failed: %v", err)
	}

	// Retrieve session
	session, err := m.GetLastfmSession()
	if err != nil {
		t.Fatalf("GetLastfmSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if session.Username != "testuser" {
		t.Errorf("Username = %q, want %q", session.Username, "testuser")
	}
	if session.SessionKey != "abc123sessionkey" {
		t.Errorf("SessionKey = %q, want %q", session.SessionKey, "abc123sessionkey")
	}
	if session.LinkedAt.IsZero() {
		t.Error("LinkedAt should not be zero")
	}
}

func TestSaveLastfmSession_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Save initial session
	_ = m.SaveLastfmSession("user1", "key1")

	// Update with new session
	_ = m.SaveLastfmSession("user2", "key2")

	session, _ := m.GetLastfmSession()
	if session.Username != "user2" {
		t.Errorf("expected updated username")
	}
	if session.SessionKey != "key2" {
		t.Errorf("expected updated session key")
	}
}

func TestDeleteLastfmSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Save session
	_ = m.SaveLastfmSession("testuser", "testkey")

	// Delete session
	if err := m.DeleteLastfmSession(); err != nil {
		t.Fatalf("DeleteLastfmSession failed: %v", err)
	}

	// Verify deleted
	session, _ := m.GetLastfmSession()
	if session != nil {
		t.Errorf("expected nil session after delete, got %+v", session)
	}
}

func TestDeleteLastfmSession_NoSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Delete non-existent session should not error
	if err := m.DeleteLastfmSession(); err != nil {
		t.Errorf("DeleteLastfmSession on empty should not error: %v", err)
	}
}

// Pending scrobbles tests

func TestAddAndGetPendingScrobbles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Empty initially
	scrobbles, err := m.GetPendingScrobbles()
	if err != nil {
		t.Fatalf("GetPendingScrobbles failed: %v", err)
	}
	if len(scrobbles) != 0 {
		t.Errorf("expected 0 scrobbles, got %d", len(scrobbles))
	}

	// Add scrobbles
	s1 := PendingScrobble{
		Artist:        "Artist 1",
		Track:         "Track 1",
		Album:         "Album 1",
		DurationSecs:  180,
		Timestamp:     time.Now().Add(-time.Hour),
		MBRecordingID: "mb-123",
	}
	s2 := PendingScrobble{
		Artist:       "Artist 2",
		Track:        "Track 2",
		DurationSecs: 240,
		Timestamp:    time.Now(),
	}

	if err := m.AddPendingScrobble(s1); err != nil {
		t.Fatalf("AddPendingScrobble failed: %v", err)
	}
	if err := m.AddPendingScrobble(s2); err != nil {
		t.Fatalf("AddPendingScrobble failed: %v", err)
	}

	// Get scrobbles
	scrobbles, err = m.GetPendingScrobbles()
	if err != nil {
		t.Fatalf("GetPendingScrobbles failed: %v", err)
	}
	if len(scrobbles) != 2 {
		t.Fatalf("expected 2 scrobbles, got %d", len(scrobbles))
	}

	// Verify first scrobble
	if scrobbles[0].Artist != "Artist 1" {
		t.Errorf("scrobble[0].Artist = %q, want %q", scrobbles[0].Artist, "Artist 1")
	}
	if scrobbles[0].Album != "Album 1" {
		t.Errorf("scrobble[0].Album = %q, want %q", scrobbles[0].Album, "Album 1")
	}
	if scrobbles[0].MBRecordingID != "mb-123" {
		t.Errorf("scrobble[0].MBRecordingID = %q, want %q", scrobbles[0].MBRecordingID, "mb-123")
	}

	// Verify second scrobble (no album)
	if scrobbles[1].Album != "" {
		t.Errorf("scrobble[1].Album should be empty, got %q", scrobbles[1].Album)
	}
}

func TestDeletePendingScrobble(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Add scrobble
	s := PendingScrobble{
		Artist:       "Artist",
		Track:        "Track",
		DurationSecs: 180,
		Timestamp:    time.Now(),
	}
	_ = m.AddPendingScrobble(s)

	// Get ID
	scrobbles, _ := m.GetPendingScrobbles()
	id := scrobbles[0].ID

	// Delete
	if err := m.DeletePendingScrobble(id); err != nil {
		t.Fatalf("DeletePendingScrobble failed: %v", err)
	}

	// Verify deleted
	scrobbles, _ = m.GetPendingScrobbles()
	if len(scrobbles) != 0 {
		t.Errorf("expected 0 scrobbles after delete, got %d", len(scrobbles))
	}
}

func TestUpdatePendingScrobbleAttempt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Add scrobble
	s := PendingScrobble{
		Artist:       "Artist",
		Track:        "Track",
		DurationSecs: 180,
		Timestamp:    time.Now(),
	}
	_ = m.AddPendingScrobble(s)

	scrobbles, _ := m.GetPendingScrobbles()
	id := scrobbles[0].ID

	// Initial state
	if scrobbles[0].Attempts != 0 {
		t.Errorf("expected 0 attempts initially, got %d", scrobbles[0].Attempts)
	}

	// Update attempt
	if err := m.UpdatePendingScrobbleAttempt(id, "connection error"); err != nil {
		t.Fatalf("UpdatePendingScrobbleAttempt failed: %v", err)
	}

	scrobbles, _ = m.GetPendingScrobbles()
	if scrobbles[0].Attempts != 1 {
		t.Errorf("expected 1 attempt after update, got %d", scrobbles[0].Attempts)
	}
	if scrobbles[0].LastError != "connection error" {
		t.Errorf("LastError = %q, want %q", scrobbles[0].LastError, "connection error")
	}

	// Update again
	_ = m.UpdatePendingScrobbleAttempt(id, "timeout")
	scrobbles, _ = m.GetPendingScrobbles()
	if scrobbles[0].Attempts != 2 {
		t.Errorf("expected 2 attempts after second update, got %d", scrobbles[0].Attempts)
	}
}

func TestDeleteOldPendingScrobbles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := &Manager{db: db}

	// Add scrobble
	s := PendingScrobble{
		Artist:       "Artist",
		Track:        "Track",
		DurationSecs: 180,
		Timestamp:    time.Now(),
	}
	_ = m.AddPendingScrobble(s)

	// Delete with 1 hour max age (should keep the scrobble)
	if err := m.DeleteOldPendingScrobbles(time.Hour); err != nil {
		t.Fatalf("DeleteOldPendingScrobbles failed: %v", err)
	}
	scrobbles, _ := m.GetPendingScrobbles()
	if len(scrobbles) != 1 {
		t.Errorf("expected scrobble to be kept (recent), got %d", len(scrobbles))
	}

	// Manually set old created_at
	_, _ = db.Exec(`UPDATE lastfm_pending_scrobbles SET created_at = ?`, time.Now().Add(-2*time.Hour).Unix())

	// Delete with 1 hour max age (should delete the scrobble)
	if err := m.DeleteOldPendingScrobbles(time.Hour); err != nil {
		t.Fatalf("DeleteOldPendingScrobbles failed: %v", err)
	}
	scrobbles, _ = m.GetPendingScrobbles()
	if len(scrobbles) != 0 {
		t.Errorf("expected scrobble to be deleted (old), got %d", len(scrobbles))
	}
}

// SaveNavigation debounce tests

func TestManager_SaveNavigation_Debounce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		db := setupTestDB(t)
		m := &Manager{db: db}
		defer m.Close()

		// Rapid successive calls - only last should be saved
		m.SaveNavigation(NavigationState{CurrentPath: "/path1"})
		m.SaveNavigation(NavigationState{CurrentPath: "/path2"})
		m.SaveNavigation(NavigationState{CurrentPath: "/path3"})

		// Before debounce timeout, nothing should be saved
		nav, _ := getNavigation(db)
		if nav != nil {
			t.Errorf("expected nil before debounce, got %+v", nav)
		}

		// Advance time past debounce (500ms)
		time.Sleep(saveDebounce + 10*time.Millisecond)
		synctest.Wait()

		// Only last state should be saved
		nav, err := getNavigation(db)
		if err != nil {
			t.Fatalf("getNavigation failed: %v", err)
		}
		if nav == nil {
			t.Fatal("expected navigation after debounce")
		}
		if nav.CurrentPath != "/path3" {
			t.Errorf("CurrentPath = %q, want /path3", nav.CurrentPath)
		}
	})
}

func TestManager_SaveNavigation_DebounceResets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		db := setupTestDB(t)
		m := &Manager{db: db}
		defer m.Close()

		// First call
		m.SaveNavigation(NavigationState{CurrentPath: "/first"})

		// Wait 400ms (before debounce triggers)
		time.Sleep(400 * time.Millisecond)

		// Second call resets the timer
		m.SaveNavigation(NavigationState{CurrentPath: "/second"})

		// Wait another 400ms (total 800ms from first, but only 400ms from second)
		time.Sleep(400 * time.Millisecond)

		// Should still not be saved (timer was reset)
		nav, _ := getNavigation(db)
		if nav != nil {
			t.Errorf("expected nil - timer should have been reset")
		}

		// Wait remaining time for second call's timer
		time.Sleep(110 * time.Millisecond)
		synctest.Wait()

		// Now it should be saved
		nav, _ = getNavigation(db)
		if nav == nil || nav.CurrentPath != "/second" {
			t.Errorf("expected /second after reset debounce, got %v", nav)
		}
	})
}

func TestManager_Close_FlushesPending(t *testing.T) {
	// Use file-based DB since we need to read after Close
	dbPath := t.TempDir() + "/test.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := initSchema(db); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	synctest.Test(t, func(t *testing.T) {
		m := &Manager{db: db}

		// Save navigation (starts debounce timer)
		m.SaveNavigation(NavigationState{CurrentPath: "/pending"})

		// Close immediately (before debounce)
		if err := m.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})

	// Reopen DB to verify flush
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db2.Close()

	nav, err := getNavigation(db2)
	if err != nil {
		t.Fatalf("getNavigation failed: %v", err)
	}
	if nav == nil {
		t.Fatal("expected navigation to be flushed on Close")
	}
	if nav.CurrentPath != "/pending" {
		t.Errorf("CurrentPath = %q, want /pending", nav.CurrentPath)
	}
}

func TestManager_Close_NoPending(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		db := setupTestDB(t)
		m := &Manager{db: db}

		// Close without any pending state
		err := m.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Should not error and db should be empty
		nav, _ := getNavigation(db)
		if nav != nil {
			t.Errorf("expected nil navigation, got %+v", nav)
		}
	})
}

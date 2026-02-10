package librarybrowser

import (
	"database/sql"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/llehouerou/waves/internal/library"
)

// Test data constants.
const (
	artistA = "Artist A"
	artistB = "Artist B"
	artistC = "Artist C"
	albumA1 = "Album A1"
	albumA2 = "Album A2"
	albumB1 = "Album B1"
)

// setupTestDB creates an in-memory SQLite database with schema and test data.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

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

// insertTestData populates the database with a realistic test dataset:
//   - Artist A: Album A1 (2020, 2 tracks), Album A2 (2022, 1 track)
//   - Artist B: Album B1 (2021, 3 tracks)
//   - Artist C: Album C1 (2019, 1 track)
func insertTestData(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, disc_number, track_number, year, added_at, updated_at)
		VALUES
			('/music/a/a1/01.mp3', 1000, 'Artist A', 'Artist A', 'Album A1', 'Track A1-1', 1, 1, 2020, 1000, 1000),
			('/music/a/a1/02.mp3', 1000, 'Artist A', 'Artist A', 'Album A1', 'Track A1-2', 1, 2, 2020, 1000, 1000),
			('/music/a/a2/01.mp3', 1000, 'Artist A', 'Artist A', 'Album A2', 'Track A2-1', 1, 1, 2022, 1000, 1000),
			('/music/b/b1/01.mp3', 1000, 'Artist B', 'Artist B', 'Album B1', 'Track B1-1', 1, 1, 2021, 1000, 1000),
			('/music/b/b1/02.mp3', 1000, 'Artist B', 'Artist B', 'Album B1', 'Track B1-2', 1, 2, 2021, 1000, 1000),
			('/music/b/b1/03.mp3', 1000, 'Artist B', 'Artist B', 'Album B1', 'Track B1-3', 1, 3, 2021, 1000, 1000),
			('/music/c/c1/01.mp3', 1000, 'Artist C', 'Artist C', 'Album C1', 'Track C1-1', 1, 1, 2019, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}
}

// newTestBrowser creates a browser model backed by an in-memory DB with test data.
func newTestBrowser(t *testing.T) Model {
	t.Helper()

	db := setupTestDB(t)
	t.Cleanup(func() { db.Close() })

	insertTestData(t, db)

	lib := library.New(db)
	m := New(lib)
	m.SetSize(120, 30)
	if err := m.Refresh(); err != nil {
		t.Fatalf("Refresh() failed: %v", err)
	}
	return m
}

// --- Refresh and initial state ---

func TestRefresh_LoadsArtists(t *testing.T) {
	m := newTestBrowser(t)

	if len(m.artists) != 3 {
		t.Fatalf("expected 3 artists, got %d", len(m.artists))
	}

	// Artists should be sorted alphabetically
	want := []string{artistA, artistB, artistC}
	for i, w := range want {
		if m.artists[i] != w {
			t.Errorf("artist[%d] = %q, want %q", i, m.artists[i], w)
		}
	}
}

func TestRefresh_LoadsAlbumsForFirstArtist(t *testing.T) {
	m := newTestBrowser(t)

	// First artist is artistA which has 2 albums
	if len(m.albums) != 2 {
		t.Fatalf("expected 2 albums for Artist A, got %d", len(m.albums))
	}

	// Albums sorted by year: Album A1 (2020) then Album A2 (2022)
	if m.albums[0].Name != albumA1 {
		t.Errorf("album[0] = %q, want %q", m.albums[0].Name, albumA1)
	}
	if m.albums[1].Name != albumA2 {
		t.Errorf("album[1] = %q, want %q", m.albums[1].Name, albumA2)
	}
}

func TestRefresh_LoadsTracksForFirstAlbum(t *testing.T) {
	m := newTestBrowser(t)

	// First album is albumA1 which has 2 tracks
	if len(m.tracks) != 2 {
		t.Fatalf("expected 2 tracks for Album A1, got %d", len(m.tracks))
	}

	if m.tracks[0].Title != "Track A1-1" {
		t.Errorf("track[0] = %q, want %q", m.tracks[0].Title, "Track A1-1")
	}
	if m.tracks[1].Title != "Track A1-2" {
		t.Errorf("track[1] = %q, want %q", m.tracks[1].Title, "Track A1-2")
	}
}

// --- Selection accessors ---

func TestSelectedArtist(t *testing.T) {
	m := newTestBrowser(t)

	if got := m.SelectedArtist(); got != artistA {
		t.Errorf("SelectedArtist() = %q, want %q", got, artistA)
	}
}

func TestSelectedAlbum(t *testing.T) {
	m := newTestBrowser(t)

	album := m.SelectedAlbum()
	if album == nil {
		t.Fatal("SelectedAlbum() = nil, want non-nil")
	}
	if album.Name != albumA1 {
		t.Errorf("SelectedAlbum().Name = %q, want %q", album.Name, albumA1)
	}
}

func TestSelectedTrack(t *testing.T) {
	m := newTestBrowser(t)

	track := m.SelectedTrack()
	if track == nil {
		t.Fatal("SelectedTrack() = nil, want non-nil")
	}
	if track.Title != "Track A1-1" {
		t.Errorf("SelectedTrack().Title = %q, want %q", track.Title, "Track A1-1")
	}
}

func TestSelectedArtist_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := library.New(db)
	m := New(lib)
	_ = m.Refresh()

	if got := m.SelectedArtist(); got != "" {
		t.Errorf("SelectedArtist() on empty = %q, want empty", got)
	}
	if got := m.SelectedAlbum(); got != nil {
		t.Errorf("SelectedAlbum() on empty = %v, want nil", got)
	}
	if got := m.SelectedTrack(); got != nil {
		t.Errorf("SelectedTrack() on empty = %v, want nil", got)
	}
}

// --- SelectArtist resets albums and tracks ---

func TestSelectArtist_ResetsAlbumsAndTracks(t *testing.T) {
	m := newTestBrowser(t)

	// Move album cursor to position 1 (Album A2) and track cursor
	m.albumCursor.SetPos(1)
	m.loadTracksForSelectedAlbum()
	m.trackCursor.SetPos(0)

	// Now switch to Artist B
	m.SelectArtist(artistB)

	// Album cursor should be reset to 0
	if m.albumCursor.Pos() != 0 {
		t.Errorf("album cursor pos = %d, want 0 after SelectArtist", m.albumCursor.Pos())
	}

	// Track cursor should be reset to 0
	if m.trackCursor.Pos() != 0 {
		t.Errorf("track cursor pos = %d, want 0 after SelectArtist", m.trackCursor.Pos())
	}

	// Albums should be for Artist B
	if len(m.albums) != 1 {
		t.Fatalf("expected 1 album for Artist B, got %d", len(m.albums))
	}
	if m.albums[0].Name != albumB1 {
		t.Errorf("album[0] = %q, want %q", m.albums[0].Name, albumB1)
	}

	// Tracks should be for Album B1
	if len(m.tracks) != 3 {
		t.Fatalf("expected 3 tracks for Album B1, got %d", len(m.tracks))
	}
}

func TestSelectArtist_NonExistent(t *testing.T) {
	m := newTestBrowser(t)

	// Select non-existent artist should not change state
	m.SelectArtist("NonExistent Artist")

	if m.SelectedArtist() != artistA {
		t.Errorf("SelectedArtist() = %q, want %q (should not change)", m.SelectedArtist(), artistA)
	}
}

// --- SelectAlbum resets tracks ---

func TestSelectAlbum_ResetsTracks(t *testing.T) {
	m := newTestBrowser(t)

	// Move track cursor to position 1
	m.trackCursor.SetPos(1)

	// Select Album A2
	m.SelectAlbum(albumA2)

	// Track cursor should be reset to 0
	if m.trackCursor.Pos() != 0 {
		t.Errorf("track cursor pos = %d, want 0 after SelectAlbum", m.trackCursor.Pos())
	}

	// Tracks should be for Album A2
	if len(m.tracks) != 1 {
		t.Fatalf("expected 1 track for Album A2, got %d", len(m.tracks))
	}
	if m.tracks[0].Title != "Track A2-1" {
		t.Errorf("track[0] = %q, want %q", m.tracks[0].Title, "Track A2-1")
	}
}

func TestSelectAlbum_NonExistent(t *testing.T) {
	m := newTestBrowser(t)

	m.SelectAlbum("NonExistent Album")

	// Should stay on Album A1
	if album := m.SelectedAlbum(); album == nil || album.Name != albumA1 {
		t.Errorf("SelectedAlbum() = %v, want Album A1 (should not change)", album)
	}
}

// --- SelectTrackByID ---

func TestSelectTrackByID(t *testing.T) {
	m := newTestBrowser(t)

	// Get the ID of the second track
	if len(m.tracks) < 2 {
		t.Fatal("need at least 2 tracks for this test")
	}
	secondTrackID := m.tracks[1].ID

	m.SelectTrackByID(secondTrackID)

	if m.trackCursor.Pos() != 1 {
		t.Errorf("track cursor pos = %d, want 1", m.trackCursor.Pos())
	}
}

func TestSelectTrackByID_NonExistent(t *testing.T) {
	m := newTestBrowser(t)

	m.SelectTrackByID(99999)

	// Should not change cursor
	if m.trackCursor.Pos() != 0 {
		t.Errorf("track cursor pos = %d, want 0 (should not change)", m.trackCursor.Pos())
	}
}

// --- Column widths ---

func TestColumnWidths_ActiveArtists(t *testing.T) {
	m := newTestBrowser(t)
	m.activeColumn = ColumnArtists

	w1, w2, w3 := m.columnWidths()

	// width=120, available=120-6=114
	// active=artists → w1=57 (50%), w2=28 (25%), w3=114-57-28=29
	if w1 != 57 {
		t.Errorf("w1 = %d, want 57", w1)
	}
	if w2 != 28 {
		t.Errorf("w2 = %d, want 28", w2)
	}
	if w3 != 29 {
		t.Errorf("w3 = %d, want 29", w3)
	}
	if w1+w2+w3 != 114 {
		t.Errorf("total = %d, want 114", w1+w2+w3)
	}
}

func TestColumnWidths_ActiveAlbums(t *testing.T) {
	m := newTestBrowser(t)
	m.activeColumn = ColumnAlbums

	w1, w2, w3 := m.columnWidths()

	// active=albums → w1=28 (25%), w2=57 (50%), w3=114-28-57=29
	if w1 != 28 {
		t.Errorf("w1 = %d, want 28", w1)
	}
	if w2 != 57 {
		t.Errorf("w2 = %d, want 57", w2)
	}
	if w3 != 29 {
		t.Errorf("w3 = %d, want 29", w3)
	}
}

func TestColumnWidths_ActiveTracks(t *testing.T) {
	m := newTestBrowser(t)
	m.activeColumn = ColumnTracks

	w1, w2, w3 := m.columnWidths()

	// active=tracks → w1=28 (25%), w2=28 (25%), w3=114-28-28=58
	if w1 != 28 {
		t.Errorf("w1 = %d, want 28", w1)
	}
	if w2 != 28 {
		t.Errorf("w2 = %d, want 28", w2)
	}
	if w3 != 58 {
		t.Errorf("w3 = %d, want 58", w3)
	}
}

func TestColumnWidths_NarrowTerminal(t *testing.T) {
	m := newTestBrowser(t)
	m.width = 42 // available = 42 - 6 = 36 = 3 * 12 (exactly min)
	m.activeColumn = ColumnArtists

	w1, w2, w3 := m.columnWidths()

	// Exactly at minimum boundary, weighted split applies: 18, 9, 9
	if w1+w2+w3 != 36 {
		t.Errorf("total = %d, want 36", w1+w2+w3)
	}
}

func TestColumnWidths_VeryNarrow_EqualDistribution(t *testing.T) {
	m := newTestBrowser(t)
	m.width = 30 // available = 30 - 6 = 24 < 3 * 12 = 36

	w1, w2, w3 := m.columnWidths()

	// Equal distribution: 8, 8, 8
	if w1 != 8 {
		t.Errorf("w1 = %d, want 8", w1)
	}
	if w2 != 8 {
		t.Errorf("w2 = %d, want 8", w2)
	}
	if w3 != 8 {
		t.Errorf("w3 = %d, want 8", w3)
	}
}

// --- Column height ---

func TestColumnHeight(t *testing.T) {
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{"normal", 30, 25},
		{"small", 10, 5},
		{"minimum", 5, 1},
		{"very_small", 3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(nil)
			m.height = tt.height
			if got := m.columnHeight(); got != tt.want {
				t.Errorf("columnHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}

// --- Active column ---

func TestActiveColumn_Default(t *testing.T) {
	m := New(nil)
	if m.ActiveColumn() != ColumnArtists {
		t.Errorf("default active column = %d, want ColumnArtists", m.ActiveColumn())
	}
}

func TestSetActiveColumn(t *testing.T) {
	m := New(nil)
	m.SetActiveColumn(ColumnTracks)
	if m.ActiveColumn() != ColumnTracks {
		t.Errorf("ActiveColumn() = %d, want ColumnTracks", m.ActiveColumn())
	}
}

// --- Focus ---

func TestFocus(t *testing.T) {
	m := New(nil)

	if m.IsFocused() {
		t.Error("new model should not be focused")
	}

	m.SetFocused(true)
	if !m.IsFocused() {
		t.Error("SetFocused(true): IsFocused() = false, want true")
	}

	m.SetFocused(false)
	if m.IsFocused() {
		t.Error("SetFocused(false): IsFocused() = false, want false")
	}
}

// --- Cascading cursor resets ---

func TestCascadingReset_ArtistChange(t *testing.T) {
	m := newTestBrowser(t)

	// Navigate to Artist A, Album A2, track position 0
	m.SelectAlbum(albumA2)
	m.trackCursor.SetPos(0)

	// Verify we're on Artist A → Album A2
	if m.SelectedArtist() != artistA {
		t.Fatalf("setup: expected Artist A, got %q", m.SelectedArtist())
	}
	if m.SelectedAlbum().Name != albumA2 {
		t.Fatalf("setup: expected Album A2, got %q", m.SelectedAlbum().Name)
	}

	// Switch artist → album and track cursors must reset
	m.SelectArtist(artistB)

	if m.albumCursor.Pos() != 0 {
		t.Errorf("album cursor = %d, want 0", m.albumCursor.Pos())
	}
	if m.trackCursor.Pos() != 0 {
		t.Errorf("track cursor = %d, want 0", m.trackCursor.Pos())
	}
	if m.SelectedArtist() != artistB {
		t.Errorf("SelectedArtist() = %q, want %q", m.SelectedArtist(), artistB)
	}
	if m.SelectedAlbum() == nil || m.SelectedAlbum().Name != albumB1 {
		t.Errorf("SelectedAlbum() = %v, want Album B1", m.SelectedAlbum())
	}
}

func TestCascadingReset_AlbumChange(t *testing.T) {
	m := newTestBrowser(t)

	// Start on Artist A → Album A1 → Track A1-2
	m.trackCursor.SetPos(1)
	if m.SelectedTrack().Title != "Track A1-2" {
		t.Fatalf("setup: expected Track A1-2, got %q", m.SelectedTrack().Title)
	}

	// Switch album → track cursor must reset, but artist stays
	m.SelectAlbum(albumA2)

	if m.SelectedArtist() != artistA {
		t.Errorf("artist should not change: got %q", m.SelectedArtist())
	}
	if m.trackCursor.Pos() != 0 {
		t.Errorf("track cursor = %d, want 0", m.trackCursor.Pos())
	}
}

// --- State persistence round-trip ---

func TestStatePersistence_RoundTrip(t *testing.T) {
	m := newTestBrowser(t)

	// Navigate to specific state: Artist B, Album B1, track 2
	m.SelectArtist(artistB)
	m.SetActiveColumn(ColumnTracks)
	m.SelectTrackByID(m.tracks[1].ID)

	// Serialize state (same format as persistence.go)
	artist := m.SelectedArtistName()
	albumName := ""
	if album := m.SelectedAlbum(); album != nil {
		albumName = album.Name
	}
	trackID := ""
	if track := m.SelectedTrack(); track != nil {
		trackID = strconv.FormatInt(track.ID, 10)
	}
	col := strconv.Itoa(int(m.ActiveColumn()))
	savedState := col + "\x00" + artist + "\x00" + albumName + "\x00" + trackID

	// Create a fresh browser and restore state
	m2 := newTestBrowser(t)
	restoreBrowserSelection(&m2, savedState)

	if m2.SelectedArtist() != artistB {
		t.Errorf("restored artist = %q, want %q", m2.SelectedArtist(), artistB)
	}
	if m2.SelectedAlbum() == nil || m2.SelectedAlbum().Name != albumB1 {
		t.Errorf("restored album = %v, want Album B1", m2.SelectedAlbum())
	}
	if m2.ActiveColumn() != ColumnTracks {
		t.Errorf("restored column = %d, want ColumnTracks", m2.ActiveColumn())
	}
	if m2.SelectedTrack() == nil || m2.SelectedTrack().ID != m.SelectedTrack().ID {
		t.Errorf("restored track ID = %v, want %v", m2.SelectedTrack(), m.SelectedTrack())
	}
}

func TestStatePersistence_EmptyState(t *testing.T) {
	m := newTestBrowser(t)
	original := m.SelectedArtist()

	restoreBrowserSelection(&m, "")

	// Should not change anything
	if m.SelectedArtist() != original {
		t.Errorf("SelectedArtist() = %q, want %q (no change)", m.SelectedArtist(), original)
	}
}

func TestStatePersistence_PartialState(t *testing.T) {
	m := newTestBrowser(t)

	// Only column + artist (no album or track)
	state := strconv.Itoa(int(ColumnAlbums)) + "\x00" + artistC
	restoreBrowserSelection(&m, state)

	if m.SelectedArtist() != artistC {
		t.Errorf("restored artist = %q, want %q", m.SelectedArtist(), artistC)
	}
	if m.ActiveColumn() != ColumnAlbums {
		t.Errorf("restored column = %d, want ColumnAlbums", m.ActiveColumn())
	}
}

// --- Favorites ---

func TestSetFavorites(t *testing.T) {
	m := newTestBrowser(t)

	trackID := m.tracks[0].ID
	favs := map[int64]bool{trackID: true}
	m.SetFavorites(favs)

	if !m.favorites[trackID] {
		t.Errorf("track %d should be favorite", trackID)
	}
	if m.favorites[trackID+999] {
		t.Error("non-favorite track should not be marked")
	}
}

// --- Search items ---

func TestCurrentColumnSearchItems_Artists(t *testing.T) {
	m := newTestBrowser(t)
	m.activeColumn = ColumnArtists

	items := m.CurrentColumnSearchItems()

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].FilterValue() != artistA {
		t.Errorf("items[0] = %q, want %q", items[0].FilterValue(), artistA)
	}
}

func TestCurrentColumnSearchItems_Albums(t *testing.T) {
	m := newTestBrowser(t)
	m.activeColumn = ColumnAlbums

	items := m.CurrentColumnSearchItems()

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// Album A1 has year 2020
	if !strings.Contains(items[0].FilterValue(), albumA1) {
		t.Errorf("items[0] = %q, want to contain %q", items[0].FilterValue(), albumA1)
	}
}

func TestCurrentColumnSearchItems_Tracks(t *testing.T) {
	m := newTestBrowser(t)
	m.activeColumn = ColumnTracks

	items := m.CurrentColumnSearchItems()

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if !strings.Contains(items[0].FilterValue(), "Track A1-1") {
		t.Errorf("items[0] = %q, want to contain %q", items[0].FilterValue(), "Track A1-1")
	}
}

// --- restoreBrowserSelection helper (same logic as update_loading.go) ---

// restoreBrowserSelection restores a browser's selection from saved state.
// The state format is "column\x00artist\x00album\x00trackID".
func restoreBrowserSelection(browser *Model, savedState string) {
	if savedState == "" {
		return
	}
	parts := strings.SplitN(savedState, "\x00", 4)
	if len(parts) < 2 {
		return
	}
	if col, err := strconv.Atoi(parts[0]); err == nil {
		browser.SetActiveColumn(Column(col))
	}
	if parts[1] != "" {
		browser.SelectArtist(parts[1])
	}
	if len(parts) >= 3 && parts[2] != "" {
		browser.SelectAlbum(parts[2])
	}
	if len(parts) >= 4 && parts[3] != "" {
		if trackID, err := strconv.ParseInt(parts[3], 10, 64); err == nil {
			browser.SelectTrackByID(trackID)
		}
	}
}

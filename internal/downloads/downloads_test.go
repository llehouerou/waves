package downloads

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates a temporary SQLite database with the required schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use temp file to avoid in-memory database connection issues
	tmpFile := t.TempDir() + "/test.db"
	db, err := sql.Open("sqlite", tmpFile+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	statements := []string{
		`CREATE TABLE downloads (
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
		)`,
		`CREATE TABLE download_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			download_id INTEGER NOT NULL REFERENCES downloads(id) ON DELETE CASCADE,
			filename TEXT NOT NULL,
			size INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			bytes_read INTEGER NOT NULL DEFAULT 0,
			verified_on_disk INTEGER NOT NULL DEFAULT 0,
			UNIQUE(download_id, filename)
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("failed to create schema: %v (stmt: %s)", err, stmt[:50])
		}
	}

	return db
}

func TestDownloadProgress(t *testing.T) {
	tests := []struct {
		name          string
		files         []DownloadFile
		wantCompleted int
		wantTotal     int
		wantPercent   float64
	}{
		{
			name:          "empty files",
			files:         nil,
			wantCompleted: 0,
			wantTotal:     0,
			wantPercent:   0,
		},
		{
			name: "all pending",
			files: []DownloadFile{
				{Size: 1000, BytesRead: 0, Status: StatusPending},
				{Size: 1000, BytesRead: 0, Status: StatusPending},
			},
			wantCompleted: 0,
			wantTotal:     2,
			wantPercent:   0,
		},
		{
			name: "half completed",
			files: []DownloadFile{
				{Size: 1000, BytesRead: 1000, Status: StatusCompleted},
				{Size: 1000, BytesRead: 0, Status: StatusPending},
			},
			wantCompleted: 1,
			wantTotal:     2,
			wantPercent:   50,
		},
		{
			name: "all completed",
			files: []DownloadFile{
				{Size: 1000, BytesRead: 1000, Status: StatusCompleted},
				{Size: 2000, BytesRead: 2000, Status: StatusCompleted},
			},
			wantCompleted: 2,
			wantTotal:     2,
			wantPercent:   100,
		},
		{
			name: "in progress with bytes",
			files: []DownloadFile{
				{Size: 1000, BytesRead: 500, Status: StatusDownloading},
				{Size: 1000, BytesRead: 250, Status: StatusDownloading},
			},
			wantCompleted: 0,
			wantTotal:     2,
			wantPercent:   37.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Download{Files: tt.files}
			completed, total, percent := d.Progress()

			if completed != tt.wantCompleted {
				t.Errorf("completed = %d, want %d", completed, tt.wantCompleted)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
			if percent != tt.wantPercent {
				t.Errorf("percent = %f, want %f", percent, tt.wantPercent)
			}
		})
	}
}

func TestExtractFolderName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`@@user1\Music\Artist - Album`, "Artist - Album"},
		{`@@user\path\to\folder`, "folder"},
		{`\\some\windows\path`, "path"},
		{"/unix/style/path", "path"},
		{"simple", "simple"},
		{`@@user`, "user"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractFolderName(tt.input)
			if got != tt.want {
				t.Errorf("ExtractFolderName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildDiskPath(t *testing.T) {
	tests := []struct {
		completedPath string
		slskdDir      string
		want          string
	}{
		{"/downloads/complete", `@@user\Music\Album`, "/downloads/complete/Album"},
		{"/mnt/music", `\\share\Artist - Album`, "/mnt/music/Artist - Album"},
	}

	for _, tt := range tests {
		t.Run(tt.slskdDir, func(t *testing.T) {
			got := BuildDiskPath(tt.completedPath, tt.slskdDir)
			if got != tt.want {
				t.Errorf("BuildDiskPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTrackNumber(t *testing.T) {
	tests := []struct {
		filename string
		want     int
	}{
		{"01 - Song.flac", 1},
		{"1. Song.mp3", 1},
		{"01_Song.flac", 1},
		{"12 Track Name.mp3", 12},
		{`path\to\05 - Track.flac`, 5},
		{"Song Without Number.mp3", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := ParseTrackNumber(tt.filename)
			if got != tt.want {
				t.Errorf("ParseTrackNumber(%q) = %d, want %d", tt.filename, got, tt.want)
			}
		})
	}
}

func TestSortFilesByTrackNumber(t *testing.T) {
	files := []DownloadFile{
		{Filename: "03 - Third.mp3"},
		{Filename: "01 - First.mp3"},
		{Filename: "No Number.mp3"},
		{Filename: "02 - Second.mp3"},
		{Filename: "Another No Number.mp3"},
	}

	sorted := SortFilesByTrackNumber(files)

	// Expected order: 01, 02, 03, then alphabetically: "Another...", "No..."
	expected := []string{
		"01 - First.mp3",
		"02 - Second.mp3",
		"03 - Third.mp3",
		"Another No Number.mp3",
		"No Number.mp3",
	}

	if len(sorted) != len(expected) {
		t.Fatalf("len(sorted) = %d, want %d", len(sorted), len(expected))
	}

	for i, want := range expected {
		if sorted[i].Filename != want {
			t.Errorf("sorted[%d].Filename = %q, want %q", i, sorted[i].Filename, want)
		}
	}
}

func TestSortFilesByTrackNumber_DoesNotMutateOriginal(t *testing.T) {
	original := []DownloadFile{
		{Filename: "03 - Third.mp3"},
		{Filename: "01 - First.mp3"},
	}

	_ = SortFilesByTrackNumber(original)

	// Original should be unchanged
	if original[0].Filename != "03 - Third.mp3" {
		t.Error("original slice was mutated")
	}
}

func TestMapSlskdState(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		// Completed states
		{"Completed, Succeeded", StatusCompleted},
		{"Completed", StatusCompleted},
		{"Succeeded", StatusCompleted},

		// In progress states
		{"InProgress", StatusDownloading},
		{"Initializing", StatusDownloading},
		{"Requested", StatusDownloading},

		// Failed states
		{"Errored", StatusFailed},
		{"Cancelled", StatusFailed},
		{"TimedOut", StatusFailed},
		{"Rejected", StatusFailed},
		{"Aborted", StatusFailed},

		// Pending states
		{"Queued", StatusPending},
		{"Queued, Remotely", StatusPending},
		{"None", StatusPending},
		{"", StatusPending},

		// Unknown states
		{"SomeUnknownState", StatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := mapSlskdState(tt.state)
			if got != tt.want {
				t.Errorf("mapSlskdState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestManagerCreate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	m := New(db)

	download := Download{
		MBReleaseGroupID: "rg-123",
		MBArtistName:     "Test Artist",
		MBAlbumTitle:     "Test Album",
		MBReleaseYear:    "2024",
		SlskdUsername:    "user1",
		SlskdDirectory:   `@@user1\Music\Test Album`,
		Files: []DownloadFile{
			{Filename: "01 - Track 1.flac", Size: 1000},
			{Filename: "02 - Track 2.flac", Size: 2000},
		},
	}

	id, err := m.Create(download)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id <= 0 {
		t.Errorf("Create() returned invalid id = %d", id)
	}

	// Verify it was created
	got, err := m.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.MBReleaseGroupID != "rg-123" {
		t.Errorf("MBReleaseGroupID = %q, want %q", got.MBReleaseGroupID, "rg-123")
	}
	if got.MBArtistName != "Test Artist" {
		t.Errorf("MBArtistName = %q, want %q", got.MBArtistName, "Test Artist")
	}
	if len(got.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(got.Files))
	}
	if got.Status != StatusPending {
		t.Errorf("Status = %q, want %q", got.Status, StatusPending)
	}
}

func TestManagerList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	m := New(db)

	// Empty list
	downloads, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(downloads) != 0 {
		t.Errorf("expected 0 downloads, got %d", len(downloads))
	}

	// Create some downloads
	_, _ = m.Create(Download{
		MBReleaseGroupID: "rg-1",
		MBArtistName:     "Artist 1",
		MBAlbumTitle:     "Album 1",
		SlskdUsername:    "user1",
		SlskdDirectory:   "dir1",
	})
	_, _ = m.Create(Download{
		MBReleaseGroupID: "rg-2",
		MBArtistName:     "Artist 2",
		MBAlbumTitle:     "Album 2",
		SlskdUsername:    "user2",
		SlskdDirectory:   "dir2",
	})

	downloads, err = m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(downloads) != 2 {
		t.Errorf("expected 2 downloads, got %d", len(downloads))
	}
}

func TestManagerDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	m := New(db)

	id, err := m.Create(Download{
		MBReleaseGroupID: "rg-1",
		MBArtistName:     "Artist",
		MBAlbumTitle:     "Album",
		SlskdUsername:    "user",
		SlskdDirectory:   "dir",
		Files: []DownloadFile{
			{Filename: "track.flac", Size: 1000},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete
	if err := m.Delete(id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err = m.Get(id)
	if err == nil {
		t.Error("expected error getting deleted download")
	}

	// List should be empty
	downloads, _ := m.List()
	if len(downloads) != 0 {
		t.Errorf("expected 0 downloads after delete, got %d", len(downloads))
	}
}

func TestManagerDeleteCompleted(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	m := New(db)

	// Create pending and completed downloads
	_, _ = m.Create(Download{
		MBReleaseGroupID: "rg-pending",
		MBArtistName:     "Artist",
		MBAlbumTitle:     "Pending Album",
		SlskdUsername:    "user",
		SlskdDirectory:   "dir1",
	})

	id2, _ := m.Create(Download{
		MBReleaseGroupID: "rg-completed",
		MBArtistName:     "Artist",
		MBAlbumTitle:     "Completed Album",
		SlskdUsername:    "user",
		SlskdDirectory:   "dir2",
	})

	// Manually mark second as completed
	_, _ = db.Exec(`UPDATE downloads SET status = ? WHERE id = ?`, StatusCompleted, id2)

	// Delete completed
	if err := m.DeleteCompleted(); err != nil {
		t.Fatalf("DeleteCompleted() error = %v", err)
	}

	// Should only have 1 download left
	downloads, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("expected 1 download after DeleteCompleted, got %d", len(downloads))
	}
	if downloads[0].MBAlbumTitle != "Pending Album" {
		t.Errorf("wrong download remaining: %q", downloads[0].MBAlbumTitle)
	}
}

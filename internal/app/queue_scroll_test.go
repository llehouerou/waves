package app

import (
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/albumview"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// TestAlbumViewReplace_ResetsQueuePanelScroll reproduces the bug where
// playing an album from the album view after radio mode has scrolled the
// queue panel far down leaves the queue panel showing blank rows, because
// the path used by the album view (ClearQueue + AddTracks + PlayTrackAtIndex)
// never resets the queue panel cursor offset.
//
// Scenario:
//  1. Radio mode fills the queue with 50 tracks; playback advances to a
//     track far down the list, scrolling the queue panel offset to a
//     high value (cursor follows the playing track).
//  2. User opens the album view and presses Enter on a 3-track album.
//  3. After the action is processed, the queue contains only the album's
//     tracks (3) but the queue panel cursor offset still references a
//     position past the end of the new queue, so the rendered queue panel
//     shows EmptyLines instead of the album tracks.
//
// Expected: after the action runs, queue panel cursor offset is 0 and the
// album tracks are visible.
func TestAlbumViewReplace_ResetsQueuePanelScroll(t *testing.T) {
	db := openLibraryTestDB(t)
	insertSingleAlbum(t, db, "Album Artist", "Album Title", 3)
	lib := library.New(db)

	m := newIntegrationTestModel()
	m.Library = lib

	// Size the queue panel so listHeight is small and scrolling is needed.
	m.Layout.QueuePanel().SetSize(60, 12)

	// Simulate radio mode: queue has 50 tracks, currently playing index 45.
	radioTracks := make([]playback.Track, 50)
	for i := range radioTracks {
		radioTracks[i] = playback.Track{
			Path:   "/radio/" + strconv.Itoa(i) + ".mp3",
			Title:  "Radio " + strconv.Itoa(i),
			Artist: "Radio Artist",
		}
	}
	m.PlaybackService.AddTracks(radioTracks...)
	m.PlaybackService.QueueMoveTo(45)
	m.Layout.QueuePanel().SyncCursor()

	// Precondition: scrolled — the panel should NOT be showing "Radio 0" anymore.
	pre := stripANSI(m.Layout.QueuePanel().View())
	if strings.Contains(pre, "Radio 0 ") {
		t.Fatalf("precondition failed: queue panel not scrolled, view still shows first radio track:\n%s", pre)
	}

	// User plays album from the album view (Enter -> QueueAlbum{Replace: true}).
	msg := action.Msg{
		Source: "albumview",
		Action: albumview.QueueAlbum{
			AlbumArtist: "Album Artist",
			Album:       "Album Title",
			Replace:     true,
		},
	}
	m, _ = updateModel(t, m, msg)

	// Queue should now hold the album's 3 tracks.
	if got := m.PlaybackService.QueueLen(); got != 3 {
		t.Fatalf("queue length after replace: got %d, want 3", got)
	}

	// Bug: cursor offset is still scrolled high, so the queue panel's
	// renderTrackList reads idx = offset + i which is past the end of the
	// 3-track queue, producing blank rows.
	post := stripANSI(m.Layout.QueuePanel().View())
	if !strings.Contains(post, "Album Track 1") {
		t.Errorf("queue panel does not show album's first track after replace; view:\n%s", post)
	}
}

// openLibraryTestDB creates an in-memory SQLite DB with the library schema.
func openLibraryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

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
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func insertSingleAlbum(t *testing.T, db *sql.DB, albumArtist, album string, trackCount int) {
	t.Helper()
	for i := 1; i <= trackCount; i++ {
		_, err := db.Exec(`
			INSERT INTO library_tracks
				(path, mtime, artist, album_artist, album, title,
				 disc_number, track_number, year, added_at, updated_at)
			VALUES (?, 1000, ?, ?, ?, ?, 1, ?, 2024, 1000, 1000)
		`, "/lib/"+album+"/"+strconv.Itoa(i)+".mp3", albumArtist, albumArtist, album,
			"Album Track "+strconv.Itoa(i), i)
		if err != nil {
			t.Fatalf("insert track: %v", err)
		}
	}
}

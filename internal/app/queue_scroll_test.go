package app

import (
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/player"
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
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Album Artist", "Album Title", 3)
	})

	// Size the queue panel so listHeight is small and scrolling is needed.
	m.Layout.QueuePanel().SetSize(60, 12)

	// Simulate radio mode: queue has 50 tracks, currently playing index 45.
	radioTracks := make([]core.Track, 50)
	for i := range radioTracks {
		radioTracks[i] = core.Track{
			Path:   "/radio/" + strconv.Itoa(i) + ".mp3",
			Title:  "Radio " + strconv.Itoa(i),
			Artist: "Radio Artist",
		}
	}
	m.Core.Playback().AddTracks(radioTracks...)
	m.Core.Playback().QueueMoveTo(45)
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
	if got := m.Core.Playback().QueueLen(); got != 3 {
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

// When the last track of a full queue panel plays, the tracks radio appends
// after it must scroll into view.
func TestQueueChanged_ShowsTracksAddedAfterPlaying(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Layout.QueuePanel().SetSize(60, 12)

	track := func(i int) core.Track {
		return core.Track{Path: "/radio/" + strconv.Itoa(i) + ".mp3", Title: "Radio " + strconv.Itoa(i), Artist: "A"}
	}
	for i := range 20 {
		m.Core.Playback().AddTracks(track(i))
	}
	m.Core.Playback().QueueMoveTo(19)
	m.Layout.QueuePanel().SyncCursor()

	m.Core.Playback().AddTracks(track(20), track(21))
	m, _ = updateModel(t, m, ServiceQueueChangedMsg{})

	if view := stripANSI(m.Layout.QueuePanel().View()); !strings.Contains(view, "Radio 20 ") {
		t.Errorf("queue panel does not show the track added after the playing one; view:\n%s", view)
	}
}

// Starting playback on one of the last tracks the queue panel shows must keep
// it in view once the player bar shows up and shrinks the panel, even started
// from outside the TUI (MPRIS), with the cursor left on another track.
func TestPlaybackStart_KeepsCurrentTrackVisible(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.loadingState = loadingDone
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	for i := range 40 {
		m.Core.Playback().AddTracks(core.Track{Path: "/q/" + strconv.Itoa(i) + ".mp3", Title: "Track " + strconv.Itoa(i), Artist: "A"})
	}

	visible := 0
	for strings.Contains(stripANSI(m.Layout.QueuePanel().View()), "Track "+strconv.Itoa(visible)+" ") {
		visible++
	}
	current := visible - 3 // the last row the cursor reaches without scrolling
	m.Core.Playback().QueueMoveTo(current)

	testPlayer(m.Core).SetState(player.Playing)
	m, _ = updateModel(t, m, ServiceStateChangedMsg{Previous: int(core.StateStopped), Current: int(core.StatePlaying)})

	if view := stripANSI(m.Layout.QueuePanel().View()); !strings.Contains(view, "Track "+strconv.Itoa(current)+" ") {
		t.Errorf("queue panel hides the current track %d once playback starts; view:\n%s", current, view)
	}
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

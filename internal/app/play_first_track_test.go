package app

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/ui/testutil"
)

// newQueuedAlbumTestModel returns a model whose library holds one album of
// three tracks, queued with `a` on the browser's artist: the queue has its
// tracks and no current one. The queue panel is shown, its cursor on the last
// track, and focused.
func newQueuedAlbumTestModel(t *testing.T) Model {
	t.Helper()
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Album Artist", "Album Title", 3)
	})
	m, _ = updateModel(t, m, keyMsg("a"))
	if n, at := m.Core.Playback().QueueLen(), m.Core.Playback().QueueCurrentIndex(); n != 3 || at != -1 {
		t.Fatalf("after a: %d tracks queued at %d, want 3 at -1", n, at)
	}
	m.Layout.ShowQueue()
	m, _ = updateModel(t, m, keyMsg("tab"))
	m, _ = updateModel(t, m, keyMsg("j"))
	m, _ = updateModel(t, m, keyMsg("j"))
	return m
}

// expectFirstTrackStarted fails the test unless space started the first track
// of the queue with no popup, and, once the core's events are fed to the
// model, the queue panel shows it playing with its cursor on it.
func expectFirstTrackStarted(t *testing.T, m Model, wantPlays []string) {
	t.Helper()
	if got := testPlayer(m.Core).PlayCalls(); !slices.Equal(got, wantPlays) {
		t.Fatalf("tracks opened = %v, want %v", got, wantPlays)
	}
	if got := m.Popups.ErrorMsg(); got != "" {
		t.Errorf("error popup %q, want none", got)
	}
	if got := m.Core.Playback().QueueCurrentIndex(); got != 0 {
		t.Errorf("queue position = %d, want 0", got)
	}

	m = pumpUntil[ServiceTrackChangedMsg](t, m)
	m, _ = updateModel(t, m, keyMsg("x")) // selects the track under the cursor

	view := testutil.StripANSI(m.Layout.QueuePanel().View())
	first := testutil.FindLine(view, "Album Track 1")
	if !strings.Contains(first, "▶") || !strings.Contains(first, "●") {
		t.Errorf("first track's line %q, want it playing and under the cursor; view:\n%s", first, view)
	}
}

// Space on a queue with tracks and no current track starts its first track.
func TestSpace_OnAQueueWithNoCurrentTrack_StartsTheFirstTrack(t *testing.T) {
	m := newQueuedAlbumTestModel(t)

	m, _ = updateModel(t, m, keyMsg(" "))

	expectFirstTrackStarted(t, m, []string{"/lib/Album Title/1.mp3"})
}

// Space after the current track was removed and playback stopped starts the
// track now first in the queue.
func TestSpace_AfterTheCurrentTrackWasRemoved_StartsTheFirstTrack(t *testing.T) {
	m := newQueuedAlbumTestModel(t)
	pb := m.Core.Playback()
	pb.QueueMoveTo(1)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	pb.RemoveTracks([]int{1})
	if err := pb.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	m, _ = updateModel(t, m, keyMsg(" "))

	expectFirstTrackStarted(t, m, []string{"/lib/Album Title/2.mp3", "/lib/Album Title/1.mp3"})
}

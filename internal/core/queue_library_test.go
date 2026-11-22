package core

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// queueTrust queues the album of newRetagCore as the Library view does, and
// returns its entries.
func queueTrust(t *testing.T, c *Core) []QueueEntry {
	t.Helper()
	tracks, err := c.Library().Tracks("low", "trust")
	if err != nil || len(tracks) != 3 {
		t.Fatalf("tracks of trust = %+v (err %v), want three", tracks, err)
	}
	c.Playback().AddTracks(TracksFromLibrary(tracks)...)
	return c.Playback().QueueEntries()
}

// names is how the queue names each of its tracks.
func names(tracks []Track) []string {
	shown := make([]string, len(tracks))
	for i, t := range tracks {
		shown[i] = t.Title + "|" + t.Artist + "|" + t.Album
	}
	return shown
}

var (
	oldTrust = []string{"amazing grace|low|trust", "canada|low|trust", "candy girl|low|trust"}
	newTrust = []string{"Amazing Grace|Low|Trust", "Canada|Low|Trust", "Candy Girl|Low|Trust"}
)

// expectNames fails the test unless the tracks are named want, in order.
func expectNames(t *testing.T, what string, tracks []Track, want []string) {
	t.Helper()
	if got := names(tracks); !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// retagTrust retags the album of newRetagCore and waits for its end.
func retagTrust(t *testing.T, c *Core) {
	t.Helper()
	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	if finished := nextEvent[RetagFinished](t, c); finished.Err != nil || !AllDone(finished.Files) {
		t.Fatalf("RetagFinished = %+v, want every file written", finished)
	}
}

// Once a client hears that the retag ended, the queue names the tracks of the
// album as the library does, each entry where it was.
func TestRetag_TheQueueNamesTheTracksAsTheLibraryNowDoes(t *testing.T) {
	c, _, _ := newRetagCore(t)
	before := queueTrust(t, c)
	c.Playback().QueueMoveTo(1)

	retagTrust(t, c)

	after := c.Playback().QueueEntries()
	tracks := c.Playback().QueueTracks()
	expectNames(t, "the queue", tracks, newTrust)
	for i := range after {
		if i < len(before) && after[i].ID != before[i].ID {
			t.Errorf("entry %d is %d after the retag, want %d: the same entry", i, after[i].ID, before[i].ID)
		}
	}
	if got := c.Playback().QueueCurrentIndex(); got != 1 {
		t.Errorf("queue position = %d after the retag, want 1", got)
	}
}

// eventsUntil returns the events the core emits up to the first E, and those
// it emits in the quiet period after it.
func eventsUntil[E Event](t *testing.T, c *Core) []Event {
	t.Helper()
	var events []Event
	for {
		e := nextEvent[Event](t, c)
		events = append(events, e)
		if _, ok := e.(E); ok {
			return append(events, eventsDuring(c)...)
		}
	}
}

// queueChanges returns the QueueChanged among events.
func queueChanges(events []Event) []QueueChanged {
	var changes []QueueChanged
	for _, e := range events {
		if q, ok := e.(QueueChanged); ok {
			changes = append(changes, q)
		}
	}
	return changes
}

// retagTrustEvents retags the album of newRetagCore, and returns the events up
// to its end and in the quiet period after it.
func retagTrustEvents(t *testing.T, c *Core) []Event {
	t.Helper()
	if err := c.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	return eventsUntil[RetagFinished](t, c)
}

// retagFile writes the tags on the file at path, as a tagger outside waves
// would, with a time a scan sees as a change.
func retagFile(t *testing.T, path string, tags FileTags) {
	t.Helper()
	tagstest.WriteMP3(t, path, tags)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// refresh scans the library and returns the events up to the end of the scan
// and in the quiet period after it.
func refresh(t *testing.T, c *Core) []Event {
	t.Helper()
	if err := c.Library().Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	return eventsUntil[ScanFinished](t, c)
}

// The queue is told once, with the names the retag wrote.
func TestRetag_ReportsTheQueueUnderItsNewNames(t *testing.T) {
	c, _, _ := newRetagCore(t)
	queueTrust(t, c)
	nextEvent[QueueChanged](t, c)

	changes := queueChanges(retagTrustEvents(t, c))

	if len(changes) != 1 {
		t.Fatalf("%d QueueChanged, want one", len(changes))
	}
	expectNames(t, "QueueChanged", changes[0].Tracks, newTrust)
}

// A retag that changes no queued track says nothing of the queue.
func TestRetag_OfAnAlbumTheQueueDoesNotHoldLeavesTheQueueUntold(t *testing.T) {
	c, _, _ := newRetagCore(t)
	c.Playback().AddTracks(Track{Path: "/music/another/album/track.mp3", Title: "Track"})
	nextEvent[QueueChanged](t, c)

	if changes := queueChanges(retagTrustEvents(t, c)); len(changes) != 0 {
		t.Errorf("QueueChanged %+v, want none", changes)
	}
}

func TestScan_ThatChangesNoQueuedTrackLeavesTheQueueUntold(t *testing.T) {
	c, _, _ := newRetagCore(t)
	queueTrust(t, c)
	nextEvent[QueueChanged](t, c)

	if changes := queueChanges(refresh(t, c)); len(changes) != 0 {
		t.Errorf("QueueChanged %+v, want none", changes)
	}
}

// A track whose file got new tags outside waves is named as the scan read it.
func TestScan_TheQueueNamesATrackWhoseTagsChangedAsTheScanReadThem(t *testing.T) {
	c, _, paths := newRetagCore(t)
	queueTrust(t, c)
	retagFile(t, paths[1], FileTags{Artist: "Low", AlbumArtist: "low", Album: "Trust", Title: "Canada", TrackNumber: 2})

	changes := queueChanges(refresh(t, c))

	want := []string{oldTrust[0], newTrust[1], oldTrust[2]}
	expectNames(t, "the queue", c.Playback().QueueTracks(), want)
	if len(changes) == 0 {
		t.Fatal("no QueueChanged")
	}
	expectNames(t, "the last QueueChanged", changes[len(changes)-1].Tracks, want)
}

// The track playing keeps playing, with nothing that says it started again,
// under the names the scan read.
func TestScan_LeavesTheTrackPlayingPlayingUnderItsNewNames(t *testing.T) {
	c, mock, paths := newRetagCore(t)
	other := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(paths[0]))), "nico", "chelsea girl", "these days.mp3")
	tagstest.WriteMP3(t, other, FileTags{Artist: "nico", AlbumArtist: "nico", Album: "chelsea girl", Title: "these days", TrackNumber: 1})
	if _, err := c.library.lib.AddTracks([]string{other}); err != nil {
		t.Fatalf("add track: %v", err)
	}
	tracks, err := c.Library().Tracks("nico", "chelsea girl")
	if err != nil || len(tracks) != 1 {
		t.Fatalf("tracks of chelsea girl = %+v (err %v), want one", tracks, err)
	}
	queueTrust(t, c)
	c.Playback().AddTracks(TracksFromLibrary(tracks)...)
	if err := c.Playback().JumpTo(3); err != nil {
		t.Fatalf("jump: %v", err)
	}
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	eventsDuring(c)
	starts := len(mock.PlayCalls())
	retagFile(t, other, FileTags{Artist: "Nico", AlbumArtist: "Nico", Album: "Chelsea Girl", Title: "These Days", TrackNumber: 1})

	events := refresh(t, c)

	if mock.State() != player.Playing {
		t.Errorf("player state = %v after the scan, want still playing", mock.State())
	}
	if got := mock.PlayCalls(); len(got) != starts {
		t.Errorf("tracks opened = %v, want nothing opened by the scan", got)
	}
	if n := count[TrackChanged](events); n != 0 {
		t.Errorf("%d TrackChanged, want none", n)
	}
	current := c.Playback().CurrentTrack()
	if current == nil {
		t.Fatal("no current track after the scan")
	}
	expectNames(t, "the current track", []Track{*current}, []string{"These Days|Nico|Chelsea Girl"})
	if got := c.Playback().QueueCurrentIndex(); got != 3 {
		t.Errorf("queue position = %d, want 3", got)
	}
}

// The refresh is no queue edit: one undo takes back the tracks the user
// added, and the redo brings them back under their new names.
func TestRetag_AddsNoUndoStepAndUndoNeverBringsTheOldNamesBack(t *testing.T) {
	t.Run("added then retagged", func(t *testing.T) {
		c, _, _ := newRetagCore(t)
		queueTrust(t, c)
		retagTrust(t, c)

		if !c.Playback().Undo() {
			t.Fatal("nothing to undo")
		}
		if got := c.Playback().QueueTracks(); len(got) != 0 {
			t.Errorf("queue after one undo = %v, want empty", names(got))
		}
		if !c.Playback().Redo() {
			t.Fatal("nothing to redo")
		}
		expectNames(t, "the queue after the redo", c.Playback().QueueTracks(), newTrust)
	})

	// Tracks the queue no longer holds, but a list undo or redo walks to does.
	t.Run("removed then retagged, then undone", func(t *testing.T) {
		c, _, _ := newRetagCore(t)
		queueTrust(t, c)
		c.Playback().RemoveTracks([]int{0})
		retagTrust(t, c)

		if !c.Playback().Undo() {
			t.Fatal("nothing to undo")
		}
		expectNames(t, "the queue after the undo", c.Playback().QueueTracks(), newTrust)
	})

	t.Run("undone then retagged, then redone", func(t *testing.T) {
		c, _, _ := newRetagCore(t)
		queueTrust(t, c)
		c.Playback().Undo()
		retagTrust(t, c)

		if !c.Playback().Redo() {
			t.Fatal("nothing to redo")
		}
		expectNames(t, "the queue after the redo", c.Playback().QueueTracks(), newTrust)
	})

	t.Run("edits before and after the retag", func(t *testing.T) {
		c, _, _ := newRetagCore(t)
		tracks, err := c.Library().Tracks("low", "trust")
		if err != nil {
			t.Fatalf("tracks: %v", err)
		}
		pb := c.Playback()
		pb.AddTracks(TracksFromLibrary(tracks[:1])...)
		pb.AddTracks(TracksFromLibrary(tracks[1:])...)
		retagTrust(t, c)
		pb.RemoveTracks([]int{0})

		steps := []struct {
			name string
			walk func() bool
			want []string
		}{
			{"undo the remove", pb.Undo, newTrust},
			{"undo the second add", pb.Undo, newTrust[:1]},
			{"redo the second add", pb.Redo, newTrust},
			{"redo the remove", pb.Redo, newTrust[1:]},
		}
		for _, step := range steps {
			if !step.walk() {
				t.Fatalf("%s: nothing to walk to", step.name)
			}
			expectNames(t, "the queue after "+step.name, pb.QueueTracks(), step.want)
		}
	})
}

func TestRetag_TheSavedQueueHoldsTheNewNames(t *testing.T) {
	c, _, _ := newRetagCore(t)
	queueTrust(t, c)

	retagTrust(t, c)

	eventually(t, "the saved queue holds the new names", func() bool {
		saved, err := c.state.GetQueue()
		if err != nil || saved == nil || len(saved.Tracks) != 3 {
			return false
		}
		got := make([]Track, len(saved.Tracks))
		for i, s := range saved.Tracks {
			got[i] = Track{Title: s.Title, Artist: s.Artist, Album: s.Album}
		}
		return slices.Equal(names(got), newTrust)
	})
}

func TestScan_TheSavedQueueHoldsTheNewNames(t *testing.T) {
	c, _, paths := newRetagCore(t)
	queueTrust(t, c)
	retagFile(t, paths[0], FileTags{Artist: "Low", AlbumArtist: "low", Album: "Trust", Title: "Amazing Grace", TrackNumber: 1})

	refresh(t, c)

	eventually(t, "the saved queue holds the new names", func() bool {
		saved, err := c.state.GetQueue()
		return err == nil && saved != nil && len(saved.Tracks) == 3 &&
			saved.Tracks[0].Title == "Amazing Grace" && saved.Tracks[0].Artist == "Low" && saved.Tracks[0].Album == "Trust"
	})
}

// A track queued from the Files view, on a file of the album, and one the
// library no longer has, stay as they were queued, and that is no failure.
func TestRetag_LeavesTheTracksTheLibraryDoesNotHaveAsTheyAre(t *testing.T) {
	c, _, paths := newRetagCore(t)
	fromFiles := Track{Path: paths[1], Title: "canada", Artist: "low", Album: "trust", TrackNumber: 2}
	gone := Track{ID: 999, Path: "/music/gone.mp3", Title: "gone", Artist: "nobody", Album: "nowhere"}
	c.Playback().AddTracks(fromFiles, gone)

	events := retagTrustEvents(t, c)

	if got := c.Playback().QueueTracks(); len(got) != 2 || got[0] != fromFiles || got[1] != gone {
		t.Errorf("queue = %+v, want %+v unchanged", got, []Track{fromFiles, gone})
	}
	if n := count[Failed](events); n != 0 {
		t.Errorf("%d Failed, want none", n)
	}
}

// A library row that cannot be read leaves its entry as it was, and is told
// once.
func TestScan_ATrackTheLibraryCannotReadStaysAsItWasAndIsTold(t *testing.T) {
	c, _, paths := newRetagCore(t)
	queueTrust(t, c)
	c.Playback().AddTracks(c.Playback().QueueTracks()[1]) // twice in the queue: one failure all the same
	spoilRow(t, c, paths[1])

	events := refresh(t, c)

	var failed []Failed
	for _, e := range events {
		if f, ok := e.(Failed); ok {
			failed = append(failed, f)
		}
	}
	if len(failed) != 1 || failed[0].Op != errmsg.OpQueueRefresh || !strings.Contains(failed[0].Err.Error(), "1 track could not be read") {
		t.Errorf("Failed = %+v, want one failure to refresh the queue, for one track", failed)
	}
	expectNames(t, "the queue", c.Playback().QueueTracks(), append(slices.Clone(oldTrust), oldTrust[1]))
}

// A track of the album started after the retag starts under its new names:
// the desktop is told them.
func TestRetag_ATrackStartedAfterItIsToldUnderItsNewNames(t *testing.T) {
	desktop := &fakeNotifier{}
	source := t.TempDir()
	c, _, _ := newRetagCoreOn(t, source, Options{LibrarySources: []string{source}, Notifications: notificationsOn(), notifier: desktop})
	queueTrust(t, c)
	retagTrust(t, c)

	if err := c.Playback().JumpTo(1); err != nil {
		t.Fatalf("jump: %v", err)
	}
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}

	told := desktop.wait(t, 1)
	if told[0].Title != "Canada" || told[0].Body != "Low · Trust" {
		t.Errorf("the desktop was told %q / %q, want Canada / Low · Trust", told[0].Title, told[0].Body)
	}
}

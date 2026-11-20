package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/state"
)

// savedQueue reads what the core wrote to its state.
func savedQueue(t *testing.T, mgr *state.Manager) *state.QueueState {
	t.Helper()
	saved, err := mgr.GetQueue()
	if err != nil {
		t.Fatalf("get queue: %v", err)
	}
	return saved
}

func TestPersistence_QueueEditsAreSavedAsTheyHappen(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	mgr := c.state
	pb := c.Playback()

	pb.AddTracks(Track{Path: "/music/a.flac", Title: "A"}, Track{Path: "/music/b.flac", Title: "B"})
	eventually(t, "the two tracks are saved", func() bool { return len(savedQueue(t, mgr).Tracks) == 2 })

	pb.RemoveTracks([]int{0})
	eventually(t, "the removal is saved", func() bool {
		saved := savedQueue(t, mgr)
		return len(saved.Tracks) == 1 && saved.Tracks[0].Title == "B"
	})

	pb.SetShuffle(true)
	eventually(t, "shuffle is saved", func() bool { return savedQueue(t, mgr).Shuffle })

	pb.QueueMoveTo(0)
	eventually(t, "the position is saved", func() bool { return savedQueue(t, mgr).CurrentIndex == 0 })
}

// Next onto a track that fails to open still moves the queue: the client is
// told where it is, and the position is saved as it happens, not only at Close.
func TestPersistence_NextOntoATrackThatFailsToOpenIsToldAndSaved(t *testing.T) {
	c, mock := newTestCore(t, Options{})
	mgr := c.state
	pb := c.Playback()
	pb.AddTracks(Track{Path: "/music/a.flac", Title: "A"}, Track{Path: "/music/b.flac", Title: "B"})
	pb.QueueMoveTo(0)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	eventually(t, "the position is saved", func() bool { return savedQueue(t, mgr).CurrentIndex == 0 })

	mock.SetPlayError(errors.New("unreadable"))
	if err := pb.Next(); err == nil {
		t.Fatal("next succeeded, want the open failure")
	}

	if e := nextEvent[TrackChanged](t, c); e.Index != 1 || e.Current == nil || e.Current.Title != "B" {
		t.Errorf("TrackChanged = %+v, want track B at index 1", e)
	}
	eventually(t, "the new position is saved", func() bool { return savedQueue(t, mgr).CurrentIndex == 1 })
}

func TestPersistence_QueueSurvivesARestart(t *testing.T) {
	seeded := openTestState(t)
	addLibraryTracks(t, seeded.DB(), "Low", 1)
	lib := library.New(seeded.DB())
	inLibrary, err := lib.TrackByPath("/lib/Low/1.mp3")
	if err != nil {
		t.Fatalf("library track: %v", err)
	}
	database := seeded.closed(t)

	first, _ := newTestCore(t, Options{Database: database})
	first.Playback().AddTracks(
		Track{Path: "/music/a.flac", Title: "A", Artist: "Someone", Album: "Something", TrackNumber: 3},
		Track{ID: inLibrary.ID, Path: inLibrary.Path, Title: inLibrary.Title},
	)
	first.Playback().QueueMoveTo(1)
	first.Playback().SetRepeatMode(RepeatAll)
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, mock := newTestCore(t, Options{Database: database})
	pb := second.Playback()

	tracks := pb.QueueTracks()
	if len(tracks) != 2 {
		t.Fatalf("restored %d tracks, want 2", len(tracks))
	}
	if a := tracks[0]; a.Path != "/music/a.flac" || a.Title != "A" || a.Artist != "Someone" || a.Album != "Something" || a.TrackNumber != 3 {
		t.Errorf("first track = %+v, want what was queued", a)
	}
	// A library track comes back with what the library knows about it.
	if b := tracks[1]; b.ID != inLibrary.ID || b.Artist != "Low" || b.Year != 2024 {
		t.Errorf("library track = %+v, want Low's track from 2024", b)
	}
	if pb.QueueCurrentIndex() != 1 {
		t.Errorf("index = %d, want 1", pb.QueueCurrentIndex())
	}
	if pb.RepeatMode() != RepeatAll {
		t.Errorf("repeat = %v, want All", pb.RepeatMode())
	}
	// Restoring is not playing, and not a queue edit.
	if !pb.IsStopped() || len(mock.PlayCalls()) != 0 {
		t.Error("restoring the queue started playback")
	}
	if pb.Undo() {
		t.Error("restoring the queue left an undo step")
	}
}

// A queue saved with tracks and no current track comes back that way, and
// Play starts its first track: the new position is saved.
func TestPersistence_QueueWithNoCurrentTrackStartsItsFirstTrackAfterARestart(t *testing.T) {
	database := filepath.Join(t.TempDir(), "waves.db")
	first, _ := newTestCore(t, Options{Database: database})
	first.Playback().AddTracks(Track{Path: "/music/a.flac"}, Track{Path: "/music/b.flac"})
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, mock := newTestCore(t, Options{Database: database})
	pb := second.Playback()
	if pb.QueueCurrentIndex() != -1 {
		t.Fatalf("restored index = %d, want -1", pb.QueueCurrentIndex())
	}

	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	if calls := mock.PlayCalls(); len(calls) != 1 || calls[0] != "/music/a.flac" {
		t.Errorf("PlayCalls = %v, want [/music/a.flac]", calls)
	}
	eventually(t, "the position is saved", func() bool { return savedQueue(t, second.state).CurrentIndex == 0 })
}

func TestPersistence_VolumeSurvivesARestart(t *testing.T) {
	database := filepath.Join(t.TempDir(), "waves.db")
	first, mock := newTestCore(t, Options{Database: database})
	pb := first.Playback()

	for range 3 {
		pb.AdjustVolume(-0.10)
	}
	if got := pb.Volume(); got != 0.7 {
		t.Fatalf("volume = %v, want 0.7", got)
	}
	if mock.Volume() != 0.7 {
		t.Errorf("player volume = %v, want 0.7", mock.Volume())
	}
	pb.ToggleMute()
	if !pb.Muted() {
		t.Fatal("not muted after ToggleMute")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, mock := newTestCore(t, Options{Database: database})
	if got := second.Playback().Volume(); got != 0.7 || !second.Playback().Muted() {
		t.Errorf("restored volume = %v muted = %v, want 0.7 muted", got, second.Playback().Muted())
	}
	if mock.Volume() != 0.7 || !mock.Muted() {
		t.Errorf("player volume = %v muted = %v, want 0.7 muted", mock.Volume(), mock.Muted())
	}
}

func TestPersistence_VolumeIsSavedAsItChanges(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	mgr := c.state

	c.Playback().AdjustVolume(-0.10)

	eventually(t, "the volume is saved", func() bool {
		saved, err := mgr.GetVolume()
		if err != nil {
			t.Fatalf("get volume: %v", err)
		}
		return saved.Volume == 0.9
	})
}

func TestAdjustVolume_Unmutes(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	pb := c.Playback()
	pb.ToggleMute()

	pb.AdjustVolume(-0.10)

	if pb.Muted() {
		t.Error("still muted after a volume change")
	}
	if got := pb.Volume(); got != 0.9 {
		t.Errorf("volume = %v, want 0.9", got)
	}
}

func TestPersistence_FailuresAreReportedToTheClient(t *testing.T) {
	t.Run("loading", func(t *testing.T) {
		seeded := openTestState(t)
		if _, err := seeded.DB().Exec(`INSERT INTO queue_state (id, current_index) VALUES (1, 'unreadable')`); err != nil {
			t.Fatalf("seed queue: %v", err)
		}

		c, _ := newTestCore(t, Options{Database: seeded.closed(t)})

		if e := waitFailed(t, c, errmsg.OpQueueLoad); e.Err == nil {
			t.Errorf("Failed = %+v, want the load error", e)
		}
	})

	t.Run("saving", func(t *testing.T) {
		c, _ := newTestCore(t, Options{})
		if _, err := c.state.DB().Exec(`DROP TABLE queue_tracks`); err != nil {
			t.Fatalf("drop queue: %v", err)
		}

		c.Playback().AddTracks(Track{Path: "/music/a.flac"})

		if e := waitFailed(t, c, errmsg.OpQueueSave); e.Err == nil {
			t.Errorf("Failed = %+v, want the save error", e)
		}
	})
}

// seedQueueOfLow seeds a database whose library holds three tracks by Low,
// those numbered in spoiled with a row TrackByID cannot read, and whose saved
// queue is those three tracks then a track the library no longer has. It returns the
// path of the database and the saved queue.
func seedQueueOfLow(t *testing.T, spoiled ...int) (string, []state.QueueTrack) {
	t.Helper()
	seeded := openTestState(t)
	addLibraryTracks(t, seeded.DB(), "Low", 3)
	lib := library.New(seeded.DB())
	var saved []state.QueueTrack
	for n := 1; n <= 3; n++ {
		lt, err := lib.TrackByPath(libraryPath("Low", n))
		if err != nil {
			t.Fatalf("library track: %v", err)
		}
		saved = append(saved, state.QueueTrack{
			TrackID: lt.ID, Path: lt.Path, Title: "Saved " + lt.Title, Artist: "Low", Album: "Album", TrackNumber: n,
		})
	}
	saved = append(saved, state.QueueTrack{TrackID: 999, Path: "/lib/gone.mp3", Title: "Gone", TrackNumber: 4})
	for _, n := range spoiled {
		if _, err := seeded.DB().Exec(`UPDATE library_tracks SET year = 'not a year' WHERE path = ?`, libraryPath("Low", n)); err != nil {
			t.Fatalf("spoil row: %v", err)
		}
	}
	if err := seeded.SaveQueue(state.QueueState{CurrentIndex: 0, Tracks: saved}); err != nil {
		t.Fatalf("save queue: %v", err)
	}
	return seeded.closed(t), saved
}

// queueLoadFailures are the Failed events about loading the queue the core
// emits within the quiet period.
func queueLoadFailures(c *Core) []Failed {
	var failed []Failed
	for _, e := range eventsDuring(c) {
		if f, ok := e.(Failed); ok && f.Op == errmsg.OpQueueLoad {
			failed = append(failed, f)
		}
	}
	return failed
}

func TestPersistence_ATrackWhoseLibraryRowCannotBeReadIsRestoredAsSavedAndTold(t *testing.T) {
	database, saved := seedQueueOfLow(t, 2, 3)

	c, _ := newTestCore(t, Options{Database: database})

	tracks := c.Playback().QueueTracks()
	if len(tracks) != 4 {
		t.Fatalf("restored %d tracks, want 4", len(tracks))
	}
	if first := tracks[0]; first.Title != "Low 1" || first.Year != 2024 || first.DiscNumber != 1 {
		t.Errorf("readable track = %+v, want what the library knows", first)
	}
	for i := 1; i < 4; i++ {
		s, got := saved[i], tracks[i]
		if got.ID != s.TrackID || got.Path != s.Path || got.Title != s.Title || got.Artist != s.Artist ||
			got.Album != s.Album || got.TrackNumber != s.TrackNumber {
			t.Errorf("track %d = %+v, want it as saved %+v", i, got, s)
		}
	}
	failed := queueLoadFailures(c)
	if len(failed) != 1 {
		t.Fatalf("Failed events about the queue = %+v, want exactly one", failed)
	}
	if err := failed[0].Err; err == nil || !strings.Contains(err.Error(), "2 tracks") ||
		!strings.Contains(err.Error(), `name "year"`) {
		t.Errorf("Failed error = %v, want the two tracks not read and the first error", err)
	}
}

func TestPersistence_OneTrackItCannotReadIsToldOnce(t *testing.T) {
	database, saved := seedQueueOfLow(t, 2)

	c, _ := newTestCore(t, Options{Database: database})

	tracks := c.Playback().QueueTracks()
	if len(tracks) != 4 || tracks[1].Title != saved[1].Title || tracks[2].Title != "Low 3" {
		t.Fatalf("restored %+v, want track 2 as saved and track 3 as the library knows it", tracks)
	}
	failed := queueLoadFailures(c)
	if len(failed) != 1 {
		t.Fatalf("Failed events about the queue = %+v, want exactly one", failed)
	}
	if err := failed[0].Err; err == nil || !strings.Contains(err.Error(), "1 track could not be read") {
		t.Errorf("Failed error = %v, want the one track not read", err)
	}
}

func TestPersistence_ATrackTheLibraryNoLongerHasIsRestoredAsSavedUntold(t *testing.T) {
	database, saved := seedQueueOfLow(t)

	c, _ := newTestCore(t, Options{Database: database})

	tracks := c.Playback().QueueTracks()
	if len(tracks) != 4 || tracks[3].ID != saved[3].TrackID || tracks[3].Title != "Gone" {
		t.Fatalf("restored %+v, want the track gone as saved last", tracks)
	}
	if failed := queueLoadFailures(c); len(failed) != 0 {
		t.Errorf("Failed events about the queue = %+v, want none", failed)
	}
}

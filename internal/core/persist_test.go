package core

import (
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
	mgr := openTestState(t)
	c, _ := newTestCore(t, Options{State: mgr})
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

func TestPersistence_QueueSurvivesARestart(t *testing.T) {
	mgr := openTestState(t)
	addLibraryTracks(t, mgr, "Low", 1)
	lib := library.New(mgr.DB())
	inLibrary, err := lib.TrackByPath("/lib/Low/1.mp3")
	if err != nil {
		t.Fatalf("library track: %v", err)
	}

	first, _ := newTestCore(t, Options{State: mgr})
	first.Playback().AddTracks(
		Track{Path: "/music/a.flac", Title: "A", Artist: "Someone", Album: "Something", TrackNumber: 3},
		Track{ID: inLibrary.ID, Path: inLibrary.Path, Title: inLibrary.Title},
	)
	first.Playback().QueueMoveTo(1)
	first.Playback().SetRepeatMode(RepeatAll)
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, mock := newTestCore(t, Options{State: mgr})
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

func TestPersistence_VolumeSurvivesARestart(t *testing.T) {
	mgr := openTestState(t)
	first, mock := newTestCore(t, Options{State: mgr})
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

	second, mock := newTestCore(t, Options{State: mgr})
	if got := second.Playback().Volume(); got != 0.7 || !second.Playback().Muted() {
		t.Errorf("restored volume = %v muted = %v, want 0.7 muted", got, second.Playback().Muted())
	}
	if mock.Volume() != 0.7 || !mock.Muted() {
		t.Errorf("player volume = %v muted = %v, want 0.7 muted", mock.Volume(), mock.Muted())
	}
}

func TestPersistence_VolumeIsSavedAsItChanges(t *testing.T) {
	mgr := openTestState(t)
	c, _ := newTestCore(t, Options{State: mgr})

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
		mgr := openTestState(t)
		mgr.Close()

		c, _ := newTestCore(t, Options{State: mgr})

		if e := waitFailed(t, c, errmsg.OpQueueLoad); e.Err == nil {
			t.Errorf("Failed = %+v, want the load error", e)
		}
	})

	t.Run("saving", func(t *testing.T) {
		mgr := openTestState(t)
		c, _ := newTestCore(t, Options{State: mgr})
		mgr.Close()

		c.Playback().AddTracks(Track{Path: "/music/a.flac"})

		if e := waitFailed(t, c, errmsg.OpQueueSave); e.Err == nil {
			t.Errorf("Failed = %+v, want the save error", e)
		}
	})
}

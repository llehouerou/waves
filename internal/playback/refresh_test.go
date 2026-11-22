package playback

import (
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// libraryService returns a service whose queue holds two library tracks and
// one from the file system, with no undo step, playing the first, with a
// subscription taken once it plays.
func libraryService(t *testing.T) (Service, *player.Mock, *Subscription) {
	t.Helper()
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{ID: 1, Path: "/a.mp3", Title: "a"},
		playlist.Track{ID: 2, Path: "/b.mp3", Title: "b"},
		playlist.Track{Path: "/c.mp3", Title: "c"},
	)
	q.ClearHistory()
	q.JumpTo(0)
	svc := New(p, q)
	t.Cleanup(func() { svc.Close() })
	must(t, svc.Play())
	return svc, p, svc.Subscribe()
}

// The entries of the library tracks take the tracks given: the track playing
// goes on with nothing that says it started or moved, and the queue is told
// once.
func TestRefreshLibraryTracks_IsNoEditAndLeavesPlaybackAlone(t *testing.T) {
	svc, p, sub := libraryService(t)
	entries := svc.QueueEntries()

	svc.RefreshLibraryTracks(map[int64]Track{
		1: {ID: 1, Path: "/a.mp3", Title: "A"},
		2: {ID: 2, Path: "/b.mp3", Title: "B"},
		0: {Path: "/c.mp3", Title: "C"}, // no library ID: the file's entry keeps its track
	})

	expectOnlyQueueChanges(t, sub, 1, 0)
	got := svc.QueueEntries()
	for i, want := range []string{"A", "B", "c"} {
		if got[i].Track.Title != want || got[i].ID != entries[i].ID {
			t.Errorf("entry %d = %d %q, want %d %q", i, got[i].ID, got[i].Track.Title, entries[i].ID, want)
		}
	}
	if p.State() != player.Playing || len(p.PlayCalls()) != 1 {
		t.Errorf("player %v after %v, want still playing the first track", p.State(), p.PlayCalls())
	}
	if current := svc.CurrentTrack(); current == nil || current.Title != "A" {
		t.Errorf("current track = %+v, want A", current)
	}
	if svc.Undo() {
		t.Errorf("undo walked to %v, want nothing to undo", svc.QueueTracks())
	}
}

func TestRefreshLibraryTracks_ThatChangesNothingTellsNothing(t *testing.T) {
	svc, _, sub := libraryService(t)

	svc.RefreshLibraryTracks(map[int64]Track{2: {ID: 2, Path: "/b.mp3", Title: "b"}, 3: {ID: 3, Path: "/d.mp3", Title: "D"}})

	expectOnlyQueueChanges(t, sub, 0, -2)
}

// A track started after the refresh starts under the names it was given.
func TestRefreshLibraryTracks_ATrackStartedAfterCarriesItsNewNames(t *testing.T) {
	svc, _, sub := libraryService(t)

	svc.RefreshLibraryTracks(map[int64]Track{2: {ID: 2, Path: "/b.mp3", Title: "B", Artist: "Low", Album: "Trust"}})
	must(t, svc.Next())

	select {
	case e := <-sub.TrackStarted:
		if e.Track.Title != "B" || e.Track.Artist != "Low" || e.Track.Album != "Trust" {
			t.Errorf("TrackStart = %+v, want B by Low on Trust", e.Track)
		}
	default:
		t.Fatal("no TrackStart")
	}
}

// The library tracks of the queue are those of its list and of the lists its
// history recorded, each once; a track from the file system is none.
func TestLibraryTrackIDs_AreThoseOfTheQueueAndItsHistory(t *testing.T) {
	svc, _, _ := libraryService(t)
	svc.AddTracks(Track{ID: 4, Path: "/d.mp3"}, Track{ID: 1, Path: "/a.mp3"})
	svc.RemoveTracks([]int{3})

	got := svc.LibraryTrackIDs()
	slices.Sort(got)
	if want := []int64{1, 2, 4}; !slices.Equal(got, want) {
		t.Errorf("LibraryTrackIDs = %v, want %v", got, want)
	}
}

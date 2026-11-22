package playback

import (
	"errors"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// selfStartService returns a service playing the first of paths, its player
// having started the second by itself, and the finish of the first held: it
// is signalled by the returned func.
func selfStartService(t *testing.T, paths ...string) (Service, *player.Mock, *Subscription, func()) {
	t.Helper()
	svc, p := playingService(t, playlist.RepeatOff, 0, paths...)
	synctest.Wait()
	sub := svc.Subscribe()
	finish := p.SimulateGaplessStart(paths[1])
	return svc, p, sub, finish
}

// A change that lands between a gapless self-start and the service handling it
// changes the track that follows: the service starts that track, the one the
// player started by itself is not reported (issue #250).
func TestSelfStart_ChangeBeforeTheFinish_StartsTheQueuesTrack(t *testing.T) {
	a, b, c := testSvcPathA, testSvcPathB, testSvcPathC
	tests := []struct {
		name      string
		change    func(svc Service)
		wantPath  string
		wantStart string
		wantIndex int
		moved     bool // the finish moves the queue to another entry
	}{
		{
			name:      "the self-started track removed",
			change:    func(svc Service) { svc.RemoveTracks([]int{1}) },
			wantPath:  c,
			wantStart: c + "@1",
			wantIndex: 1,
			moved:     true,
		},
		{
			name:      "the next track moved before the self-started one",
			change:    func(svc Service) { svc.MoveTracks([]int{1}, 1) },
			wantPath:  c,
			wantStart: c + "@1",
			wantIndex: 1,
			moved:     true,
		},
		{
			name:      "the queue position moved",
			change:    func(svc Service) { svc.QueueMoveTo(1) },
			wantPath:  c,
			wantStart: c + "@2",
			wantIndex: 2,
			moved:     true,
		},
		{
			name:      "repeat-one turned on",
			change:    func(svc Service) { svc.SetRepeatMode(RepeatOne) },
			wantPath:  a,
			wantStart: a + "@0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				svc, p, sub, finish := selfStartService(t, a, b, c)
				selfStarted := p.TrackStarts()
				tt.change(svc)
				synctest.Wait()
				trackChanges(sub)
				finish()
				synctest.Wait()

				if got, want := p.PlayCalls(), []string{a, tt.wantPath}; !slices.Equal(got, want) {
					t.Errorf("play calls = %v, want %v", got, want)
				}
				if got, want := starts(sub), []string{tt.wantStart}; !slices.Equal(got, want) {
					t.Errorf("track starts = %v, want %v", got, want)
				}
				if got, want := p.TrackInfo().Path, svc.CurrentTrack().Path; got != want {
					t.Errorf("player on %s, queue on %s", got, want)
				}
				changes := trackChanges(sub)
				switch {
				case !tt.moved && len(changes) != 0:
					t.Errorf("track changes = %+v, want none", changes)
				case tt.moved && (len(changes) != 1 || changes[0].Index != tt.wantIndex || changes[0].Current.Path != tt.wantPath):
					t.Errorf("track changes = %+v, want one to %s", changes, tt.wantStart)
				}

				// The finish of the track the player started by itself is
				// stale: the service started the queue's track since.
				p.SimulateFinishedOf(selfStarted)
				synctest.Wait()
				if got := len(p.PlayCalls()); got != 2 {
					t.Errorf("play calls after the stale finish = %v, want none more", p.PlayCalls())
				}
				if got := starts(sub); len(got) != 0 {
					t.Errorf("track starts after the stale finish = %v, want none", got)
				}
			})
		})
	}
}

// With nothing changed in between, the self-start is the start of the queue's
// track: it is reported, and the player is left alone.
func TestSelfStart_NoChange_ReportsTheSelfStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, p, sub, finish := selfStartService(t, testSvcPathA, testSvcPathB, testSvcPathC)
		finish()
		synctest.Wait()

		if got, want := p.PlayCalls(), []string{testSvcPathA}; !slices.Equal(got, want) {
			t.Errorf("play calls = %v, want %v", got, want)
		}
		if got, want := starts(sub), []string{testSvcPathB + "@1"}; !slices.Equal(got, want) {
			t.Errorf("track starts = %v, want %v", got, want)
		}
	})
}

// A player with no track after its self-start is not on the queue's track:
// the service starts it.
func TestSelfStart_PlayerWithNoTrack_StartsTheQueuesTrack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, p, sub, finish := selfStartService(t, testSvcPathA, testSvcPathB, testSvcPathC)
		p.SetTrackInfo(nil)
		finish()
		synctest.Wait()

		if got, want := p.PlayCalls(), []string{testSvcPathA, testSvcPathB}; !slices.Equal(got, want) {
			t.Errorf("play calls = %v, want %v", got, want)
		}
		if got, want := starts(sub), []string{testSvcPathB + "@1"}; !slices.Equal(got, want) {
			t.Errorf("track starts = %v, want %v", got, want)
		}
	})
}

// The queue's track is another entry with the path the player started: the
// file heard is the one reported, so the self-start stands.
func TestSelfStart_SamePathAnotherEntry_ReportsTheSelfStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p, sub, finish := selfStartService(t, testSvcPathA, testSvcPathB, testSvcPathB)
		svc.RemoveTracks([]int{1})
		finish()
		synctest.Wait()

		if got, want := p.PlayCalls(), []string{testSvcPathA}; !slices.Equal(got, want) {
			t.Errorf("play calls = %v, want %v", got, want)
		}
		if got, want := starts(sub), []string{testSvcPathB + "@1"}; !slices.Equal(got, want) {
			t.Errorf("track starts = %v, want %v", got, want)
		}
	})
}

// A queue left with nothing to move to stops the player on the track it
// started by itself.
func TestSelfStart_NothingToMoveTo_Stops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p, sub, finish := selfStartService(t, testSvcPathA, testSvcPathB)
		svc.RemoveTracks([]int{1})
		finish()
		synctest.Wait()

		if !svc.IsStopped() {
			t.Error("the player still runs on the track removed")
		}
		if got, want := p.PlayCalls(), []string{testSvcPathA}; !slices.Equal(got, want) {
			t.Errorf("play calls = %v, want %v", got, want)
		}
		if got := starts(sub); len(got) != 0 {
			t.Errorf("track starts = %v, want none", got)
		}
		expectStateChanges(t, sub, StateChange{Previous: StatePlaying, Current: StateStopped})
	})
}

// The queue's track the service starts in place of the self-started one fails
// to open: handled as any failed start of a finish.
func TestSelfStart_ChangeBeforeTheFinish_FailedOpen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p, sub, finish := selfStartService(t, testSvcPathA, testSvcPathB, testSvcPathC)
		svc.RemoveTracks([]int{1})
		p.SetPlayError(errors.New("unreadable"))
		finish()
		synctest.Wait()

		if !svc.IsStopped() {
			t.Error("the player still runs on the track removed")
		}
		if got := starts(sub); len(got) != 0 {
			t.Errorf("track starts = %v, want none", got)
		}
		expectStateChanges(t, sub, StateChange{Previous: StatePlaying, Current: StateStopped})
		if got, want := errorPaths(sub), []string{testSvcPathC}; !slices.Equal(got, want) {
			t.Errorf("errors = %v, want %v", got, want)
		}
		changes := trackChanges(sub)
		if len(changes) != 1 || changes[0].Index != 1 || changes[0].Current.Path != testSvcPathC {
			t.Errorf("track changes = %+v, want one to C at 1", changes)
		}
	})
}

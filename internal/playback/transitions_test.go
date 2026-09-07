package playback

import (
	"errors"
	"testing"
	"testing/synctest"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// Repeat-one: the queue "advances" to the same track, which must be replayed.
func TestEdge_RepeatOne_ReplaysTrack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(0)
		q.SetRepeatMode(playlist.RepeatOne)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}

		p.SimulateFinished()
		synctest.Wait()

		calls := p.PlayCalls()
		if len(calls) != 2 || calls[1] != testSvcPathA {
			t.Errorf("PlayCalls() = %v, want the track replayed", calls)
		}
	})
}

// The same track twice in a row in the queue: the second must be played.
func TestEdge_DuplicateConsecutiveTrack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathA})
		q.JumpTo(0)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}

		p.SimulateFinished()
		synctest.Wait()

		calls := p.PlayCalls()
		if len(calls) != 2 {
			t.Errorf("PlayCalls() = %v, want the duplicate played again", calls)
		}
	})
}

// Repeat-all with a single track: the queue wraps onto the same track.
func TestEdge_RepeatAll_SingleTrack_Replays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA})
		q.JumpTo(0)
		q.SetRepeatMode(playlist.RepeatAll)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}

		p.SimulateFinished()
		synctest.Wait()

		if calls := p.PlayCalls(); len(calls) != 2 {
			t.Errorf("PlayCalls() = %v, want the single track replayed", calls)
		}
	})
}

// A real gapless switch onto the same path (repeat-one with preload) must not be
// restarted: the player already began it.
func TestEdge_RepeatOne_GaplessSwitch_NotRestarted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA})
		q.JumpTo(0)
		q.SetRepeatMode(playlist.RepeatOne)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}

		p.SimulateGaplessSwitch(testSvcPathA)
		synctest.Wait()

		if calls := p.PlayCalls(); len(calls) != 1 {
			t.Errorf("PlayCalls() = %v, want no restart after a gapless switch", calls)
		}
	})
}

// Seeking past the end stops the player before signalling finished (issue #38).
// The TrackChange must reach subscribers only once the next track is playing:
// the UI sizes its layout from the live state when it handles that event.
func TestEdge_SeekPastEnd_TrackChangeAfterNextStarted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(0)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}
		sub := svc.Subscribe()

		// Seeking past the end: the player stops, then reports finished. Hold the
		// next track open so the transition is observable mid-flight.
		release := p.BlockPlay()
		p.Stop()
		p.SimulateFinished()
		synctest.Wait()

		select {
		case e := <-sub.TrackChanged:
			t.Fatalf("TrackChange %+v emitted while the player is still stopped", e)
		default:
		}

		release()
		synctest.Wait()

		select {
		case <-sub.TrackChanged:
		default:
			t.Fatal("no TrackChange emitted after the next track started")
		}
		if svc.State() != StatePlaying {
			t.Errorf("State() = %v at TrackChange, want Playing", svc.State())
		}
	})
}

// The tests below pin the behaviour of the four queue transitions
// (handleTrackFinished, Next, Previous, JumpTo) at the edges where they
// currently diverge, so the collapse into one transition can be shown to change
// nothing. See issue #52.

// A paused player is still active: Next must start the track it moved to.
func TestTransition_Next_WhilePaused_StartsNextTrack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(0)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}
		if err := svc.Pause(); err != nil {
			t.Fatal(err)
		}

		if err := svc.Next(); err != nil {
			t.Fatalf("Next() error = %v", err)
		}

		calls := p.PlayCalls()
		if len(calls) != 2 || calls[1] != testSvcPathB {
			t.Errorf("PlayCalls() = %v, want the next track started from paused", calls)
		}
	})
}

// Next at the end of the queue while paused stops, and the StateChange reports
// Paused as the previous state.
func TestTransition_Next_AtEndWhilePaused_StopsFromPaused(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA})
		q.JumpTo(0)

		svc := New(p, q)
		defer svc.Close()
		sub := svc.Subscribe()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}
		<-sub.StateChanged
		if err := svc.Pause(); err != nil {
			t.Fatal(err)
		}
		<-sub.StateChanged

		if err := svc.Next(); err != nil {
			t.Fatalf("Next() error = %v", err)
		}

		e := <-sub.StateChanged
		if e.Previous != StatePaused || e.Current != StateStopped {
			t.Errorf("StateChange = %v -> %v, want Paused -> Stopped", e.Previous, e.Current)
		}
		if svc.State() != StateStopped {
			t.Errorf("State() = %v, want Stopped", svc.State())
		}
	})
}

// Previous from a paused player starts the track it moved to.
func TestTransition_Previous_WhilePaused_StartsPreviousTrack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(1)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}
		if err := svc.Pause(); err != nil {
			t.Fatal(err)
		}

		if err := svc.Previous(); err != nil {
			t.Fatalf("Previous() error = %v", err)
		}

		calls := p.PlayCalls()
		if len(calls) != 2 || calls[1] != testSvcPathA {
			t.Errorf("PlayCalls() = %v, want the previous track started from paused", calls)
		}
	})
}

// Previous while stopped moves the queue and reports it, but starts nothing.
func TestTransition_Previous_WhileStopped_MovesWithoutPlaying(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(1)

		svc := New(p, q)
		defer svc.Close()
		sub := svc.Subscribe()

		if err := svc.Previous(); err != nil {
			t.Fatalf("Previous() error = %v", err)
		}

		e := <-sub.TrackChanged
		if e.Index != 0 {
			t.Errorf("TrackChange.Index = %d, want 0", e.Index)
		}
		if e.Previous == nil || e.Previous.Path != testSvcPathB {
			t.Errorf("TrackChange.Previous = %v, want %s", e.Previous, testSvcPathB)
		}
		if len(p.PlayCalls()) != 0 {
			t.Errorf("PlayCalls() = %v, want none while stopped", p.PlayCalls())
		}
	})
}

// JumpTo from a paused player starts the track it moved to.
func TestTransition_JumpTo_WhilePaused_StartsTargetTrack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(
			playlist.Track{Path: testSvcPathA},
			playlist.Track{Path: testSvcPathB},
			playlist.Track{Path: testSvcPathC},
		)
		q.JumpTo(0)

		svc := New(p, q)
		defer svc.Close()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}
		if err := svc.Pause(); err != nil {
			t.Fatal(err)
		}

		if err := svc.JumpTo(2); err != nil {
			t.Fatalf("JumpTo() error = %v", err)
		}

		calls := p.PlayCalls()
		if len(calls) != 2 || calls[1] != testSvcPathC {
			t.Errorf("PlayCalls() = %v, want the target track started from paused", calls)
		}
	})
}

// A failed open on a track finish stops the player and reports both the state
// change and the error.
func TestTransition_Finished_StartFails_StopsAndReportsError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(0)

		svc := New(p, q)
		defer svc.Close()
		sub := svc.Subscribe()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}
		<-sub.StateChanged

		p.SetPlayError(errors.New("open failed"))
		p.SimulateFinished()
		synctest.Wait()

		e := <-sub.StateChanged
		if e.Current != StateStopped {
			t.Errorf("StateChange.Current = %v, want Stopped", e.Current)
		}
		select {
		case ev := <-sub.Error:
			if ev.Path != testSvcPathB {
				t.Errorf("ErrorEvent.Path = %q, want %q", ev.Path, testSvcPathB)
			}
		default:
			t.Error("no ErrorEvent emitted for a failed open on finish")
		}
		if svc.State() != StateStopped {
			t.Errorf("State() = %v, want Stopped", svc.State())
		}
	})
}

// A failed open on Next stops the player and reports it, the same as a failed
// open on a track finish. Before issue #52 it returned the error and nothing
// else, leaving the queue moved onto a track that was not playing while the
// player kept running on the old one, with no event to tell anyone.
func TestTransition_Next_StartFails_StopsAndReportsError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(0)

		svc := New(p, q)
		defer svc.Close()
		sub := svc.Subscribe()
		if err := svc.Play(); err != nil {
			t.Fatal(err)
		}
		<-sub.StateChanged

		p.SetPlayError(errors.New("open failed"))
		err := svc.Next()

		if err == nil {
			t.Fatal("Next() error = nil, want the open failure")
		}
		if svc.State() != StateStopped {
			t.Errorf("State() = %v, want Stopped", svc.State())
		}
		e := <-sub.StateChanged
		if e.Previous != StatePlaying || e.Current != StateStopped {
			t.Errorf("StateChange = %v -> %v, want Playing -> Stopped", e.Previous, e.Current)
		}
		select {
		case ev := <-sub.Error:
			if ev.Path != testSvcPathB {
				t.Errorf("ErrorEvent.Path = %q, want %q", ev.Path, testSvcPathB)
			}
		default:
			t.Error("no ErrorEvent emitted for a failed open on Next")
		}
	})
}

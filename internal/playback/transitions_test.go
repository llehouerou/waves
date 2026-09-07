package playback

import (
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

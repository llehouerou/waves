package playback

import (
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// abcService returns a stopped service on the queue A, B, C with A current,
// and a subscription taken once it is set up.
func abcService(t *testing.T) (Service, *Subscription) {
	t.Helper()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB}, playlist.Track{Path: testSvcPathC})
	q.JumpTo(0)
	svc := New(player.NewMock(), q)
	t.Cleanup(func() { svc.Close() })
	return svc, svc.Subscribe()
}

// expectTrackChange fails the test unless sub has received exactly one
// TrackChange, from the index from to the index to.
func expectTrackChange(t *testing.T, sub *Subscription, from, to int) {
	t.Helper()
	got := trackChanges(sub)
	if len(got) != 1 {
		t.Fatalf("TrackChanges = %+v, want one from %d to %d", got, from, to)
	}
	if got[0].PreviousIndex != from || got[0].Index != to {
		t.Errorf("TrackChange from %d to %d, want from %d to %d", got[0].PreviousIndex, got[0].Index, from, to)
	}
}

// Starting the current track from stopped records it as the last played,
// whether Play or Toggle asks: the next start of another track is a track
// change.
func TestStartFromStopped_RecordsTheLastPlayed(t *testing.T) {
	for name, start := range starters {
		t.Run(name, func(t *testing.T) {
			svc, sub := abcService(t)

			must(t, start(svc))
			svc.QueueMoveTo(1)
			must(t, svc.Play())

			expectTrackChange(t, sub, 0, 1)
		})
	}
}

// Starting from stopped a track other than the last played is a track change,
// whether Play or Toggle asks.
func TestStartFromStopped_OfAnotherTrack_IsATrackChange(t *testing.T) {
	for name, start := range starters {
		t.Run(name, func(t *testing.T) {
			svc, sub := abcService(t)
			must(t, svc.Play())
			must(t, svc.Stop())
			svc.QueueMoveTo(2)

			must(t, start(svc))
			expectTrackChange(t, sub, 0, 2)

			svc.QueueMoveTo(0)
			must(t, svc.Play())
			expectTrackChange(t, sub, 2, 0)
		})
	}
}

package playback

import (
	"errors"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// starters are the two commands that start the queue from stopped.
var starters = map[string]func(Service) error{
	"play":   Service.Play,
	"toggle": Service.Toggle,
}

// noCurrentService returns a stopped service on the queue A, B, C with no
// current track, and a subscription taken once it is set up.
func noCurrentService(t *testing.T, shuffle bool) (Service, *player.Mock, *Subscription) {
	t.Helper()
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB}, playlist.Track{Path: testSvcPathC})
	q.SetShuffle(shuffle)
	svc := New(p, q)
	t.Cleanup(func() { svc.Close() })
	return svc, p, svc.Subscribe()
}

// expectMovedToFirst fails the test unless sub has received exactly one
// TrackChange, from no track to path at index 0.
func expectMovedToFirst(t *testing.T, sub *Subscription, path string) {
	t.Helper()
	got := trackChanges(sub)
	if len(got) != 1 {
		t.Fatalf("TrackChanges = %+v, want one", got)
	}
	if e := got[0]; e.Index != 0 || e.Current == nil || e.Current.Path != path {
		t.Errorf("TrackChange moved to %d %+v, want %s at 0", e.Index, e.Current, path)
	}
}

// A stopped queue with tracks and no current track starts its first one,
// shuffle on or off, and tells every subscriber the queue moved there.
func TestNoCurrentTrack_StartsTheFirstTrack(t *testing.T) {
	for name, start := range starters {
		for _, mode := range []struct {
			suffix  string
			shuffle bool
		}{{"", false}, {" with shuffle", true}} {
			t.Run(name+mode.suffix, func(t *testing.T) {
				svc, p, sub := noCurrentService(t, mode.shuffle)

				must(t, start(svc))

				if got := p.PlayCalls(); !slices.Equal(got, []string{testSvcPathA}) {
					t.Errorf("PlayCalls = %v, want [%s]", got, testSvcPathA)
				}
				if got := svc.QueueCurrentIndex(); got != 0 {
					t.Errorf("QueueCurrentIndex() = %d, want 0", got)
				}
				if got := starts(sub); !slices.Equal(got, []string{testSvcPathA + "@0"}) {
					t.Errorf("TrackStarts = %v, want [%s@0]", got, testSvcPathA)
				}
				expectStateChanges(t, sub, StateChange{Previous: StateStopped, Current: StatePlaying})
				expectMovedToFirst(t, sub, testSvcPathA)
			})
		}
	}
}

// Once the current track is removed and playback stopped, the start is that
// of the track now first in the queue.
func TestNoCurrentTrack_AfterTheCurrentTrackWasRemoved_StartsTheFirstTrack(t *testing.T) {
	for name, start := range starters {
		t.Run(name, func(t *testing.T) {
			svc, p := stoppedService(t)
			must(t, svc.Play())
			svc.RemoveTracks([]int{0})
			must(t, svc.Stop())
			sub := svc.Subscribe()

			must(t, start(svc))

			if got := p.PlayCalls(); !slices.Equal(got, []string{testSvcPathA, testSvcPathB}) {
				t.Errorf("PlayCalls = %v, want [%s %s]", got, testSvcPathA, testSvcPathB)
			}
			if got := svc.QueueCurrentIndex(); got != 0 {
				t.Errorf("QueueCurrentIndex() = %d, want 0", got)
			}
			if got := starts(sub); !slices.Equal(got, []string{testSvcPathB + "@0"}) {
				t.Errorf("TrackStarts = %v, want [%s@0]", got, testSvcPathB)
			}
			expectStateChanges(t, sub, StateChange{Previous: StateStopped, Current: StatePlaying})
			expectMovedToFirst(t, sub, testSvcPathB)
		})
	}
}

// The start of the first track is recorded as the last played: a later Play
// on another track reports the move.
func TestNoCurrentTrack_StartOfTheFirstTrackIsLastPlayed(t *testing.T) {
	for name, start := range starters {
		t.Run(name, func(t *testing.T) {
			svc, _, sub := noCurrentService(t, false)
			must(t, start(svc))
			trackChanges(sub)

			svc.QueueMoveTo(2)
			must(t, svc.Play())

			got := trackChanges(sub)
			if len(got) != 1 {
				t.Fatalf("TrackChanges = %+v, want one", got)
			}
			e := got[0]
			if e.PreviousIndex != 0 || e.Previous == nil || e.Previous.Path != testSvcPathA {
				t.Errorf("TrackChange left %d %+v, want A at 0", e.PreviousIndex, e.Previous)
			}
			if e.Index != 2 || e.Current == nil || e.Current.Path != testSvcPathC {
				t.Errorf("TrackChange moved to %d %+v, want C at 2", e.Index, e.Current)
			}
		})
	}
}

// A first track that fails to open leaves the queue on it, stopped: the
// caller gets the error, subscribers the move and no ErrorEvent.
func TestNoCurrentTrack_FirstTrackThatFailsToOpen(t *testing.T) {
	for name, start := range starters {
		t.Run(name, func(t *testing.T) {
			svc, p, sub := noCurrentService(t, false)
			p.SetPlayError(errUnreadable)

			if err := start(svc); !errors.Is(err, errUnreadable) {
				t.Fatalf("error = %v, want the open error", err)
			}

			if got := svc.State(); got != StateStopped {
				t.Errorf("State() = %v, want Stopped", got)
			}
			if got := svc.QueueCurrentIndex(); got != 0 {
				t.Errorf("QueueCurrentIndex() = %d, want 0", got)
			}
			expectMovedToFirst(t, sub, testSvcPathA)
			if got := errorPaths(sub); len(got) != 0 {
				t.Errorf("ErrorEvents for %v, want none", got)
			}
		})
	}
}

// With a current track, the start is that of the current track, not the first.
func TestNoCurrentTrack_CurrentTrackIsStartedRatherThanTheFirst(t *testing.T) {
	for name, start := range starters {
		t.Run(name, func(t *testing.T) {
			svc, p := stoppedService(t)
			svc.QueueMoveTo(1)

			must(t, start(svc))

			if got := p.PlayCalls(); !slices.Equal(got, []string{testSvcPathB}) {
				t.Errorf("PlayCalls = %v, want [%s]", got, testSvcPathB)
			}
			if got := svc.QueueCurrentIndex(); got != 1 {
				t.Errorf("QueueCurrentIndex() = %d, want 1", got)
			}
		})
	}
}

func TestNoCurrentTrack_EmptyQueueIsRefused(t *testing.T) {
	for name, start := range starters {
		t.Run(name, func(t *testing.T) {
			svc := New(player.NewMock(), playlist.NewQueue())
			t.Cleanup(func() { svc.Close() })

			if err := start(svc); !errors.Is(err, ErrEmptyQueue) {
				t.Errorf("error = %v, want ErrEmptyQueue", err)
			}
		})
	}
}

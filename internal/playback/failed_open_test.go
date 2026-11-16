package playback

import (
	"errors"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

var errUnreadable = errors.New("unreadable")

// failedOpenService returns a service on the queue A, B, C, with B started and
// left in the given state, and a subscription taken after that start. Every
// open from now on fails, the way the real player fails it: stopped first.
func failedOpenService(t *testing.T, state State) (Service, *player.Mock, *Subscription) {
	t.Helper()
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB}, playlist.Track{Path: testSvcPathC})
	q.JumpTo(1)
	svc := New(p, q)
	t.Cleanup(func() { svc.Close() })
	must(t, svc.Play())
	if state == StatePaused {
		must(t, svc.Pause())
	}
	synctest.Wait()

	sub := svc.Subscribe()
	p.SetPlayError(errUnreadable)
	return svc, p, sub
}

// errorPaths drains the subscription's errors and returns their paths.
func errorPaths(sub *Subscription) []string {
	var got []string
	for len(sub.Error) > 0 {
		got = append(got, (<-sub.Error).Path)
	}
	return got
}

// A track that fails to open in place of one playing or paused stops playback:
// subscribers are told, once, from the state the start left.
func TestFailedOpen_ReplacingAnActiveTrackReportsTheStop(t *testing.T) {
	tests := []struct {
		name  string
		state State
		act   func(svc Service, p *player.Mock) error
		// wantErr is what act returns: nil for a finish, which nobody waits on.
		wantErr error
		// errorPaths are the tracks the ErrorEvents name.
		errorPaths []string
	}{
		{
			name:       "track finishes",
			state:      StatePlaying,
			act:        func(_ Service, p *player.Mock) error { p.SimulateFinished(); return nil },
			errorPaths: []string{testSvcPathC},
		},
		{
			name:  "track finishes after a seek past the end stopped the player",
			state: StatePlaying,
			act: func(_ Service, p *player.Mock) error {
				p.SetState(player.Stopped)
				p.SimulateFinished()
				return nil
			},
			errorPaths: []string{testSvcPathC},
		},
		{
			name:       "next",
			state:      StatePlaying,
			wantErr:    errUnreadable,
			act:        func(svc Service, _ *player.Mock) error { return svc.Next() },
			errorPaths: []string{testSvcPathC},
		},
		{
			name:       "next while paused",
			state:      StatePaused,
			wantErr:    errUnreadable,
			act:        func(svc Service, _ *player.Mock) error { return svc.Next() },
			errorPaths: []string{testSvcPathC},
		},
		{
			name:       "previous",
			state:      StatePlaying,
			wantErr:    errUnreadable,
			act:        func(svc Service, _ *player.Mock) error { return svc.Previous() },
			errorPaths: []string{testSvcPathA},
		},
		{
			name:       "jump",
			state:      StatePlaying,
			wantErr:    errUnreadable,
			act:        func(svc Service, _ *player.Mock) error { return svc.JumpTo(2) },
			errorPaths: []string{testSvcPathC},
		},
		{
			// How the TUI starts a track. The caller has the error: no ErrorEvent.
			name:    "play another track",
			state:   StatePlaying,
			wantErr: errUnreadable,
			act: func(svc Service, _ *player.Mock) error {
				svc.QueueMoveTo(2)
				return svc.Play()
			},
		},
		{
			name:    "play another track while paused",
			state:   StatePaused,
			wantErr: errUnreadable,
			act: func(svc Service, _ *player.Mock) error {
				svc.QueueMoveTo(2)
				return svc.Play()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				svc, p, sub := failedOpenService(t, tt.state)

				if err := tt.act(svc, p); !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				synctest.Wait()

				expectStateChanges(t, sub, StateChange{Previous: tt.state, Current: StateStopped})
				if got := errorPaths(sub); !slices.Equal(got, tt.errorPaths) {
					t.Errorf("ErrorEvent paths = %v, want %v", got, tt.errorPaths)
				}
				if got := svc.State(); got != StateStopped {
					t.Errorf("State() = %v, want Stopped", got)
				}
			})
		})
	}
}

// A start from a stop that fails to open changes no state.
func TestFailedOpen_FromStopReportsNoStateChange(t *testing.T) {
	tests := []struct {
		name string
		svc  func(t *testing.T) (Service, *player.Mock)
		act  func(svc Service) error
	}{
		{
			name: "play",
			svc:  stoppedService,
			act:  func(svc Service) error { return svc.Play() },
		},
		{
			name: "toggle",
			svc:  stoppedService,
			act:  func(svc Service) error { return svc.Toggle() },
		},
		{
			name: "resume after the queue ran out",
			svc: func(t *testing.T) (Service, *player.Mock) {
				svc, p := ranOutService(t)
				svc.AddTracks(Track{Path: testSvcPathC})
				return svc, p
			},
			act: func(svc Service) error { return svc.ResumeAfterRunOut() },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				svc, p := tt.svc(t)
				sub := svc.Subscribe()
				p.SetPlayError(errUnreadable)

				if err := tt.act(svc); !errors.Is(err, errUnreadable) {
					t.Errorf("error = %v, want %v", err, errUnreadable)
				}
				synctest.Wait()

				expectStateChanges(t, sub)
				if got := svc.State(); got != StateStopped {
					t.Errorf("State() = %v, want Stopped", got)
				}
			})
		})
	}
}

// stoppedService returns a stopped service on the queue A, B, at A.
func stoppedService(t *testing.T) (Service, *player.Mock) {
	t.Helper()
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
	q.JumpTo(0)
	svc := New(p, q)
	t.Cleanup(func() { svc.Close() })
	return svc, p
}

// trackChanges drains the subscription's track changes.
func trackChanges(sub *Subscription) []TrackChange {
	var got []TrackChange
	for len(sub.TrackChanged) > 0 {
		got = append(got, <-sub.TrackChanged)
	}
	return got
}

// expectTrackChangeFromB fails the test unless sub has received exactly one
// TrackChange, from B at 1 to path at index.
func expectTrackChangeFromB(t *testing.T, sub *Subscription, index int, path string) {
	t.Helper()
	got := trackChanges(sub)
	if len(got) != 1 {
		t.Fatalf("TrackChanges = %+v, want one", got)
	}
	e := got[0]
	if e.PreviousIndex != 1 || e.Previous == nil || e.Previous.Path != testSvcPathB {
		t.Errorf("TrackChange left %d %+v, want B at 1", e.PreviousIndex, e.Previous)
	}
	if e.Index != index || e.Current == nil || e.Current.Path != path {
		t.Errorf("TrackChange moved to %d %+v, want %s at %d", e.Index, e.Current, path, index)
	}
}

// expectNoQueueChangeNorStart fails the test if sub received a QueueChange or
// a TrackStart.
func expectNoQueueChangeNorStart(t *testing.T, sub *Subscription) {
	t.Helper()
	if n := len(sub.QueueChanged); n != 0 {
		t.Errorf("%d QueueChanges, want none", n)
	}
	if got := starts(sub); len(got) != 0 {
		t.Errorf("TrackStarts = %v, want none", got)
	}
}

// A transition moves the queue whether or not the track it moved to opens:
// subscribers are told where it is, the way a start that worked tells them.
func TestFailedOpen_TransitionReportsTheTrackItMovedTo(t *testing.T) {
	tests := []struct {
		name      string
		state     State
		act       func(svc Service, p *player.Mock)
		wantIndex int
		wantPath  string
	}{
		{
			name:      "track finishes",
			state:     StatePlaying,
			act:       func(_ Service, p *player.Mock) { p.SimulateFinished() },
			wantIndex: 2,
			wantPath:  testSvcPathC,
		},
		{
			name:  "track finishes after a seek past the end stopped the player",
			state: StatePlaying,
			act: func(_ Service, p *player.Mock) {
				p.SetState(player.Stopped)
				p.SimulateFinished()
			},
			wantIndex: 2,
			wantPath:  testSvcPathC,
		},
		{
			name:      "next",
			state:     StatePlaying,
			act:       func(svc Service, _ *player.Mock) { _ = svc.Next() },
			wantIndex: 2,
			wantPath:  testSvcPathC,
		},
		{
			name:      "next while paused",
			state:     StatePaused,
			act:       func(svc Service, _ *player.Mock) { _ = svc.Next() },
			wantIndex: 2,
			wantPath:  testSvcPathC,
		},
		{
			name:      "previous",
			state:     StatePlaying,
			act:       func(svc Service, _ *player.Mock) { _ = svc.Previous() },
			wantIndex: 0,
			wantPath:  testSvcPathA,
		},
		{
			name:      "jump",
			state:     StatePlaying,
			act:       func(svc Service, _ *player.Mock) { _ = svc.JumpTo(2) },
			wantIndex: 2,
			wantPath:  testSvcPathC,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				svc, p, sub := failedOpenService(t, tt.state)

				tt.act(svc, p)
				synctest.Wait()

				expectTrackChangeFromB(t, sub, tt.wantIndex, tt.wantPath)
				expectStateChanges(t, sub, StateChange{Previous: tt.state, Current: StateStopped})
				if got := errorPaths(sub); !slices.Equal(got, []string{tt.wantPath}) {
					t.Errorf("ErrorEvent paths = %v, want [%s]", got, tt.wantPath)
				}
				expectNoQueueChangeNorStart(t, sub)
				if got := svc.QueueCurrentIndex(); got != tt.wantIndex {
					t.Errorf("QueueCurrentIndex() = %d, want %d", got, tt.wantIndex)
				}
			})
		})
	}
}

// The TrackChange of a failed start comes once the player stopped, not while
// the track is still opening: a subscriber that reads the live state on it
// sees the state it ends in.
func TestFailedOpen_TrackChangeOnceThePlayerStopped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p, sub := failedOpenService(t, StatePlaying)
		release := p.BlockPlay()

		go func() { _ = svc.Next() }()
		synctest.Wait()
		if got := trackChanges(sub); len(got) != 0 {
			t.Fatalf("TrackChange %+v emitted while the track is still opening", got)
		}

		release()
		synctest.Wait()
		expectTrackChangeFromB(t, sub, 2, testSvcPathC)
		if got := svc.State(); got != StateStopped {
			t.Errorf("State() = %v at TrackChange, want Stopped", got)
		}
	})
}

// Resuming a queue that ran out onto a track that fails to open moves the
// queue: it is told, from a stop to a stop.
func TestFailedOpen_ResumeAfterRunOutReportsTheTrackItMovedTo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := ranOutService(t)
		svc.AddTracks(Track{Path: testSvcPathC})
		sub := svc.Subscribe()
		p.SetPlayError(errUnreadable)

		if err := svc.ResumeAfterRunOut(); !errors.Is(err, errUnreadable) {
			t.Errorf("error = %v, want %v", err, errUnreadable)
		}
		synctest.Wait()

		expectTrackChangeFromB(t, sub, 2, testSvcPathC)
		expectStateChanges(t, sub)
		if got := errorPaths(sub); !slices.Equal(got, []string{testSvcPathC}) {
			t.Errorf("ErrorEvent paths = %v, want [%s]", got, testSvcPathC)
		}
		expectNoQueueChangeNorStart(t, sub)
	})
}

// Repeat-one finishing into its own track, which no longer opens, stays on
// that track: no TrackChange.
func TestFailedOpen_RepeatOneOntoItselfReportsNoTrackChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := playingService(t, playlist.RepeatOne, 1, testSvcPathA, testSvcPathB)
		synctest.Wait()
		sub := svc.Subscribe()
		p.SetPlayError(errUnreadable)

		finish(p)

		if got := trackChanges(sub); len(got) != 0 {
			t.Errorf("TrackChanges = %+v, want none", got)
		}
		if got := errorPaths(sub); !slices.Equal(got, []string{testSvcPathB}) {
			t.Errorf("ErrorEvent paths = %v, want [%s]", got, testSvcPathB)
		}
	})
}

// The track a failed transition moved to was told: playing it once it opens
// tells nothing more.
func TestFailedOpen_PlayingTheTrackOnceItOpensReportsNoTrackChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p, sub := failedOpenService(t, StatePlaying)
		_ = svc.Next()
		synctest.Wait()
		trackChanges(sub)

		p.SetPlayError(nil)
		must(t, svc.Play())
		synctest.Wait()

		if got := trackChanges(sub); len(got) != 0 {
			t.Errorf("TrackChanges = %+v, want none", got)
		}
		if got := starts(sub); !slices.Equal(got, []string{testSvcPathC + "@2"}) {
			t.Errorf("TrackStarts = %v, want [%s@2]", got, testSvcPathC)
		}
	})
}

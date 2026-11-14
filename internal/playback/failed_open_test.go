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

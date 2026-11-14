package playback

import (
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// settle is how long a test gives the watcher to act on a finish, when what it
// checks is that nothing happens.
const settle = 20 * time.Millisecond

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// finishDuringOpen runs cmd with the open of its track held, signals a finish
// while it is held, lets the watcher receive it, then releases the open. It
// returns what cmd returned once anything the finish set off had time to show.
// Not under synctest: a regression blocks the watcher on playPathMu, which
// synctest.Wait does not count as blocked, so the test would hang, not fail.
func finishDuringOpen(t *testing.T, p *player.Mock, cmd func() error) error {
	t.Helper()
	calls := len(p.PlayCalls())
	release := p.BlockPlay()

	errc := make(chan error, 1)
	go func() { errc <- cmd() }()
	waitFor(t, "the command to reach the open", func() bool { return len(p.PlayCalls()) > calls })

	p.SimulateFinished()
	waitFor(t, "the watcher to receive the finish", func() bool { return len(p.FinishedChan()) == 0 })
	time.Sleep(settle)
	release()

	var err error
	select {
	case err = <-errc:
	case <-time.After(2 * time.Second):
		t.Fatal("the command never returned")
	}
	time.Sleep(settle)
	return err
}

func newStaleFinishService(t *testing.T) (Service, *player.Mock, *Subscription) {
	t.Helper()
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB}, playlist.Track{Path: testSvcPathC})
	q.JumpTo(0)
	svc := New(p, q)
	t.Cleanup(func() { svc.Close() })
	return svc, p, svc.Subscribe()
}

// A finish that arrives while a command opens a track belongs to the track the
// command replaces: the command's transition is the only one (issue #120).
func TestStaleFinish_CommandOpeningATrack(t *testing.T) {
	a, b, c := testSvcPathA, testSvcPathB, testSvcPathC
	tests := []struct {
		name       string
		setup      func(t *testing.T, svc Service, p *player.Mock)
		cmd        func(svc Service) error
		wantIndex  int
		wantCalls  []string
		wantStarts []string
	}{
		{
			name:       "next",
			setup:      func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			cmd:        Service.Next,
			wantIndex:  1,
			wantCalls:  []string{a, b},
			wantStarts: []string{a + "@0", b + "@1"},
		},
		{
			name: "play",
			setup: func(t *testing.T, svc Service, _ *player.Mock) {
				must(t, svc.Play())
				svc.QueueMoveTo(1)
			},
			cmd:        Service.Play,
			wantIndex:  1,
			wantCalls:  []string{a, b},
			wantStarts: []string{a + "@0", b + "@1"},
		},
		{
			name: "toggle from stopped",
			setup: func(t *testing.T, svc Service, p *player.Mock) {
				must(t, svc.Play())
				p.SetState(player.Stopped)
			},
			cmd:        Service.Toggle,
			wantIndex:  0,
			wantCalls:  []string{a, a},
			wantStarts: []string{a + "@0", a + "@0"},
		},
		{
			name: "previous",
			setup: func(t *testing.T, svc Service, _ *player.Mock) {
				must(t, svc.JumpTo(1))
				must(t, svc.Play())
			},
			cmd:        Service.Previous,
			wantIndex:  0,
			wantCalls:  []string{b, a},
			wantStarts: []string{b + "@1", a + "@0"},
		},
		{
			name:       "jump",
			setup:      func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			cmd:        func(svc Service) error { return svc.JumpTo(2) },
			wantIndex:  2,
			wantCalls:  []string{a, c},
			wantStarts: []string{a + "@0", c + "@2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, p, sub := newStaleFinishService(t)
			tt.setup(t, svc, p)

			must(t, finishDuringOpen(t, p, func() error { return tt.cmd(svc) }))

			if got := svc.QueueCurrentIndex(); got != tt.wantIndex {
				t.Errorf("queue index = %d, want %d", got, tt.wantIndex)
			}
			if !svc.IsPlaying() {
				t.Errorf("State = %v, want Playing", svc.State())
			}
			if got := p.PlayCalls(); !slices.Equal(got, tt.wantCalls) {
				t.Errorf("PlayCalls = %v, want %v", got, tt.wantCalls)
			}
			if got := starts(sub); !slices.Equal(got, tt.wantStarts) {
				t.Errorf("track starts = %v, want %v", got, tt.wantStarts)
			}
		})
	}
}

// Ignoring a stale finish must not swallow the next real one: the track the
// command started still moves the queue on when it ends.
func TestStaleFinish_TrackStartedByTheCommandStillAdvances(t *testing.T) {
	svc, p, sub := newStaleFinishService(t)
	must(t, svc.Play())
	must(t, finishDuringOpen(t, p, svc.Next))
	starts(sub)

	p.SimulateFinished()
	waitFor(t, "the next track to start", func() bool { return len(p.PlayCalls()) == 3 })

	if got := svc.QueueCurrentIndex(); got != 2 {
		t.Errorf("queue index = %d, want 2", got)
	}
	if got := p.PlayCalls(); !slices.Equal(got, []string{testSvcPathA, testSvcPathB, testSvcPathC}) {
		t.Errorf("PlayCalls = %v", got)
	}
	waitFor(t, "the track start", func() bool { return len(sub.TrackStarted) > 0 })
	if got := starts(sub); !slices.Equal(got, []string{testSvcPathC + "@2"}) {
		t.Errorf("track starts = %v, want [%s@2]", got, testSvcPathC)
	}
}

// A command whose open fails while a finish arrives: the failure is the only
// outcome, and nothing else is opened.
func TestStaleFinish_CommandOpenFails(t *testing.T) {
	svc, p, sub := newStaleFinishService(t)
	must(t, svc.Play())
	p.SetPlayError(errors.New("unreadable"))

	if err := finishDuringOpen(t, p, svc.Next); err == nil {
		t.Fatal("Next succeeded, want the open failure")
	}

	if got := svc.QueueCurrentIndex(); got != 1 {
		t.Errorf("queue index = %d, want 1", got)
	}
	if !svc.IsStopped() {
		t.Errorf("State = %v, want Stopped", svc.State())
	}
	if got := p.PlayCalls(); !slices.Equal(got, []string{testSvcPathA, testSvcPathB}) {
		t.Errorf("PlayCalls = %v, want [a b]", got)
	}
	if n := len(sub.Error); n != 1 {
		t.Errorf("%d errors reported, want 1", n)
	}
}

// A finish handled after Stop belongs to the track Stop ended: playback stays
// stopped where the user stopped it.
func TestStaleFinish_AfterStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p, _ := newStaleFinishService(t)
		must(t, svc.Play())
		must(t, svc.Stop())

		p.SimulateFinished()
		synctest.Wait()

		if got := svc.QueueCurrentIndex(); got != 0 {
			t.Errorf("queue index = %d, want 0", got)
		}
		if !svc.IsStopped() {
			t.Errorf("State = %v, want Stopped", svc.State())
		}
		if got := p.PlayCalls(); !slices.Equal(got, []string{testSvcPathA}) {
			t.Errorf("PlayCalls = %v, want [a]", got)
		}
	})
}

// Seeking past the end stops the player before its finish is signalled: a
// Stop in between, on a player already stopped, still makes that finish stale.
func TestStaleFinish_AfterStopOfAPlayerAlreadyStopped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p, _ := newStaleFinishService(t)
		must(t, svc.Play())
		p.SetState(player.Stopped) // the seek past the end
		must(t, svc.Stop())

		p.SimulateFinished()
		synctest.Wait()

		if got := svc.QueueCurrentIndex(); got != 0 {
			t.Errorf("queue index = %d, want 0", got)
		}
		if got := p.PlayCalls(); !slices.Equal(got, []string{testSvcPathA}) {
			t.Errorf("PlayCalls = %v, want [a]", got)
		}
	})
}

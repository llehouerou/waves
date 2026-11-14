package playback

import (
	"errors"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

const testSvcPathD = "/d.mp3"

// playingService returns a service whose queue holds paths in mode, playing
// the track at current.
func playingService(t *testing.T, mode playlist.RepeatMode, current int, paths ...string) (Service, *player.Mock) {
	t.Helper()
	p := player.NewMock()
	q := playlist.NewQueue()
	for _, path := range paths {
		q.Add(playlist.Track{Path: path})
	}
	q.JumpTo(current)
	q.SetRepeatMode(mode)

	svc := New(p, q)
	t.Cleanup(func() { svc.Close() })
	must(t, svc.Play())
	return svc, p
}

// finish plays the track the player is on to the end.
func finish(p *player.Mock) {
	p.SimulateFinished()
	synctest.Wait()
}

// ranOutService returns a service in radio mode whose queue ran out: B, its
// last track, played to the end.
func ranOutService(t *testing.T) (Service, *player.Mock) {
	t.Helper()
	svc, p := playingService(t, playlist.RepeatRadio, 1, testSvcPathA, testSvcPathB)
	finish(p)
	if !svc.IsStopped() {
		t.Fatal("the last track finished and the player is still running")
	}
	return svc, p
}

func TestResumeAfterRunOut_StartsTheTrackAfterTheOneThatRanOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := ranOutService(t)
		svc.AddTracks(Track{Path: testSvcPathC}, Track{Path: testSvcPathD})

		if err := svc.ResumeAfterRunOut(); err != nil {
			t.Fatal(err)
		}

		if got, want := p.PlayCalls(), []string{testSvcPathB, testSvcPathC}; !slices.Equal(got, want) {
			t.Errorf("PlayCalls() = %v, want %v", got, want)
		}
		if got := svc.QueueCurrentIndex(); got != 2 {
			t.Errorf("QueueCurrentIndex() = %d, want 2", got)
		}
	})
}

// Whatever the client does to playback or to the queue position after the
// queue ran out cancels the resume.
func TestResumeAfterRunOut_CancelledByTheClient(t *testing.T) {
	onlyB := []string{testSvcPathB}
	replayed := []string{testSvcPathB, testSvcPathB}
	tests := []struct {
		name string
		act  func(t *testing.T, svc Service)
		want []string // the player starts
	}{
		{"play", func(t *testing.T, svc Service) { must(t, svc.Play()) }, replayed},
		{"toggle", func(t *testing.T, svc Service) { must(t, svc.Toggle()) }, replayed},
		{"stop", func(t *testing.T, svc Service) { must(t, svc.Stop()) }, onlyB},
		{"jump to", func(t *testing.T, svc Service) { must(t, svc.JumpTo(0)) }, onlyB},
		{"previous", func(t *testing.T, svc Service) { must(t, svc.Previous()) }, onlyB},
		{"next", func(t *testing.T, svc Service) { must(t, svc.Next()) }, onlyB},
		{"queue move to", func(_ *testing.T, svc Service) { svc.QueueMoveTo(0) }, onlyB},
		{"queue advance", func(_ *testing.T, svc Service) { svc.QueueAdvance() }, onlyB},
		{"replace", func(_ *testing.T, svc Service) {
			svc.ReplaceTracks(Track{Path: testSvcPathC}, Track{Path: testSvcPathD})
		}, onlyB},
		{"clear", func(_ *testing.T, svc Service) {
			svc.ClearQueue()
			svc.AddTracks(Track{Path: testSvcPathC})
		}, onlyB},
		{"radio off", func(_ *testing.T, svc Service) { svc.SetRepeatMode(RepeatOff) }, onlyB},
		{"remove the track that ran out", func(_ *testing.T, svc Service) { svc.RemoveTracks([]int{1}) }, onlyB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				svc, p := ranOutService(t)
				svc.AddTracks(Track{Path: testSvcPathC}, Track{Path: testSvcPathD})
				tt.act(t, svc)

				if err := svc.ResumeAfterRunOut(); err != nil {
					t.Fatal(err)
				}

				if got := p.PlayCalls(); !slices.Equal(got, tt.want) {
					t.Errorf("PlayCalls() = %v, want %v", got, tt.want)
				}
			})
		})
	}
}

// A client start cancels the resume, even one whose track fails to open and
// leaves the player stopped.
func TestResumeAfterRunOut_CancelledByAFailedPlay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := ranOutService(t)
		svc.AddTracks(Track{Path: testSvcPathC})
		p.SetPlayError(errors.New("open failed"))
		if err := svc.Play(); err == nil {
			t.Fatal("Play() = nil, want the open error")
		}
		p.SetPlayError(nil)

		must(t, svc.ResumeAfterRunOut())

		if got, want := p.PlayCalls(), []string{testSvcPathB, testSvcPathB}; !slices.Equal(got, want) {
			t.Errorf("PlayCalls() = %v, want %v", got, want)
		}
	})
}

// Next with nothing after it is a run-out only when the player was active.
func TestResumeAfterRunOut_NextWhileStoppedIsNoRunOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA})
		q.JumpTo(0)
		q.SetRepeatMode(playlist.RepeatRadio)
		svc := New(p, q)
		defer svc.Close()

		must(t, svc.Next())
		svc.AddTracks(Track{Path: testSvcPathB})
		must(t, svc.ResumeAfterRunOut())

		if got := p.PlayCalls(); len(got) != 0 {
			t.Errorf("PlayCalls() = %v, want none", got)
		}
	})
}

// A track that fails to open stops the player: that is no run-out either.
func TestResumeAfterRunOut_FailedOpenIsNoRunOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := player.NewMock()
		q := playlist.NewQueue()
		q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
		q.JumpTo(0)
		q.SetRepeatMode(playlist.RepeatRadio)
		svc := New(p, q)
		defer svc.Close()
		must(t, svc.Play())

		p.SetPlayError(errors.New("open failed"))
		p.SimulateFinished()
		synctest.Wait()
		p.SetPlayError(nil)
		svc.AddTracks(Track{Path: testSvcPathC})
		must(t, svc.ResumeAfterRunOut())

		if got, want := p.PlayCalls(), []string{testSvcPathA, testSvcPathB}; !slices.Equal(got, want) {
			t.Errorf("PlayCalls() = %v, want %v", got, want)
		}
	})
}

// A queue cleared while its last track played has no current track when the
// fill lands: the resume starts the first track of the fill.
func TestResumeAfterRunOut_ClearedWhilePlayingStartsTheFirstOfTheFill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := playingService(t, playlist.RepeatRadio, 1, testSvcPathA, testSvcPathB)
		svc.ClearQueue()
		finish(p)
		if !svc.IsStopped() {
			t.Fatal("the last track finished and the player is still running")
		}

		svc.AddTracks(Track{Path: testSvcPathC}, Track{Path: testSvcPathD})
		must(t, svc.ResumeAfterRunOut())

		if got, want := p.PlayCalls(), []string{testSvcPathB, testSvcPathC}; !slices.Equal(got, want) {
			t.Errorf("PlayCalls() = %v, want %v", got, want)
		}
		if got := svc.QueueCurrentIndex(); got != 0 {
			t.Errorf("QueueCurrentIndex() = %d, want 0", got)
		}
	})
}

// A fill that lands on a cleared queue before its last track ends is played
// when that track ends: it starts the first track of the fill.
func TestTrackFinished_ClearedQueueRefilledBeforeTheEndStartsTheFirstOfTheFill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := playingService(t, playlist.RepeatRadio, 1, testSvcPathA, testSvcPathB)
		svc.ClearQueue()
		svc.AddTracks(Track{Path: testSvcPathC}, Track{Path: testSvcPathD})
		finish(p)

		if got, want := p.PlayCalls(), []string{testSvcPathB, testSvcPathC}; !slices.Equal(got, want) {
			t.Errorf("PlayCalls() = %v, want %v", got, want)
		}
		if got := svc.QueueCurrentIndex(); got != 0 {
			t.Errorf("QueueCurrentIndex() = %d, want 0", got)
		}
	})
}

// Removing the track that plays does not run the queue out, whether tracks
// remain or not, when it ends or on Next (MPRIS Next while it still plays):
// playback stops, and the fill that lands afterwards has nothing to resume
// (#166 in radio mode).
func TestResumeAfterRunOut_RemovingTheCurrentTrackIsNoRunOut(t *testing.T) {
	queues := []struct {
		name    string
		paths   []string
		current int
	}{
		{"tracks remain", []string{testSvcPathA, testSvcPathB, testSvcPathD}, 1},
		{"queue emptied", []string{testSvcPathB}, 0},
	}
	ends := []struct {
		name string
		end  func(t *testing.T, svc Service, p *player.Mock)
	}{
		{"finish", func(_ *testing.T, _ Service, p *player.Mock) { finish(p) }},
		{"next", func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Next()) }},
	}
	for _, q := range queues {
		for _, e := range ends {
			t.Run(q.name+"/"+e.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					svc, p := playingService(t, playlist.RepeatRadio, q.current, q.paths...)
					svc.RemoveTracks([]int{q.current})
					e.end(t, svc, p)
					if !svc.IsStopped() {
						t.Fatal("the removed track ended and the player is still running")
					}

					svc.AddTracks(Track{Path: testSvcPathC}) // the radio fill lands
					must(t, svc.ResumeAfterRunOut())

					if got, want := p.PlayCalls(), []string{testSvcPathB}; !slices.Equal(got, want) {
						t.Errorf("PlayCalls() = %v, want %v", got, want)
					}
					if got := svc.QueueCurrentIndex(); got != -1 {
						t.Errorf("QueueCurrentIndex() = %d, want -1", got)
					}
				})
			})
		}
	}
}

// Removing the current track ends in a stop: the finish of the removed track
// starts nothing, even though the queue still holds the tracks after it (#166).
func TestRemoveTracks_RemovedCurrentTrackFinishesInAStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := playingService(t, playlist.RepeatOff, 1, testSvcPathA, testSvcPathB, testSvcPathD)
		svc.RemoveTracks([]int{1})
		finish(p)

		if !svc.IsStopped() {
			t.Error("the removed track finished and the player is still running")
		}
		if got, want := p.PlayCalls(), []string{testSvcPathB}; !slices.Equal(got, want) {
			t.Errorf("PlayCalls() = %v, want %v", got, want)
		}
		if got := svc.QueueCurrentIndex(); got != -1 {
			t.Errorf("QueueCurrentIndex() = %d, want -1", got)
		}
	})
}

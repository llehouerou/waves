package playback

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// starts drains the track starts the subscription has received, as "path@index".
func starts(sub *Subscription) []string {
	var got []string
	for {
		select {
		case e := <-sub.TrackStarted:
			got = append(got, fmt.Sprintf("%s@%d", e.Track.Path, e.Index))
		default:
			return got
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Every start of a track is reported once, and nothing else is: a resume, a
// move of the queue position, or a track that does not open.
func TestTrackStart_OncePerStart(t *testing.T) {
	a, b, c := testSvcPathA+"@0", testSvcPathB+"@1", testSvcPathC+"@2"
	tests := []struct {
		name   string
		repeat playlist.RepeatMode
		setup  func(t *testing.T, svc Service, p *player.Mock) // before subscribing
		act    func(t *testing.T, svc Service, p *player.Mock)
		want   []string
	}{
		{
			name: "first play",
			act:  func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			want: []string{a},
		},
		{
			name:  "play on the track playing",
			setup: func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act:   func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			want:  []string{a},
		},
		{
			name: "play after a stop on another track",
			setup: func(t *testing.T, svc Service, _ *player.Mock) {
				must(t, svc.Play())
				must(t, svc.Stop())
				svc.QueueMoveTo(1)
			},
			act:  func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			want: []string{b},
		},
		{
			name: "toggle from stopped",
			act:  func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Toggle()) },
			want: []string{a},
		},
		{
			name: "toggle from paused",
			setup: func(t *testing.T, svc Service, _ *player.Mock) {
				must(t, svc.Play())
				must(t, svc.Pause())
			},
			act: func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Toggle()) },
		},
		{
			name:  "next while playing",
			setup: func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act:   func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Next()) },
			want:  []string{b},
		},
		{
			name: "previous while paused",
			setup: func(t *testing.T, svc Service, _ *player.Mock) {
				must(t, svc.JumpTo(1))
				must(t, svc.Play())
				must(t, svc.Pause())
			},
			act:  func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Previous()) },
			want: []string{a},
		},
		{
			name:  "jump while playing",
			setup: func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act:   func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.JumpTo(2)) },
			want:  []string{c},
		},
		{
			name: "next, previous and jump while stopped",
			act: func(t *testing.T, svc Service, _ *player.Mock) {
				must(t, svc.Next())
				must(t, svc.Previous())
				must(t, svc.JumpTo(2))
			},
		},
		{
			name: "queue moves",
			act: func(_ *testing.T, svc Service, _ *player.Mock) {
				svc.QueueAdvance()
				svc.QueueMoveTo(2)
			},
		},
		{
			name:  "a track finishes",
			setup: func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act:   func(_ *testing.T, _ Service, p *player.Mock) { p.SimulateFinished() },
			want:  []string{b},
		},
		{
			name:  "the player starts the next track itself",
			setup: func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act:   func(_ *testing.T, _ Service, p *player.Mock) { p.SimulateGaplessSwitch(testSvcPathB) },
			want:  []string{b},
		},
		{
			name:   "repeat-one restarts the track",
			repeat: playlist.RepeatOne,
			setup:  func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act:    func(_ *testing.T, _ Service, p *player.Mock) { p.SimulateFinished() },
			want:   []string{a},
		},
		{
			name:   "repeat-one, the player restarts the track itself",
			repeat: playlist.RepeatOne,
			setup:  func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act:    func(_ *testing.T, _ Service, p *player.Mock) { p.SimulateGaplessSwitch(testSvcPathA) },
			want:   []string{a},
		},
		{
			name: "play fails to open",
			act: func(t *testing.T, svc Service, p *player.Mock) {
				p.SetPlayError(errors.New("unreadable"))
				if svc.Play() == nil {
					t.Fatal("Play succeeded, want an error")
				}
			},
		},
		{
			name:  "next fails to open",
			setup: func(t *testing.T, svc Service, _ *player.Mock) { must(t, svc.Play()) },
			act: func(t *testing.T, svc Service, p *player.Mock) {
				p.SetPlayError(errors.New("unreadable"))
				if svc.Next() == nil {
					t.Fatal("Next succeeded, want an error")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				p := player.NewMock()
				q := playlist.NewQueue()
				q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB}, playlist.Track{Path: testSvcPathC})
				q.JumpTo(0)
				q.SetRepeatMode(tt.repeat)
				svc := New(p, q)
				defer svc.Close()
				if tt.setup != nil {
					tt.setup(t, svc, p)
					synctest.Wait()
				}

				sub := svc.Subscribe()
				tt.act(t, svc, p)
				synctest.Wait()

				if got := starts(sub); !slices.Equal(got, tt.want) {
					t.Errorf("track starts = %v, want %v", got, tt.want)
				}
			})
		})
	}
}

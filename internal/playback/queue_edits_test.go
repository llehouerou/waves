package playback

import (
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// newEditTestService returns a service on a queue holding paths, with the
// queue on index current.
func newEditTestService(t *testing.T, current int, paths ...string) Service {
	t.Helper()
	svc, _ := newEditTestServiceWithPlayer(t, current, paths...)
	return svc
}

func newEditTestServiceWithPlayer(t *testing.T, current int, paths ...string) (Service, *player.Mock) {
	t.Helper()
	q := playlist.NewQueue()
	for _, path := range paths {
		q.Add(playlist.Track{Path: path})
	}
	q.ClearHistory()
	q.JumpTo(current)
	p := player.NewMock()
	svc := New(p, q)
	t.Cleanup(func() { _ = svc.Close() })
	return svc, p
}

func TestNew_InstallsPreloadOfNextQueueTrack(t *testing.T) {
	svc, p := newEditTestServiceWithPlayer(t, 0, testSvcPathA, testSvcPathB)

	preload := p.PreloadFunc()
	if preload == nil {
		t.Fatal("New did not install a preload func on the player")
	}
	if got := preload(); got != testSvcPathB {
		t.Errorf("preload() = %q, want %q", got, testSvcPathB)
	}
	svc.QueueMoveTo(1)
	if got := preload(); got != "" {
		t.Errorf("preload() at the last track = %q, want none", got)
	}
}

// Anything that changes which track comes next must drop the preloaded one,
// or gapless playback starts a track the queue no longer leads to.
func TestNextTrackChanges_ClearPreload(t *testing.T) {
	removeSecond := func(s Service) { s.RemoveTracks([]int{1}) }
	changes := map[string]struct{ setup, change func(Service) }{
		"add":     {nil, func(s Service) { s.AddTracks(Track{Path: testSvcPathC}) }},
		"replace": {nil, func(s Service) { s.ReplaceTracks(Track{Path: testSvcPathC}) }},
		"remove":  {nil, removeSecond},
		"move":    {nil, func(s Service) { s.MoveTracks([]int{1}, -1) }},
		"clear":   {nil, func(s Service) { s.ClearQueue() }},
		"undo":    {removeSecond, func(s Service) { s.Undo() }},
		"redo":    {func(s Service) { removeSecond(s); s.Undo() }, func(s Service) { s.Redo() }},
		"shuffle": {nil, func(s Service) { s.ToggleShuffle() }},
		"repeat":  {nil, func(s Service) { s.CycleRepeatMode() }},
	}
	for name, c := range changes {
		t.Run(name, func(t *testing.T) {
			svc, p := newEditTestServiceWithPlayer(t, 0, testSvcPathA, testSvcPathB)
			if c.setup != nil {
				c.setup(svc)
			}
			before := p.PreloadClears()

			c.change(svc)

			if p.PreloadClears() == before {
				t.Error("preloaded track was not cleared")
			}
		})
	}
}

func queuePaths(svc Service) []string {
	tracks := svc.QueueTracks()
	paths := make([]string, len(tracks))
	for i, t := range tracks {
		paths[i] = t.Path
	}
	return paths
}

func TestRemoveTracks_IsOneQueueEdit(t *testing.T) {
	svc := newEditTestService(t, 0, testSvcPathA, testSvcPathB, testSvcPathC)
	sub := svc.Subscribe()

	svc.RemoveTracks([]int{2, 0})

	if got := queuePaths(svc); !slices.Equal(got, []string{testSvcPathB}) {
		t.Fatalf("queue = %v, want [%s]", got, testSvcPathB)
	}
	select {
	case <-sub.QueueChanged:
	default:
		t.Fatal("RemoveTracks did not emit QueueChange")
	}
	if !svc.Undo() {
		t.Fatal("Undo failed")
	}
	if got := queuePaths(svc); !slices.Equal(got, []string{testSvcPathA, testSvcPathB, testSvcPathC}) {
		t.Fatalf("after one undo, queue = %v, want all three tracks back", got)
	}
}

func TestMoveTracks_IsOneQueueEdit(t *testing.T) {
	svc := newEditTestService(t, 0, testSvcPathA, testSvcPathB, testSvcPathC)
	sub := svc.Subscribe()

	svc.MoveTracks([]int{0, 1}, 1)

	if got := queuePaths(svc); !slices.Equal(got, []string{testSvcPathC, testSvcPathA, testSvcPathB}) {
		t.Fatalf("queue = %v, want [C A B]", got)
	}
	if got := svc.QueueCurrentIndex(); got != 1 {
		t.Errorf("current index = %d, want 1 (A moved with the block)", got)
	}
	select {
	case <-sub.QueueChanged:
	default:
		t.Fatal("MoveTracks did not emit QueueChange")
	}
	if !svc.Undo() {
		t.Fatal("Undo failed")
	}
	if got := queuePaths(svc); !slices.Equal(got, []string{testSvcPathA, testSvcPathB, testSvcPathC}) {
		t.Fatalf("after one undo, queue = %v, want original order", got)
	}
}

func TestMoveTracks_OutOfBoundsIsNoEdit(t *testing.T) {
	svc := newEditTestService(t, 0, testSvcPathA, testSvcPathB)
	sub := svc.Subscribe()

	svc.MoveTracks([]int{0}, -1)

	select {
	case <-sub.QueueChanged:
		t.Fatal("a move that cannot happen emitted QueueChange")
	default:
	}
	if svc.Undo() {
		t.Fatal("a move that cannot happen left an undo step")
	}
}

// Play decides whether the track changed by comparing the queue position to
// Last-played. An edit that shifts the playing track's index must shift
// Last-played with it, or replaying the same track reads as a track change.
func TestQueueEdits_ShiftLastPlayed(t *testing.T) {
	tests := []struct {
		name    string
		current int
		edit    func(Service)
	}{
		{"remove above", 2, func(s Service) { s.RemoveTracks([]int{0}) }},
		{"move above down past it", 2, func(s Service) { s.MoveTracks([]int{0}, 2) }},
		{"move it", 0, func(s Service) { s.MoveTracks([]int{0}, 1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newEditTestService(t, tt.current, testSvcPathA, testSvcPathB, testSvcPathC)
			sub := svc.Subscribe()
			if err := svc.Play(); err != nil {
				t.Fatal(err)
			}

			tt.edit(svc)
			if err := svc.Play(); err != nil {
				t.Fatal(err)
			}

			select {
			case e := <-sub.TrackChanged:
				t.Fatalf("replaying the same track emitted TrackChange %+v", e)
			default:
			}
		})
	}
}

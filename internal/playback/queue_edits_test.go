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
	q := playlist.NewQueue()
	for _, path := range paths {
		q.Add(playlist.Track{Path: path})
	}
	q.ClearHistory()
	q.JumpTo(current)
	svc := New(player.NewMock(), q)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
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

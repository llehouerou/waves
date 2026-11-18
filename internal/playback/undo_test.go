package playback

import (
	"slices"
	"testing"
	"testing/synctest"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

const (
	testSvcPathX = "/x.mp3"
	testSvcPathY = "/y.mp3"
)

// historyService returns a service whose queue holds paths in mode, with no
// undo step, playing the track at current.
func historyService(t *testing.T, mode playlist.RepeatMode, current int, paths ...string) (Service, *player.Mock) {
	t.Helper()
	p := player.NewMock()
	q := playlist.NewQueue()
	for _, path := range paths {
		q.Add(playlist.Track{Path: path})
	}
	q.ClearHistory()
	q.JumpTo(current)
	q.SetRepeatMode(mode)

	svc := New(p, q)
	t.Cleanup(func() { svc.Close() })
	must(t, svc.Play())
	return svc, p
}

// expectOnlyQueueChanges checks that sub got n QueueChange, the last one on
// index, and nothing that says the track or the state changed.
func expectOnlyQueueChanges(t *testing.T, sub *Subscription, n, index int) {
	t.Helper()
	got := 0
	last := QueueChange{Index: -2}
	for done := false; !done; {
		select {
		case e := <-sub.QueueChanged:
			got++
			last = e
		default:
			done = true
		}
	}
	if got != n {
		t.Errorf("got %d QueueChange, want %d", got, n)
	}
	if last.Index != index {
		t.Errorf("QueueChange.Index = %d, want %d", last.Index, index)
	}
	select {
	case e := <-sub.TrackChanged:
		t.Errorf("unexpected TrackChange %+v", e)
	case e := <-sub.TrackStarted:
		t.Errorf("unexpected TrackStart %+v", e)
	case e := <-sub.StateChanged:
		t.Errorf("unexpected StateChange %+v", e)
	default:
	}
}

// An undo or a redo keeps the queue on the entry it was on, wherever the
// restored list puts it, and on none when the list does not hold it: the track
// that follows the one playing is the one after that entry, or none at all.
// An entry an edit took out while it was current is current again when an
// undo or a redo brings it back.
func TestUndoRedo_QueueStaysOnTheCurrentEntry(t *testing.T) {
	undo := func(s Service) bool { return s.Undo() }
	redo := func(s Service) bool { return s.Redo() }
	tests := []struct {
		name    string
		paths   []string
		current int
		edit    func(t *testing.T, s Service)
		walk    []func(Service) bool
		// wantQueue and wantIndex are the queue after the walk; wantPlays,
		// every track the player started once the track playing ended.
		wantQueue []string
		wantIndex int
		wantPlays []string
	}{
		{
			name: "undo of a removal above it", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC}, current: 2,
			edit:      func(_ *testing.T, s Service) { s.RemoveTracks([]int{0}) },
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathC}, wantIndex: 2,
			wantPlays: []string{testSvcPathC},
		},
		{
			name: "undo of a move of it", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC}, current: 0,
			edit:      func(_ *testing.T, s Service) { s.MoveTracks([]int{0}, 1) },
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathC}, wantIndex: 0,
			wantPlays: []string{testSvcPathA, testSvcPathB},
		},
		{
			name: "redo of a removal above it", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC, testSvcPathD}, current: 0,
			edit: func(t *testing.T, s Service) {
				s.RemoveTracks([]int{0})
				s.Undo()
				must(t, s.JumpTo(1))
			},
			walk:      []func(Service) bool{redo},
			wantQueue: []string{testSvcPathB, testSvcPathC, testSvcPathD}, wantIndex: 0,
			wantPlays: []string{testSvcPathA, testSvcPathB, testSvcPathC},
		},
		{
			name: "undo of its removal", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC}, current: 1,
			edit:      func(_ *testing.T, s Service) { s.RemoveTracks([]int{1}) },
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathC}, wantIndex: 1,
			wantPlays: []string{testSvcPathB, testSvcPathC},
		},
		{
			name: "redo of its removal", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC}, current: 1,
			edit:      func(_ *testing.T, s Service) { s.RemoveTracks([]int{1}) },
			walk:      []func(Service) bool{undo, redo},
			wantQueue: []string{testSvcPathA, testSvcPathC}, wantIndex: -1,
			wantPlays: []string{testSvcPathB},
		},
		{
			name: "undo of a clear", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC}, current: 1,
			edit:      func(_ *testing.T, s Service) { s.ClearQueue() },
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathC}, wantIndex: 1,
			wantPlays: []string{testSvcPathB, testSvcPathC},
		},
		{
			name: "duplicated, the second one playing", paths: []string{testSvcPathA, testSvcPathB, testSvcPathA}, current: 2,
			edit:      func(_ *testing.T, s Service) { s.RemoveTracks([]int{1}) },
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathA}, wantIndex: 2,
			wantPlays: []string{testSvcPathA},
		},
		{
			name: "duplicated, the first one playing", paths: []string{testSvcPathA, testSvcPathB, testSvcPathA}, current: 0,
			edit:      func(_ *testing.T, s Service) { s.RemoveTracks([]int{1}) },
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathA}, wantIndex: 0,
			wantPlays: []string{testSvcPathA, testSvcPathB},
		},
		{
			name: "undo of the add that brought it", paths: []string{testSvcPathA, testSvcPathB}, current: 0,
			edit: func(t *testing.T, s Service) {
				s.AddTracks(Track{Path: testSvcPathC})
				must(t, s.JumpTo(2))
			},
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB}, wantIndex: -1,
			wantPlays: []string{testSvcPathA, testSvcPathC},
		},
		{
			name: "redo of the add that brought it", paths: []string{testSvcPathA, testSvcPathB}, current: 0,
			edit: func(t *testing.T, s Service) {
				s.AddTracks(Track{Path: testSvcPathC})
				must(t, s.JumpTo(2))
			},
			walk:      []func(Service) bool{undo, redo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathC}, wantIndex: 2,
			wantPlays: []string{testSvcPathA, testSvcPathC},
		},
		{
			name: "undo of a replace", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC}, current: 1,
			edit: func(t *testing.T, s Service) {
				s.ReplaceTracks(Track{Path: testSvcPathX}, Track{Path: testSvcPathY})
				must(t, s.Play())
			},
			walk:      []func(Service) bool{undo},
			wantQueue: []string{testSvcPathA, testSvcPathB, testSvcPathC}, wantIndex: -1,
			wantPlays: []string{testSvcPathB, testSvcPathX},
		},
		{
			name: "redo of a replace", paths: []string{testSvcPathA, testSvcPathB, testSvcPathC}, current: 1,
			edit: func(t *testing.T, s Service) {
				s.ReplaceTracks(Track{Path: testSvcPathX}, Track{Path: testSvcPathY})
				must(t, s.Play())
			},
			walk:      []func(Service) bool{undo, redo},
			wantQueue: []string{testSvcPathX, testSvcPathY}, wantIndex: 0,
			wantPlays: []string{testSvcPathB, testSvcPathX, testSvcPathY},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				svc, p := historyService(t, playlist.RepeatOff, tt.current, tt.paths...)
				tt.edit(t, svc)
				plays := p.PlayCalls()
				sub := svc.Subscribe()

				for i, walk := range tt.walk {
					if !walk(svc) {
						t.Fatalf("step %d: undo/redo refused", i)
					}
				}

				if got := queuePaths(svc); !slices.Equal(got, tt.wantQueue) {
					t.Fatalf("queue = %v, want %v", got, tt.wantQueue)
				}
				if got := svc.QueueCurrentIndex(); got != tt.wantIndex {
					t.Fatalf("QueueCurrentIndex() = %d, want %d", got, tt.wantIndex)
				}
				if playing := plays[len(plays)-1]; tt.wantIndex >= 0 && svc.CurrentTrack().Path != playing {
					t.Errorf("CurrentTrack() = %s, want %s, the track playing", svc.CurrentTrack().Path, playing)
				}
				if tt.wantIndex < 0 && (svc.CurrentTrack() != nil || svc.QueueHasNext() || svc.QueuePeekNext() != nil) {
					t.Error("a queue with no current track has a current or a next track")
				}
				expectOnlyQueueChanges(t, sub, len(tt.walk), tt.wantIndex)
				if got := p.PlayCalls(); !slices.Equal(got, plays) || !svc.IsPlaying() {
					t.Errorf("undo/redo touched the player: PlayCalls() = %v, playing %v", got, svc.IsPlaying())
				}

				finish(p)

				if got := p.PlayCalls(); !slices.Equal(got, tt.wantPlays) {
					t.Errorf("once the track ended, PlayCalls() = %v, want %v", got, tt.wantPlays)
				}
				if stopped := !svc.IsPlaying(); stopped != slices.Equal(plays, tt.wantPlays) {
					t.Errorf("once the track ended, stopped = %v", stopped)
				}
			})
		})
	}
}

// The removed track that ended is current again when its removal is undone:
// Play starts it.
func TestUndo_RemovalOfATrackThatEndedSinceMakesItCurrent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, p := historyService(t, playlist.RepeatOff, 1, testSvcPathA, testSvcPathB, testSvcPathC)
		svc.RemoveTracks([]int{1})
		finish(p)
		if !svc.IsStopped() {
			t.Fatal("the removed track ended and the player is still running")
		}

		svc.Undo()

		if got := svc.QueueCurrentIndex(); got != 1 {
			t.Fatalf("QueueCurrentIndex() = %d, want 1", got)
		}
		must(t, svc.Play())
		if got, want := p.PlayCalls(), []string{testSvcPathB, testSvcPathB}; !slices.Equal(got, want) {
			t.Errorf("PlayCalls() = %v, want %v", got, want)
		}
	})
}

// In radio mode, an undo that empties the queue under the track playing
// leaves it as a clear does, whether that undo took the track out or an
// earlier one did: that track ends in a run-out, and the fill that lands
// afterwards starts. One that leaves tracks leaves it as a removal of the
// current track does: the fill starts nothing.
func TestUndo_TakingTheCurrentTrackOutInRadioMode(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		// adds are queue edits, each an AddTracks; the last track added is
		// then played, and undos undo that many edits.
		adds      [][]Track
		undos     int
		wantQueue []string
		wantPlays []string
	}{
		{"queue emptied", nil, [][]Track{{{Path: testSvcPathA}}}, 1,
			[]string{}, []string{testSvcPathA, testSvcPathX}},
		{"queue emptied by a later undo", nil,
			[][]Track{{{Path: testSvcPathA}, {Path: testSvcPathB}}, {{Path: testSvcPathC}}}, 2,
			[]string{}, []string{testSvcPathC, testSvcPathX}},
		{"tracks remain", []string{testSvcPathA, testSvcPathB}, [][]Track{{{Path: testSvcPathC}}}, 1,
			[]string{testSvcPathA, testSvcPathB}, []string{testSvcPathA, testSvcPathC}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				p := player.NewMock()
				q := playlist.NewQueue()
				for _, path := range tt.paths {
					q.Add(playlist.Track{Path: path})
				}
				q.ClearHistory()
				q.SetRepeatMode(playlist.RepeatRadio)
				svc := New(p, q)
				t.Cleanup(func() { svc.Close() })
				if len(tt.paths) > 0 {
					q.JumpTo(0)
					must(t, svc.Play())
				}
				for _, add := range tt.adds {
					svc.AddTracks(add...)
				}
				must(t, svc.JumpTo(svc.QueueLen()-1))
				if !svc.IsPlaying() {
					must(t, svc.Play())
				}

				for range tt.undos {
					svc.Undo()
				}
				if got := queuePaths(svc); !slices.Equal(got, tt.wantQueue) {
					t.Fatalf("queue = %v, want %v", got, tt.wantQueue)
				}
				if got := svc.QueueCurrentIndex(); got != -1 {
					t.Fatalf("QueueCurrentIndex() = %d, want -1", got)
				}
				finish(p)
				svc.AddTracks(Track{Path: testSvcPathX}) // the radio fill lands
				must(t, svc.ResumeAfterRunOut())

				if got := p.PlayCalls(); !slices.Equal(got, tt.wantPlays) {
					t.Errorf("PlayCalls() = %v, want %v", got, tt.wantPlays)
				}
			})
		})
	}
}

// A track queued twice is two entries, and each row keeps its entry through
// an undo: a client can follow an entry by its ID.
func TestQueueEntries_KeepTheirIDThroughUndo(t *testing.T) {
	q := playlist.NewQueue()
	svc := New(player.NewMock(), q)
	t.Cleanup(func() { svc.Close() })
	svc.AddTracks(Track{Path: "/a"}, Track{Path: "/b"}, Track{Path: "/a"})

	ids := func() []EntryID {
		entries := svc.QueueEntries()
		ids := make([]EntryID, len(entries))
		for i, e := range entries {
			ids[i] = e.ID
		}
		return ids
	}
	before := ids()
	if len(before) != 3 || before[0] == 0 || before[1] == 0 || before[2] == 0 ||
		before[0] == before[1] || before[0] == before[2] || before[1] == before[2] {
		t.Fatalf("entries of a, b, a = %v, want three distinct non-zero IDs", before)
	}
	if paths := svc.QueueEntries(); paths[0].Track.Path != "/a" || paths[1].Track.Path != "/b" || paths[2].Track.Path != "/a" {
		t.Errorf("entries = %v, want the tracks a, b, a", paths)
	}

	svc.RemoveTracks([]int{0})
	if got := ids(); !slices.Equal(got, before[1:]) {
		t.Errorf("entries after removing the first = %v, want %v", got, before[1:])
	}
	if !svc.Undo() {
		t.Fatal("nothing to undo")
	}
	if got := ids(); !slices.Equal(got, before) {
		t.Errorf("entries after the undo = %v, want %v", got, before)
	}
}

// internal/playlist/queue_test.go
//
//nolint:goconst // test file with repeated string literals
package playlist

import "testing"

func TestNewQueue(t *testing.T) {
	q := NewQueue()

	if q.Len() != 0 {
		t.Errorf("Len() = %d, want 0", q.Len())
	}
	if q.CurrentIndex() != -1 {
		t.Errorf("CurrentIndex() = %d, want -1", q.CurrentIndex())
	}
	if q.Current() != nil {
		t.Error("Current() should be nil for empty queue")
	}
}

func TestQueue_Add(t *testing.T) {
	q := NewQueue()

	q.Add(Track{Path: "/track1.mp3"}, Track{Path: "/track2.mp3"})

	if q.Len() != 2 {
		t.Errorf("Len() = %d, want 2", q.Len())
	}
	// Add doesn't change current index
	if q.CurrentIndex() != -1 {
		t.Errorf("CurrentIndex() = %d, want -1 (unchanged)", q.CurrentIndex())
	}
}

func TestQueue_AddAndPlay(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/existing.mp3"})

	track := q.AddAndPlay(Track{Path: "/new1.mp3"}, Track{Path: "/new2.mp3"})

	if q.Len() != 3 {
		t.Errorf("Len() = %d, want 3", q.Len())
	}
	if q.CurrentIndex() != 1 {
		t.Errorf("CurrentIndex() = %d, want 1", q.CurrentIndex())
	}
	if track == nil || track.Path != "/new1.mp3" {
		t.Errorf("returned track = %v, want /new1.mp3", track)
	}
}

func TestQueue_AddAndPlay_Empty(t *testing.T) {
	q := NewQueue()

	track := q.AddAndPlay()

	if track != nil {
		t.Error("AddAndPlay with no tracks should return nil")
	}
}

func TestQueue_Replace(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/old1.mp3"}, Track{Path: "/old2.mp3"})
	q.JumpTo(1)

	track := q.Replace(Track{Path: "/new.mp3"})

	if q.Len() != 1 {
		t.Errorf("Len() = %d, want 1", q.Len())
	}
	if q.CurrentIndex() != 0 {
		t.Errorf("CurrentIndex() = %d, want 0", q.CurrentIndex())
	}
	if track == nil || track.Path != "/new.mp3" {
		t.Errorf("returned track = %v, want /new.mp3", track)
	}
}

func TestQueue_Replace_Empty(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/old.mp3"})

	track := q.Replace()

	if q.Len() != 0 {
		t.Errorf("Len() = %d, want 0", q.Len())
	}
	if track != nil {
		t.Error("Replace with no tracks should return nil")
	}
}

func TestQueue_JumpTo(t *testing.T) {
	q := NewQueue()
	q.Add(
		Track{Path: "/track0.mp3"},
		Track{Path: "/track1.mp3"},
		Track{Path: "/track2.mp3"},
	)

	track := q.JumpTo(1)

	if q.CurrentIndex() != 1 {
		t.Errorf("CurrentIndex() = %d, want 1", q.CurrentIndex())
	}
	if track == nil || track.Path != "/track1.mp3" {
		t.Errorf("JumpTo returned %v, want /track1.mp3", track)
	}
}

func TestQueue_JumpTo_Invalid(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/track.mp3"})

	track := q.JumpTo(5)

	if track != nil {
		t.Error("JumpTo with invalid index should return nil")
	}
}

func TestQueue_Next_Normal(t *testing.T) {
	q := NewQueue()
	q.Add(
		Track{Path: "/track0.mp3"},
		Track{Path: "/track1.mp3"},
		Track{Path: "/track2.mp3"},
	)
	q.JumpTo(0)

	track := q.Next()

	if q.CurrentIndex() != 1 {
		t.Errorf("CurrentIndex() = %d, want 1", q.CurrentIndex())
	}
	if track == nil || track.Path != "/track1.mp3" {
		t.Errorf("Next() = %v, want /track1.mp3", track)
	}
}

func TestQueue_Next_AtEnd_NoRepeat(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/track0.mp3"}, Track{Path: "/track1.mp3"})
	q.JumpTo(1) // at last track

	track := q.Next()

	if track != nil {
		t.Error("Next() at end with RepeatOff should return nil")
	}
}

func TestQueue_Next_AtEnd_RepeatAll(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/track0.mp3"}, Track{Path: "/track1.mp3"})
	q.JumpTo(1)
	q.SetRepeatMode(RepeatAll)

	track := q.Next()

	if q.CurrentIndex() != 0 {
		t.Errorf("CurrentIndex() = %d, want 0 (wrapped)", q.CurrentIndex())
	}
	if track == nil || track.Path != "/track0.mp3" {
		t.Errorf("Next() = %v, want /track0.mp3", track)
	}
}

func TestQueue_Next_RepeatOne(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/track0.mp3"}, Track{Path: "/track1.mp3"})
	q.JumpTo(0)
	q.SetRepeatMode(RepeatOne)

	track := q.Next()

	if q.CurrentIndex() != 0 {
		t.Errorf("CurrentIndex() = %d, want 0 (same track)", q.CurrentIndex())
	}
	if track == nil || track.Path != "/track0.mp3" {
		t.Errorf("Next() = %v, want /track0.mp3", track)
	}
}

func TestQueue_HasNext(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*PlayingQueue)
		wantHas bool
	}{
		{
			name:    "empty queue",
			setup:   func(_ *PlayingQueue) {},
			wantHas: false,
		},
		{
			name: "no current track with tracks in queue",
			setup: func(q *PlayingQueue) {
				q.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})
				// currentIndex is -1, no track is currently playing
			},
			wantHas: false,
		},
		{
			name: "at start",
			setup: func(q *PlayingQueue) {
				q.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})
				q.JumpTo(0)
			},
			wantHas: true,
		},
		{
			name: "at end no repeat",
			setup: func(q *PlayingQueue) {
				q.Add(Track{Path: "/a.mp3"})
				q.JumpTo(0)
			},
			wantHas: false,
		},
		{
			name: "at end with repeat all",
			setup: func(q *PlayingQueue) {
				q.Add(Track{Path: "/a.mp3"})
				q.JumpTo(0)
				q.SetRepeatMode(RepeatAll)
			},
			wantHas: true,
		},
		{
			name: "with shuffle",
			setup: func(q *PlayingQueue) {
				q.Add(Track{Path: "/a.mp3"})
				q.JumpTo(0)
				q.SetShuffle(true)
			},
			wantHas: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewQueue()
			tt.setup(q)

			got := q.HasNext()
			if got != tt.wantHas {
				t.Errorf("HasNext() = %v, want %v", got, tt.wantHas)
			}
		})
	}
}

func TestQueue_RemoveAt(t *testing.T) {
	t.Run("remove before current", func(t *testing.T) {
		q := NewQueue()
		q.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"}, Track{Path: "/c.mp3"})
		q.JumpTo(2)

		ok := q.RemoveAt(0)

		if !ok {
			t.Error("RemoveAt should return true")
		}
		if q.Len() != 2 {
			t.Errorf("Len() = %d, want 2", q.Len())
		}
		if q.CurrentIndex() != 1 {
			t.Errorf("CurrentIndex() = %d, want 1 (adjusted)", q.CurrentIndex())
		}
	})

	t.Run("remove current", func(t *testing.T) {
		q := NewQueue()
		q.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"}, Track{Path: "/c.mp3"})
		q.JumpTo(1)

		q.RemoveAt(1)

		// After removing current track, currentIndex becomes -1
		// Playback will stop when the current track finishes
		if q.CurrentIndex() != -1 {
			t.Errorf("CurrentIndex() = %d, want -1 (no current track)", q.CurrentIndex())
		}
		if q.Current() != nil {
			t.Error("Current() should be nil after removing current track")
		}
		// HasNext should return false when there's no current track
		if q.HasNext() {
			t.Error("HasNext() should be false when currentIndex is -1")
		}
	})

	t.Run("remove after current", func(t *testing.T) {
		q := NewQueue()
		q.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"}, Track{Path: "/c.mp3"})
		q.JumpTo(0)

		q.RemoveAt(2)

		if q.CurrentIndex() != 0 {
			t.Errorf("CurrentIndex() = %d, want 0 (unchanged)", q.CurrentIndex())
		}
	})
}

func TestQueue_Clear(t *testing.T) {
	q := NewQueue()
	q.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})
	q.JumpTo(1)

	q.Clear()

	if q.Len() != 0 {
		t.Errorf("Len() = %d, want 0", q.Len())
	}
	if q.CurrentIndex() != -1 {
		t.Errorf("CurrentIndex() = %d, want -1", q.CurrentIndex())
	}
}

func TestQueue_CycleRepeatMode(t *testing.T) {
	q := NewQueue()

	if q.RepeatMode() != RepeatOff {
		t.Errorf("initial RepeatMode() = %v, want RepeatOff", q.RepeatMode())
	}

	mode := q.CycleRepeatMode()
	if mode != RepeatAll {
		t.Errorf("after 1st cycle = %v, want RepeatAll", mode)
	}

	mode = q.CycleRepeatMode()
	if mode != RepeatOne {
		t.Errorf("after 2nd cycle = %v, want RepeatOne", mode)
	}

	mode = q.CycleRepeatMode()
	if mode != RepeatRadio {
		t.Errorf("after 3rd cycle = %v, want RepeatRadio", mode)
	}

	mode = q.CycleRepeatMode()
	if mode != RepeatOff {
		t.Errorf("after 4th cycle = %v, want RepeatOff", mode)
	}
}

func TestQueue_ToggleShuffle(t *testing.T) {
	q := NewQueue()

	if q.Shuffle() {
		t.Error("initial Shuffle() should be false")
	}

	got := q.ToggleShuffle()
	if !got {
		t.Error("ToggleShuffle() should return true")
	}
	if !q.Shuffle() {
		t.Error("Shuffle() should be true after toggle")
	}

	got = q.ToggleShuffle()
	if got {
		t.Error("ToggleShuffle() should return false")
	}
}

func TestPlayingQueue_PeekNext(t *testing.T) {
	q := NewQueue()
	track1 := Track{Path: "/music/track1.mp3", Title: "Track 1"}
	track2 := Track{Path: "/music/track2.mp3", Title: "Track 2"}
	track3 := Track{Path: "/music/track3.mp3", Title: "Track 3"}

	// Empty queue
	if q.PeekNext() != nil {
		t.Error("PeekNext() on empty queue should return nil")
	}

	// Add tracks and set position
	q.Replace(track1, track2, track3)
	q.JumpTo(0)

	// Should return track2 without changing position
	next := q.PeekNext()
	if next == nil {
		t.Fatal("PeekNext() should not be nil when there's a next track")
	}
	if next.Path != track2.Path {
		t.Errorf("PeekNext().Path = %s, want %s", next.Path, track2.Path)
	}
	if q.CurrentIndex() != 0 {
		t.Errorf("CurrentIndex() = %d, want 0 (position unchanged)", q.CurrentIndex())
	}

	// At last track with repeat off
	q.JumpTo(2)
	if q.PeekNext() != nil {
		t.Error("PeekNext() at last track with RepeatOff should return nil")
	}

	// At last track with repeat all
	q.SetRepeatMode(RepeatAll)
	next = q.PeekNext()
	if next == nil {
		t.Fatal("PeekNext() with RepeatAll should not be nil")
	}
	if next.Path != track1.Path {
		t.Errorf("PeekNext().Path = %s, want %s (wrap to first)", next.Path, track1.Path)
	}

	// Repeat one returns current track
	q.SetRepeatMode(RepeatOne)
	q.JumpTo(1)
	next = q.PeekNext()
	if next == nil {
		t.Fatal("PeekNext() with RepeatOne should not be nil")
	}
	if next.Path != track2.Path {
		t.Errorf("PeekNext().Path = %s, want %s (repeat one)", next.Path, track2.Path)
	}
}

func TestQueue_MoveIndices(t *testing.T) {
	t.Run("move up", func(t *testing.T) {
		q := NewQueue()
		q.Add(
			Track{Path: "/a.mp3"},
			Track{Path: "/b.mp3"},
			Track{Path: "/c.mp3"},
			Track{Path: "/d.mp3"},
		)

		newIndices, ok := q.MoveIndices([]int{2, 3}, -1)

		if !ok {
			t.Error("MoveIndices should succeed")
		}
		if newIndices[0] != 1 || newIndices[1] != 2 {
			t.Errorf("newIndices = %v, want [1, 2]", newIndices)
		}
		tracks := q.Tracks()
		if tracks[1].Path != "/c.mp3" || tracks[2].Path != "/d.mp3" {
			t.Error("tracks not moved correctly")
		}
	})

	t.Run("move down", func(t *testing.T) {
		q := NewQueue()
		q.Add(
			Track{Path: "/a.mp3"},
			Track{Path: "/b.mp3"},
			Track{Path: "/c.mp3"},
		)

		newIndices, ok := q.MoveIndices([]int{0, 1}, 1)

		if !ok {
			t.Error("MoveIndices should succeed")
		}
		if newIndices[0] != 1 || newIndices[1] != 2 {
			t.Errorf("newIndices = %v, want [1, 2]", newIndices)
		}
	})

	t.Run("cannot move past bounds", func(t *testing.T) {
		q := NewQueue()
		q.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})

		_, ok := q.MoveIndices([]int{0}, -1)

		if ok {
			t.Error("should not be able to move index 0 up")
		}
	})
}

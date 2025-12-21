//nolint:goconst // test file with repeated string literals
package playlist

import (
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/library"
)

func TestNewPlaylist(t *testing.T) {
	p := NewPlaylist()

	if p.Len() != 0 {
		t.Errorf("Len() = %d, want 0", p.Len())
	}
	if p.Tracks() == nil {
		t.Error("Tracks() should return empty slice, not nil")
	}
}

func TestPlaylist_Add(t *testing.T) {
	p := NewPlaylist()

	p.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})

	if p.Len() != 2 {
		t.Errorf("Len() = %d, want 2", p.Len())
	}

	tracks := p.Tracks()
	if tracks[0].Path != "/a.mp3" {
		t.Errorf("tracks[0].Path = %q, want /a.mp3", tracks[0].Path)
	}
	if tracks[1].Path != "/b.mp3" {
		t.Errorf("tracks[1].Path = %q, want /b.mp3", tracks[1].Path)
	}
}

func TestPlaylist_Add_Empty(t *testing.T) {
	p := NewPlaylist()

	p.Add() // Add nothing

	if p.Len() != 0 {
		t.Errorf("Len() = %d, want 0", p.Len())
	}
}

func TestPlaylist_Remove(t *testing.T) {
	p := NewPlaylist()
	p.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"}, Track{Path: "/c.mp3"})

	ok := p.Remove(1)

	if !ok {
		t.Error("Remove should return true")
	}
	if p.Len() != 2 {
		t.Errorf("Len() = %d, want 2", p.Len())
	}

	tracks := p.Tracks()
	if tracks[0].Path != "/a.mp3" {
		t.Errorf("tracks[0].Path = %q, want /a.mp3", tracks[0].Path)
	}
	if tracks[1].Path != "/c.mp3" {
		t.Errorf("tracks[1].Path = %q, want /c.mp3", tracks[1].Path)
	}
}

func TestPlaylist_Remove_InvalidIndex(t *testing.T) {
	p := NewPlaylist()
	p.Add(Track{Path: "/a.mp3"})

	tests := []struct {
		name  string
		index int
	}{
		{"negative", -1},
		{"out of bounds", 5},
		{"at length", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok := p.Remove(tt.index)
			if ok {
				t.Error("Remove with invalid index should return false")
			}
		})
	}
}

func TestPlaylist_Clear(t *testing.T) {
	p := NewPlaylist()
	p.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})

	p.Clear()

	if p.Len() != 0 {
		t.Errorf("Len() = %d, want 0", p.Len())
	}
}

func TestPlaylist_Tracks_ReturnsCopy(t *testing.T) {
	p := NewPlaylist()
	p.Add(Track{Path: "/a.mp3"})

	tracks := p.Tracks()
	tracks[0].Path = "/modified.mp3"

	// Original should be unchanged
	original := p.Tracks()
	if original[0].Path != "/a.mp3" {
		t.Error("Tracks() should return a copy, not the original slice")
	}
}

func TestPlaylist_Track(t *testing.T) {
	p := NewPlaylist()
	p.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})

	track := p.Track(0)
	if track == nil {
		t.Fatal("Track(0) should not be nil")
	}
	if track.Path != "/a.mp3" {
		t.Errorf("Track(0).Path = %q, want /a.mp3", track.Path)
	}

	track = p.Track(1)
	if track == nil {
		t.Fatal("Track(1) should not be nil")
	}
	if track.Path != "/b.mp3" {
		t.Errorf("Track(1).Path = %q, want /b.mp3", track.Path)
	}
}

func TestPlaylist_Track_InvalidIndex(t *testing.T) {
	p := NewPlaylist()
	p.Add(Track{Path: "/a.mp3"})

	tests := []struct {
		name  string
		index int
	}{
		{"negative", -1},
		{"out of bounds", 5},
		{"at length", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track := p.Track(tt.index)
			if track != nil {
				t.Error("Track with invalid index should return nil")
			}
		})
	}
}

func TestPlaylist_Move(t *testing.T) {
	t.Run("move forward", func(t *testing.T) {
		p := NewPlaylist()
		p.Add(
			Track{Path: "/a.mp3"},
			Track{Path: "/b.mp3"},
			Track{Path: "/c.mp3"},
		)

		ok := p.Move(0, 2)

		if !ok {
			t.Error("Move should return true")
		}
		tracks := p.Tracks()
		if tracks[0].Path != "/b.mp3" {
			t.Errorf("tracks[0].Path = %q, want /b.mp3", tracks[0].Path)
		}
		if tracks[1].Path != "/c.mp3" {
			t.Errorf("tracks[1].Path = %q, want /c.mp3", tracks[1].Path)
		}
		if tracks[2].Path != "/a.mp3" {
			t.Errorf("tracks[2].Path = %q, want /a.mp3", tracks[2].Path)
		}
	})

	t.Run("move backward", func(t *testing.T) {
		p := NewPlaylist()
		p.Add(
			Track{Path: "/a.mp3"},
			Track{Path: "/b.mp3"},
			Track{Path: "/c.mp3"},
		)

		ok := p.Move(2, 0)

		if !ok {
			t.Error("Move should return true")
		}
		tracks := p.Tracks()
		if tracks[0].Path != "/c.mp3" {
			t.Errorf("tracks[0].Path = %q, want /c.mp3", tracks[0].Path)
		}
		if tracks[1].Path != "/a.mp3" {
			t.Errorf("tracks[1].Path = %q, want /a.mp3", tracks[1].Path)
		}
		if tracks[2].Path != "/b.mp3" {
			t.Errorf("tracks[2].Path = %q, want /b.mp3", tracks[2].Path)
		}
	})

	t.Run("move to same position", func(t *testing.T) {
		p := NewPlaylist()
		p.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})

		ok := p.Move(1, 1)

		if !ok {
			t.Error("Move to same position should return true")
		}
		tracks := p.Tracks()
		if tracks[1].Path != "/b.mp3" {
			t.Errorf("tracks[1].Path = %q, want /b.mp3", tracks[1].Path)
		}
	})
}

func TestPlaylist_Move_InvalidIndex(t *testing.T) {
	p := NewPlaylist()
	p.Add(Track{Path: "/a.mp3"}, Track{Path: "/b.mp3"})

	tests := []struct {
		name string
		from int
		to   int
	}{
		{"negative from", -1, 0},
		{"negative to", 0, -1},
		{"from out of bounds", 5, 0},
		{"to out of bounds", 0, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok := p.Move(tt.from, tt.to)
			if ok {
				t.Error("Move with invalid index should return false")
			}
		})
	}
}

// History tests

func TestNewQueueHistory(t *testing.T) {
	h := NewQueueHistory(10)

	if h.CanUndo() {
		t.Error("new history should not be able to undo")
	}
	if h.CanRedo() {
		t.Error("new history should not be able to redo")
	}
}

func TestQueueHistory_Push(t *testing.T) {
	h := NewQueueHistory(10)

	tracks := []Track{{Path: "/a.mp3"}, {Path: "/b.mp3"}}
	h.Push(tracks)

	// After first push, still can't undo (need at least 2 states)
	if h.CanUndo() {
		t.Error("after first push, should not be able to undo")
	}

	h.Push([]Track{{Path: "/c.mp3"}})

	if !h.CanUndo() {
		t.Error("after second push, should be able to undo")
	}
}

func TestQueueHistory_Undo(t *testing.T) {
	h := NewQueueHistory(10)

	h.Push([]Track{{Path: "/a.mp3"}})
	h.Push([]Track{{Path: "/b.mp3"}})

	restored, ok := h.Undo()

	if !ok {
		t.Error("Undo should succeed")
	}
	if len(restored) != 1 || restored[0].Path != "/a.mp3" {
		t.Errorf("restored = %v, want [{/a.mp3}]", restored)
	}
}

func TestQueueHistory_Undo_Empty(t *testing.T) {
	h := NewQueueHistory(10)

	restored, ok := h.Undo()

	if ok {
		t.Error("Undo on empty history should return false")
	}
	if restored != nil {
		t.Error("Undo on empty should return nil")
	}
}

func TestQueueHistory_Undo_AtStart(t *testing.T) {
	h := NewQueueHistory(10)
	h.Push([]Track{{Path: "/a.mp3"}})

	restored, ok := h.Undo()

	if ok {
		t.Error("Undo at start should return false")
	}
	if restored != nil {
		t.Error("Undo at start should return nil")
	}
}

func TestQueueHistory_Redo(t *testing.T) {
	h := NewQueueHistory(10)
	h.Push([]Track{{Path: "/a.mp3"}})
	h.Push([]Track{{Path: "/b.mp3"}})
	h.Undo()

	restored, ok := h.Redo()

	if !ok {
		t.Error("Redo should succeed")
	}
	if len(restored) != 1 || restored[0].Path != "/b.mp3" {
		t.Errorf("restored = %v, want [{/b.mp3}]", restored)
	}
}

func TestQueueHistory_Redo_AtEnd(t *testing.T) {
	h := NewQueueHistory(10)
	h.Push([]Track{{Path: "/a.mp3"}})

	restored, ok := h.Redo()

	if ok {
		t.Error("Redo at end should return false")
	}
	if restored != nil {
		t.Error("Redo at end should return nil")
	}
}

func TestQueueHistory_Redo_Empty(t *testing.T) {
	h := NewQueueHistory(10)

	restored, ok := h.Redo()

	if ok {
		t.Error("Redo on empty should return false")
	}
	if restored != nil {
		t.Error("Redo on empty should return nil")
	}
}

func TestQueueHistory_PushClearsRedo(t *testing.T) {
	h := NewQueueHistory(10)
	h.Push([]Track{{Path: "/a.mp3"}})
	h.Push([]Track{{Path: "/b.mp3"}})
	h.Push([]Track{{Path: "/c.mp3"}})
	h.Undo() // back to b
	h.Undo() // back to a

	// Push new state should clear redo (b and c)
	h.Push([]Track{{Path: "/d.mp3"}})

	if h.CanRedo() {
		t.Error("push should clear redo states")
	}

	// Can only undo to a
	restored, _ := h.Undo()
	if restored[0].Path != "/a.mp3" {
		t.Errorf("should undo to a, got %q", restored[0].Path)
	}
}

func TestQueueHistory_MaxSize(t *testing.T) {
	h := NewQueueHistory(3)

	h.Push([]Track{{Path: "/a.mp3"}})
	h.Push([]Track{{Path: "/b.mp3"}})
	h.Push([]Track{{Path: "/c.mp3"}})
	h.Push([]Track{{Path: "/d.mp3"}}) // should trim a

	// Undo should go: d -> c -> b (a is trimmed)
	h.Undo()
	h.Undo()

	if h.CanUndo() {
		t.Error("should not be able to undo past max size")
	}
}

func TestQueueHistory_ReturnsCopy(t *testing.T) {
	h := NewQueueHistory(10)

	original := []Track{{Path: "/a.mp3"}}
	h.Push(original)
	h.Push([]Track{{Path: "/b.mp3"}})

	restored, _ := h.Undo()
	restored[0].Path = "/modified.mp3"

	// Push again and undo
	h.Push([]Track{{Path: "/c.mp3"}})
	restoredAgain, _ := h.Undo()

	// Should get original value, not modified
	if restoredAgain[0].Path != "/a.mp3" {
		t.Errorf("history should store copies, got %q", restoredAgain[0].Path)
	}
}

func TestQueueHistory_MultipleUndoRedo(t *testing.T) {
	h := NewQueueHistory(10)

	h.Push([]Track{{Path: "/1.mp3"}})
	h.Push([]Track{{Path: "/2.mp3"}})
	h.Push([]Track{{Path: "/3.mp3"}})
	h.Push([]Track{{Path: "/4.mp3"}})

	// Undo twice
	h.Undo()
	restored, _ := h.Undo()
	if restored[0].Path != "/2.mp3" {
		t.Errorf("after 2 undos, got %q, want /2.mp3", restored[0].Path)
	}

	// Redo once
	restored, _ = h.Redo()
	if restored[0].Path != "/3.mp3" {
		t.Errorf("after redo, got %q, want /3.mp3", restored[0].Path)
	}

	// Redo again
	restored, _ = h.Redo()
	if restored[0].Path != "/4.mp3" {
		t.Errorf("after second redo, got %q, want /4.mp3", restored[0].Path)
	}

	// Can't redo anymore
	if h.CanRedo() {
		t.Error("should not be able to redo at end")
	}
}

// Utility function tests

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "00:00"},
		{30 * time.Second, "00:30"},
		{1 * time.Minute, "01:00"},
		{1*time.Minute + 30*time.Second, "01:30"},
		{5*time.Minute + 45*time.Second, "05:45"},
		{10 * time.Minute, "10:00"},
		{59*time.Minute + 59*time.Second, "59:59"},
		{60 * time.Minute, "60:00"},
		{90 * time.Minute, "90:00"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFromLibraryTrack(t *testing.T) {
	libTrack := library.Track{
		ID:          123,
		Path:        "/music/song.mp3",
		Title:       "Test Song",
		Artist:      "Test Artist",
		Album:       "Test Album",
		TrackNumber: 5,
	}

	track := FromLibraryTrack(libTrack)

	if track.ID != 123 {
		t.Errorf("ID = %d, want 123", track.ID)
	}
	if track.Path != "/music/song.mp3" {
		t.Errorf("Path = %q, want /music/song.mp3", track.Path)
	}
	if track.Title != "Test Song" {
		t.Errorf("Title = %q, want Test Song", track.Title)
	}
	if track.Artist != "Test Artist" {
		t.Errorf("Artist = %q, want Test Artist", track.Artist)
	}
	if track.Album != "Test Album" {
		t.Errorf("Album = %q, want Test Album", track.Album)
	}
	if track.TrackNumber != 5 {
		t.Errorf("TrackNumber = %d, want 5", track.TrackNumber)
	}
}

func TestFromLibraryTracks(t *testing.T) {
	libTracks := []library.Track{
		{ID: 1, Path: "/a.mp3", Title: "A"},
		{ID: 2, Path: "/b.mp3", Title: "B"},
		{ID: 3, Path: "/c.mp3", Title: "C"},
	}

	tracks := FromLibraryTracks(libTracks)

	if len(tracks) != 3 {
		t.Fatalf("len = %d, want 3", len(tracks))
	}

	for i, track := range tracks {
		if track.ID != libTracks[i].ID {
			t.Errorf("tracks[%d].ID = %d, want %d", i, track.ID, libTracks[i].ID)
		}
		if track.Path != libTracks[i].Path {
			t.Errorf("tracks[%d].Path = %q, want %q", i, track.Path, libTracks[i].Path)
		}
	}
}

func TestFromLibraryTracks_Empty(t *testing.T) {
	tracks := FromLibraryTracks(nil)

	if len(tracks) != 0 {
		t.Errorf("len = %d, want 0", len(tracks))
	}
}

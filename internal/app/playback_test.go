// internal/app/playback_test.go
package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

func TestHandleSpaceAction_WhenStopped_StartsPlayback(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(playlist.Track{Path: "/test.mp3"})
	m.Playback.Queue().JumpTo(0)

	cmd := m.HandleSpaceAction()

	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	if mock.State() != player.Playing {
		t.Error("expected player to be playing")
	}
	if cmd == nil {
		t.Error("expected tick command")
	}
}

func TestHandleSpaceAction_WhenPlaying_Pauses(t *testing.T) {
	m := newPlaybackTestModel()
	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	m.HandleSpaceAction()

	if mock.State() != player.Paused {
		t.Errorf("player state = %v, want Paused", mock.State())
	}
}

func TestHandleSpaceAction_WhenPaused_Resumes(t *testing.T) {
	m := newPlaybackTestModel()
	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Paused)

	m.HandleSpaceAction()

	if mock.State() != player.Playing {
		t.Errorf("player state = %v, want Playing", mock.State())
	}
}

func TestHandleSpaceAction_WhenStoppedAndEmptyQueue_DoesNothing(t *testing.T) {
	m := newPlaybackTestModel()

	cmd := m.HandleSpaceAction()

	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	if mock.State() != player.Stopped {
		t.Errorf("player state = %v, want Stopped", mock.State())
	}
	if cmd != nil {
		t.Error("expected no command for empty queue")
	}
}

func TestStartQueuePlayback_WithEmptyQueue_ReturnsNil(t *testing.T) {
	m := newPlaybackTestModel()

	cmd := m.StartQueuePlayback()

	if cmd != nil {
		t.Error("expected nil command for empty queue")
	}
}

func TestStartQueuePlayback_WithTrack_PlaysAndReturnsTick(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(playlist.Track{Path: "/music/song.mp3"})
	m.Playback.Queue().JumpTo(0)

	cmd := m.StartQueuePlayback()

	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	calls := mock.PlayCalls()
	if len(calls) != 1 || calls[0] != "/music/song.mp3" {
		t.Errorf("PlayCalls = %v, want [/music/song.mp3]", calls)
	}
	if cmd == nil {
		t.Error("expected tick command")
	}
}

func TestJumpToQueueIndex_WhenStopped_DoesNotStartPlayback(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(
		playlist.Track{Path: "/track1.mp3"},
		playlist.Track{Path: "/track2.mp3"},
	)

	cmd := m.JumpToQueueIndex(1)

	if m.Playback.Queue().CurrentIndex() != 1 {
		t.Errorf("CurrentIndex = %d, want 1", m.Playback.Queue().CurrentIndex())
	}
	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	if len(mock.PlayCalls()) != 0 {
		t.Error("expected no play calls when stopped")
	}
	if cmd != nil {
		t.Error("expected nil command when stopped")
	}
}

func TestJumpToQueueIndex_WhenPlaying_ReturnsTimeoutCmd(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(
		playlist.Track{Path: "/track1.mp3"},
		playlist.Track{Path: "/track2.mp3"},
	)
	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	cmd := m.JumpToQueueIndex(1)

	if cmd == nil {
		t.Error("expected timeout command when playing")
	}
	if m.PendingTrackIdx != 1 {
		t.Errorf("PendingTrackIdx = %d, want 1", m.PendingTrackIdx)
	}
}

func TestAdvanceToNextTrack_EmptyQueue_ReturnsNil(t *testing.T) {
	m := newPlaybackTestModel()

	cmd := m.AdvanceToNextTrack()

	if cmd != nil {
		t.Error("expected nil for empty queue")
	}
}

func TestAdvanceToNextTrack_WhenStopped_AdvancesWithoutPlaying(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(
		playlist.Track{Path: "/track1.mp3"},
		playlist.Track{Path: "/track2.mp3"},
	)
	m.Playback.Queue().JumpTo(0)

	cmd := m.AdvanceToNextTrack()

	if m.Playback.Queue().CurrentIndex() != 1 {
		t.Errorf("CurrentIndex = %d, want 1", m.Playback.Queue().CurrentIndex())
	}
	if cmd != nil {
		t.Error("expected nil command when stopped")
	}
}

func TestGoToPreviousTrack_AtStart_ReturnsNil(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(playlist.Track{Path: "/track1.mp3"})
	m.Playback.Queue().JumpTo(0)

	cmd := m.GoToPreviousTrack()

	if cmd != nil {
		t.Error("expected nil at start of queue")
	}
	if m.Playback.Queue().CurrentIndex() != 0 {
		t.Errorf("CurrentIndex = %d, want 0", m.Playback.Queue().CurrentIndex())
	}
}

func TestGoToPreviousTrack_NotAtStart_MovesPrevious(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(
		playlist.Track{Path: "/track1.mp3"},
		playlist.Track{Path: "/track2.mp3"},
	)
	m.Playback.Queue().JumpTo(1)

	m.GoToPreviousTrack()

	if m.Playback.Queue().CurrentIndex() != 0 {
		t.Errorf("CurrentIndex = %d, want 0", m.Playback.Queue().CurrentIndex())
	}
}

func TestPlayTrackAtIndex_ValidIndex_PlaysTrack(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(
		playlist.Track{Path: "/track1.mp3"},
		playlist.Track{Path: "/track2.mp3"},
	)

	cmd := m.PlayTrackAtIndex(1)

	mock, ok := m.Playback.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	calls := mock.PlayCalls()
	if len(calls) != 1 || calls[0] != "/track2.mp3" {
		t.Errorf("PlayCalls = %v, want [/track2.mp3]", calls)
	}
	if cmd == nil {
		t.Error("expected tick command")
	}
}

func TestPlayTrackAtIndex_InvalidIndex_ReturnsNil(t *testing.T) {
	m := newPlaybackTestModel()
	m.Playback.Queue().Add(playlist.Track{Path: "/track1.mp3"})

	cmd := m.PlayTrackAtIndex(5)

	if cmd != nil {
		t.Error("expected nil for invalid index")
	}
}

func TestTogglePlayerDisplayMode_WhenStopped_DoesNothing(_ *testing.T) {
	m := newPlaybackTestModel()

	m.TogglePlayerDisplayMode()

	// No panic means success - mode unchanged when stopped
}

// newPlaybackTestModel creates a model for playback tests.
func newPlaybackTestModel() *Model {
	queue := playlist.NewQueue()
	p := player.NewMock()
	return &Model{
		Playback: NewPlaybackManager(p, queue),
		Layout:   NewLayoutManager(queuepanel.New(queue)),
		StateMgr: state.NewMock(),
	}
}

// internal/app/playback_test.go
package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

func TestHandleSpaceAction_WhenStopped_StartsPlayback(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/test.mp3"})
	m.Core.Playback().QueueMoveTo(0)

	_ = m.HandleSpaceAction()

	mock := testPlayer(m.Core)
	if mock.State() != player.Playing {
		t.Error("expected player to be playing")
	}
	// Note: Tick commands are now started via service events, not returned directly
}

func TestHandleSpaceAction_WhenPlaying_Pauses(t *testing.T) {
	m := newPlaybackTestModel(t)
	mock := testPlayer(m.Core)
	mock.SetState(player.Playing)

	m.HandleSpaceAction()

	if mock.State() != player.Paused {
		t.Errorf("player state = %v, want Paused", mock.State())
	}
}

func TestHandleSpaceAction_WhenPaused_Resumes(t *testing.T) {
	m := newPlaybackTestModel(t)
	mock := testPlayer(m.Core)
	mock.SetState(player.Paused)

	m.HandleSpaceAction()

	if mock.State() != player.Playing {
		t.Errorf("player state = %v, want Playing", mock.State())
	}
}

func TestHandleSpaceAction_WhenStoppedAndEmptyQueue_DoesNothing(t *testing.T) {
	m := newPlaybackTestModel(t)

	cmd := m.HandleSpaceAction()

	mock := testPlayer(m.Core)
	if mock.State() != player.Stopped {
		t.Errorf("player state = %v, want Stopped", mock.State())
	}
	if cmd != nil {
		t.Error("expected no command for empty queue")
	}
}

func TestStartQueuePlayback_WithEmptyQueue_ReturnsNil(t *testing.T) {
	m := newPlaybackTestModel(t)

	cmd := m.StartQueuePlayback()

	if cmd != nil {
		t.Error("expected nil command for empty queue")
	}
}

func TestStartQueuePlayback_WithTrack_PlaysTrack(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/music/song.mp3"})
	m.Core.Playback().QueueMoveTo(0)

	_ = m.StartQueuePlayback()

	mock := testPlayer(m.Core)
	calls := mock.PlayCalls()
	if len(calls) != 1 || calls[0] != "/music/song.mp3" {
		t.Errorf("PlayCalls = %v, want [/music/song.mp3]", calls)
	}
	// Note: Tick commands are now started via service events, not returned directly
}

func TestJumpToQueueIndex_WhenStopped_DoesNotStartPlayback(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(
		core.Track{Path: "/track1.mp3"},
		core.Track{Path: "/track2.mp3"},
	)

	cmd := m.JumpToQueueIndex(1)

	if m.Core.Playback().QueueCurrentIndex() != 1 {
		t.Errorf("CurrentIndex = %d, want 1", m.Core.Playback().QueueCurrentIndex())
	}
	mock := testPlayer(m.Core)
	if len(mock.PlayCalls()) != 0 {
		t.Error("expected no play calls when stopped")
	}
	if cmd != nil {
		t.Error("expected nil command when stopped")
	}
}

func TestJumpToQueueIndex_WhenPlaying_ReturnsTimeoutCmd(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(
		core.Track{Path: "/track1.mp3"},
		core.Track{Path: "/track2.mp3"},
	)
	mock := testPlayer(m.Core)
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
	m := newPlaybackTestModel(t)

	cmd := m.AdvanceToNextTrack()

	if cmd != nil {
		t.Error("expected nil for empty queue")
	}
}

func TestAdvanceToNextTrack_WhenStopped_AdvancesWithoutPlaying(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(
		core.Track{Path: "/track1.mp3"},
		core.Track{Path: "/track2.mp3"},
	)
	m.Core.Playback().QueueMoveTo(0)

	cmd := m.AdvanceToNextTrack()

	if m.Core.Playback().QueueCurrentIndex() != 1 {
		t.Errorf("CurrentIndex = %d, want 1", m.Core.Playback().QueueCurrentIndex())
	}
	if cmd != nil {
		t.Error("expected nil command when stopped")
	}
}

func TestGoToPreviousTrack_AtStart_ReturnsNil(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/track1.mp3"})
	m.Core.Playback().QueueMoveTo(0)

	cmd := m.GoToPreviousTrack()

	if cmd != nil {
		t.Error("expected nil at start of queue")
	}
	if m.Core.Playback().QueueCurrentIndex() != 0 {
		t.Errorf("CurrentIndex = %d, want 0", m.Core.Playback().QueueCurrentIndex())
	}
}

func TestGoToPreviousTrack_NotAtStart_MovesPrevious(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(
		core.Track{Path: "/track1.mp3"},
		core.Track{Path: "/track2.mp3"},
	)
	m.Core.Playback().QueueMoveTo(1)

	m.GoToPreviousTrack()

	if m.Core.Playback().QueueCurrentIndex() != 0 {
		t.Errorf("CurrentIndex = %d, want 0", m.Core.Playback().QueueCurrentIndex())
	}
}

func TestPlayTrackAtIndex_ValidIndex_PlaysTrack(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(
		core.Track{Path: "/track1.mp3"},
		core.Track{Path: "/track2.mp3"},
	)

	_ = m.PlayTrackAtIndex(1)

	mock := testPlayer(m.Core)
	calls := mock.PlayCalls()
	if len(calls) != 1 || calls[0] != "/track2.mp3" {
		t.Errorf("PlayCalls = %v, want [/track2.mp3]", calls)
	}
	// Note: Tick commands are now started via service events, not returned directly
}

func TestPlayTrackAtIndex_InvalidIndex_ReturnsNil(t *testing.T) {
	m := newPlaybackTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/track1.mp3"})

	cmd := m.PlayTrackAtIndex(5)

	if cmd != nil {
		t.Error("expected nil for invalid index")
	}
}

func TestTogglePlayerDisplayMode_WhenStopped_DoesNothing(t *testing.T) {
	m := newPlaybackTestModel(t)

	m.TogglePlayerDisplayMode()

	// No panic means success - mode unchanged when stopped
}

// newPlaybackTestModel creates a model for playback tests.
func newPlaybackTestModel(t testing.TB) *Model {
	t.Helper()
	c := newTestCore(t)
	return &Model{
		Core:   c,
		Layout: NewLayoutManager(queuepanel.New(c.Playback())),
	}
}

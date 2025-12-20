// internal/app/playback.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// PlayTrack attempts to play a track and handles errors consistently.
// Returns commands for tick and radio fill (if on last track).
// Always calls ResizeComponents to ensure proper layout.
func (m *Model) PlayTrack(path string) tea.Cmd {
	if err := m.Playback.Play(path); err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpPlaybackStart, err))
		m.ResizeComponents()
		m.Layout.QueuePanel().SyncCursor()
		return nil
	}
	m.ResizeComponents()
	m.Layout.QueuePanel().SyncCursor()

	// Reset scrobble state for new track
	m.ScrobbleState = &lastfm.ScrobbleState{
		TrackPath: path,
		StartedAt: time.Now(),
	}

	// Reset radio fill flag for new track
	m.RadioFillTriggered = false

	// Trigger radio fill when starting the last track (pre-fetch next tracks)
	if radioCmd := m.triggerRadioFill(); radioCmd != nil {
		return tea.Batch(TickCmd(), radioCmd)
	}

	return TickCmd()
}

// HandleSpaceAction handles the space key: toggle pause/resume or start playback.
func (m *Model) HandleSpaceAction() tea.Cmd {
	if !m.Playback.IsStopped() {
		m.Playback.Toggle()
		return nil
	}
	return m.StartQueuePlayback()
}

// StartQueuePlayback starts playback from the current queue position.
func (m *Model) StartQueuePlayback() tea.Cmd {
	if m.Playback.Queue().IsEmpty() {
		return nil
	}
	track := m.Playback.Queue().Current()
	if track == nil {
		return nil
	}
	m.Layout.QueuePanel().SyncCursor()
	return m.PlayTrack(track.Path)
}

// JumpToQueueIndex moves to a queue position with debouncing when playing.
func (m *Model) JumpToQueueIndex(index int) tea.Cmd {
	m.Playback.Queue().JumpTo(index)
	m.Layout.QueuePanel().SyncCursor()

	if m.Playback.IsStopped() {
		m.SaveQueueState()
		return nil
	}
	m.TrackSkipVersion++
	m.PendingTrackIdx = index
	return TrackSkipTimeoutCmd(m.TrackSkipVersion)
}

// AdvanceToNextTrack advances to the next track respecting shuffle/repeat modes.
func (m *Model) AdvanceToNextTrack() tea.Cmd {
	if m.Playback.Queue().IsEmpty() {
		return nil
	}

	nextTrack := m.Playback.Queue().Next()
	if nextTrack == nil {
		return nil
	}

	m.Layout.QueuePanel().SyncCursor()

	if m.Playback.IsStopped() {
		m.SaveQueueState()
		return nil
	}

	m.TrackSkipVersion++
	m.PendingTrackIdx = m.Playback.Queue().CurrentIndex()
	return TrackSkipTimeoutCmd(m.TrackSkipVersion)
}

// GoToPreviousTrack moves to the previous track (always linear, ignores shuffle).
func (m *Model) GoToPreviousTrack() tea.Cmd {
	if m.Playback.Queue().CurrentIndex() <= 0 {
		return nil
	}
	return m.JumpToQueueIndex(m.Playback.Queue().CurrentIndex() - 1)
}

// PlayTrackAtIndex plays the track at the given queue index.
func (m *Model) PlayTrackAtIndex(index int) tea.Cmd {
	track := m.Playback.Queue().JumpTo(index)
	if track == nil {
		return nil
	}

	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()
	return m.PlayTrack(track.Path)
}

// TogglePlayerDisplayMode cycles between compact and expanded player display.
func (m *Model) TogglePlayerDisplayMode() {
	if m.Playback.IsStopped() {
		return
	}

	if m.Playback.DisplayMode() == playerbar.ModeExpanded {
		m.Playback.SetDisplayMode(playerbar.ModeCompact)
	} else {
		minHeightForExpanded := playerbar.Height(playerbar.ModeExpanded) + 8
		if m.Layout.Height() >= minHeightForExpanded {
			m.Playback.SetDisplayMode(playerbar.ModeExpanded)
		}
	}

	m.ResizeComponents()
	m.Layout.QueuePanel().SyncCursor()
}

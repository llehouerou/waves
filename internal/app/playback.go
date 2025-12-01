// internal/app/playback.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// HandleSpaceAction handles the space key: toggle pause/resume or start playback.
func (m *Model) HandleSpaceAction() tea.Cmd {
	if m.Player.State() != player.Stopped {
		m.Player.Toggle()
		return nil
	}
	return m.StartQueuePlayback()
}

// StartQueuePlayback starts playback from the current queue position.
func (m *Model) StartQueuePlayback() tea.Cmd {
	if m.Queue.IsEmpty() {
		return nil
	}
	track := m.Queue.Current()
	if track == nil {
		return nil
	}
	if err := m.Player.Play(track.Path); err != nil {
		m.ErrorMsg = err.Error()
		return nil
	}
	m.QueuePanel.SyncCursor()
	m.ResizeComponents()
	return TickCmd()
}

// JumpToQueueIndex moves to a queue position with debouncing when playing.
func (m *Model) JumpToQueueIndex(index int) tea.Cmd {
	m.Queue.JumpTo(index)
	m.QueuePanel.SyncCursor()

	if m.Player.State() == player.Stopped {
		m.SaveQueueState()
		return nil
	}
	m.TrackSkipVersion++
	m.PendingTrackIdx = index
	return TrackSkipTimeoutCmd(m.TrackSkipVersion)
}

// AdvanceToNextTrack advances to the next track respecting shuffle/repeat modes.
func (m *Model) AdvanceToNextTrack() tea.Cmd {
	if m.Queue.IsEmpty() {
		return nil
	}

	nextTrack := m.Queue.Next()
	if nextTrack == nil {
		return nil
	}

	m.QueuePanel.SyncCursor()

	if m.Player.State() == player.Stopped {
		m.SaveQueueState()
		return nil
	}

	m.TrackSkipVersion++
	m.PendingTrackIdx = m.Queue.CurrentIndex()
	return TrackSkipTimeoutCmd(m.TrackSkipVersion)
}

// GoToPreviousTrack moves to the previous track (always linear, ignores shuffle).
func (m *Model) GoToPreviousTrack() tea.Cmd {
	if m.Queue.CurrentIndex() <= 0 {
		return nil
	}
	return m.JumpToQueueIndex(m.Queue.CurrentIndex() - 1)
}

// PlayTrackAtIndex plays the track at the given queue index.
func (m *Model) PlayTrackAtIndex(index int) tea.Cmd {
	track := m.Queue.JumpTo(index)
	if track == nil {
		return nil
	}

	if err := m.Player.Play(track.Path); err != nil {
		m.ErrorMsg = err.Error()
		return nil
	}

	m.SaveQueueState()
	m.QueuePanel.SyncCursor()
	m.ResizeComponents()
	return TickCmd()
}

// TogglePlayerDisplayMode cycles between compact and expanded player display.
func (m *Model) TogglePlayerDisplayMode() {
	if m.Player.State() == player.Stopped {
		return
	}

	if m.PlayerDisplayMode == playerbar.ModeExpanded {
		m.PlayerDisplayMode = playerbar.ModeCompact
	} else {
		minHeightForExpanded := playerbar.Height(playerbar.ModeExpanded) + 8
		if m.Height >= minHeightForExpanded {
			m.PlayerDisplayMode = playerbar.ModeExpanded
		}
	}

	m.ResizeComponents()
}

// internal/app/playback.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// HandleSpaceAction handles the space key: toggle pause/resume or start playback.
func (m *Model) HandleSpaceAction() tea.Cmd {
	if !m.PlaybackService.IsStopped() {
		if err := m.PlaybackService.Toggle(); err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		}
		return nil
	}
	return m.StartQueuePlayback()
}

// StartQueuePlayback starts playback from the current queue position.
func (m *Model) StartQueuePlayback() tea.Cmd {
	if m.PlaybackService.QueueIsEmpty() {
		return nil
	}
	m.Layout.QueuePanel().SyncCursor()
	if err := m.PlaybackService.Play(); err != nil {
		m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		return nil
	}
	// Service emits events; handleServiceStateChanged starts TickCmd
	return nil
}

// JumpToQueueIndex moves to a queue position with debouncing when playing.
func (m *Model) JumpToQueueIndex(index int) tea.Cmd {
	m.PlaybackService.QueueMoveTo(index)
	m.Layout.QueuePanel().SyncCursor()

	if m.PlaybackService.IsStopped() {
		return nil
	}
	m.TrackSkipVersion++
	m.PendingTrackIdx = index
	return TrackSkipTimeoutCmd(m.TrackSkipVersion)
}

// AdvanceToNextTrack advances to the next track respecting shuffle/repeat modes.
func (m *Model) AdvanceToNextTrack() tea.Cmd {
	if m.PlaybackService.QueueIsEmpty() {
		return nil
	}

	nextTrack := m.PlaybackService.QueueAdvance()
	if nextTrack == nil {
		return nil
	}

	m.Layout.QueuePanel().SyncCursor()

	if m.PlaybackService.IsStopped() {
		return nil
	}

	m.TrackSkipVersion++
	m.PendingTrackIdx = m.PlaybackService.QueueCurrentIndex()
	return TrackSkipTimeoutCmd(m.TrackSkipVersion)
}

// GoToPreviousTrack moves to the previous track (always linear, ignores shuffle).
func (m *Model) GoToPreviousTrack() tea.Cmd {
	if m.PlaybackService.QueueCurrentIndex() <= 0 {
		return nil
	}
	return m.JumpToQueueIndex(m.PlaybackService.QueueCurrentIndex() - 1)
}

// PlayTrackAtIndex plays the track at the given queue index.
// QueueMoveTo emits TrackChange which triggers handleServiceTrackChanged
// to handle notifications, scrobble, album art, lyrics, etc.
func (m *Model) PlayTrackAtIndex(index int) tea.Cmd {
	track := m.PlaybackService.QueueMoveTo(index)
	if track == nil {
		return nil
	}

	if err := m.PlaybackService.Play(); err != nil {
		m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		return nil
	}

	return nil
}

// TogglePlayerDisplayMode cycles between compact and expanded player display.
// Returns a command when switching to expanded needs album art loaded.
func (m *Model) TogglePlayerDisplayMode() tea.Cmd {
	if m.PlaybackService.IsStopped() {
		return nil
	}

	var cmd tea.Cmd
	if m.Layout.PlayerDisplayMode() == playerbar.ModeExpanded {
		m.switchToCompactMode()
	} else {
		cmd = m.switchToExpandedMode()
	}

	m.ResizeComponents()
	m.Layout.QueuePanel().SyncCursor()
	return cmd
}

func (m *Model) switchToCompactMode() {
	m.Layout.SetPlayerDisplayMode(playerbar.ModeCompact)
	if m.AlbumArt != nil {
		m.albumArtPendingTransmit = m.AlbumArt.Clear()
	}
}

// switchToExpandedMode shows the expanded player, returning a command to load
// album art when there is a track to load it for.
func (m *Model) switchToExpandedMode() tea.Cmd {
	minHeightForExpanded := playerbar.Height(playerbar.ModeExpanded) + 8
	if m.Layout.Height() < minHeightForExpanded {
		return nil
	}
	m.Layout.SetPlayerDisplayMode(playerbar.ModeExpanded)
	if m.AlbumArt == nil {
		return nil
	}
	m.AlbumArt.InvalidateCache()
	return m.albumArtCmdIfNeeded()
}

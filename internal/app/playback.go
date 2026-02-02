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
	if err := m.PlaybackService.PlayPath(path); err != nil {
		m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
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
	m.SaveQueueState()
	// Service emits events; handleServiceStateChanged starts TickCmd
	return nil
}

// JumpToQueueIndex moves to a queue position with debouncing when playing.
func (m *Model) JumpToQueueIndex(index int) tea.Cmd {
	m.PlaybackService.QueueMoveTo(index)
	m.Layout.QueuePanel().SyncCursor()

	if m.PlaybackService.IsStopped() {
		m.SaveQueueState()
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
		m.SaveQueueState()
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
func (m *Model) PlayTrackAtIndex(index int) tea.Cmd {
	track := m.PlaybackService.QueueMoveTo(index)
	if track == nil {
		return nil
	}

	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()

	if err := m.PlaybackService.Play(); err != nil {
		m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		return nil
	}

	// Handle track change manually since service.Play() doesn't emit
	// TrackChange when called after debounced queue navigation
	// (the queue was already moved by AdvanceToNextTrack/GoToPreviousTrack)
	m.resetScrobbleState()

	// Clear album art immediately and start async preparation
	var cmds []tea.Cmd
	if m.AlbumArt != nil {
		m.albumArtPendingTransmit = m.AlbumArt.Clear()
		if cmd := m.prepareAlbumArtCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Update lyrics popup if visible
	if m.Popups != nil {
		if lyr := m.Popups.Lyrics(); lyr != nil {
			cmds = append(cmds, lyr.SetTrack(
				track.Path, track.Artist, track.Title, track.Album,
				m.PlaybackService.Player().Duration(),
			))
		}
	}

	if radioCmd := m.triggerRadioFill(); radioCmd != nil {
		cmds = append(cmds, radioCmd)
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// TogglePlayerDisplayMode cycles between compact and expanded player display.
// Returns a command to prepare album art when switching to expanded mode.
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

func (m *Model) switchToExpandedMode() tea.Cmd {
	minHeightForExpanded := playerbar.Height(playerbar.ModeExpanded) + 8
	if m.Layout.Height() < minHeightForExpanded {
		return nil
	}
	m.Layout.SetPlayerDisplayMode(playerbar.ModeExpanded)
	// Trigger async album art preparation
	return m.prepareAlbumArtCmd()
}

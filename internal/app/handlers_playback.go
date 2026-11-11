// internal/app/handlers_playback.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/keymap"
)

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S, L.
func (m *Model) handlePlaybackKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling playback actions
	case keymap.ActionPlayPause:
		return handler.Handled(m.HandleSpaceAction())
	case keymap.ActionStop:
		_ = m.Core.Playback().Stop()
		m.ResizeComponents()
		return handler.HandledNoCmd
	case keymap.ActionNextTrack:
		return handler.Handled(m.AdvanceToNextTrack())
	case keymap.ActionPrevTrack:
		return handler.Handled(m.GoToPreviousTrack())
	case keymap.ActionFirstTrack:
		if !m.Core.Playback().QueueIsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(0))
		}
		return handler.HandledNoCmd
	case keymap.ActionLastTrack:
		if !m.Core.Playback().QueueIsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(m.Core.Playback().QueueLen() - 1))
		}
		return handler.HandledNoCmd
	case keymap.ActionTogglePlayerDisplay:
		return handler.Handled(m.TogglePlayerDisplayMode())
	case keymap.ActionSeekBack:
		m.handleSeek(-5)
		return handler.HandledNoCmd
	case keymap.ActionSeekForward:
		m.handleSeek(5)
		return handler.HandledNoCmd
	case keymap.ActionSeekBackLong:
		m.handleSeek(-15)
		return handler.HandledNoCmd
	case keymap.ActionSeekForwardLong:
		m.handleSeek(15)
		return handler.HandledNoCmd
	case keymap.ActionCycleRepeat:
		m.Core.Playback().CycleRepeatMode()
		return handler.HandledNoCmd
	case keymap.ActionToggleShuffle:
		m.Core.Playback().ToggleShuffle()
		return handler.HandledNoCmd
	case keymap.ActionShowLyrics:
		return handler.Handled(m.handleShowLyrics())
	case keymap.ActionVolumeUp:
		m.Core.Playback().AdjustVolume(0.10)
		return handler.HandledNoCmd
	case keymap.ActionVolumeDown:
		m.Core.Playback().AdjustVolume(-0.10)
		return handler.HandledNoCmd
	case keymap.ActionToggleMute:
		m.Core.Playback().ToggleMute()
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// handleShowLyrics shows or hides the lyrics popup.
func (m *Model) handleShowLyrics() tea.Cmd {
	// Toggle off if already showing
	if m.Popups.IsVisible(popupctl.Lyrics) {
		m.Popups.Hide(popupctl.Lyrics)
		return nil
	}

	// Need a current track to show lyrics
	track := m.Core.Playback().CurrentTrack()
	if track == nil {
		m.Popups.ShowError("No track playing")
		return nil
	}

	return m.Popups.ShowLyrics(
		m.LyricsSource,
		track.Path,
		track.Artist,
		track.Title,
		track.Album,
		m.Core.Playback().Duration(),
	)
}

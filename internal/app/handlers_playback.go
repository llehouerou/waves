// internal/app/handlers_playback.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/playlist"
)

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S.
func (m *Model) handlePlaybackKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling playback actions
	case keymap.ActionPlayPause:
		return handler.Handled(m.HandleSpaceAction())
	case keymap.ActionStop:
		_ = m.PlaybackService.Stop()
		m.ResizeComponents()
		return handler.HandledNoCmd
	case keymap.ActionNextTrack:
		return handler.Handled(m.AdvanceToNextTrack())
	case keymap.ActionPrevTrack:
		return handler.Handled(m.GoToPreviousTrack())
	case keymap.ActionFirstTrack:
		if !m.Playback.Queue().IsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(0))
		}
		return handler.HandledNoCmd
	case keymap.ActionLastTrack:
		if !m.Playback.Queue().IsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(m.Playback.Queue().Len() - 1))
		}
		return handler.HandledNoCmd
	case keymap.ActionTogglePlayerDisplay:
		m.TogglePlayerDisplayMode()
		return handler.HandledNoCmd
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
		return handler.Handled(m.handleCycleRepeat())
	case keymap.ActionToggleShuffle:
		m.PlaybackService.ToggleShuffle()
		m.SaveQueueState()
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// handleCycleRepeat cycles through repeat modes, integrating radio mode.
// Cycle: Off -> All -> One -> Radio -> Off
// If Last.fm is not configured, Radio mode is skipped.
func (m *Model) handleCycleRepeat() tea.Cmd {
	currentMode := m.Playback.Queue().RepeatMode()

	// Determine next mode
	nextMode := m.nextRepeatMode(currentMode)

	// Handle radio state transitions
	cmd := m.handleRadioTransition(currentMode, nextMode)

	m.Playback.Queue().SetRepeatMode(nextMode)
	m.SaveQueueState()

	return cmd
}

// nextRepeatMode returns the next repeat mode in the cycle.
func (m *Model) nextRepeatMode(current playlist.RepeatMode) playlist.RepeatMode {
	switch current {
	case playlist.RepeatOff:
		return playlist.RepeatAll
	case playlist.RepeatAll:
		return playlist.RepeatOne
	case playlist.RepeatOne:
		// Only go to Radio mode if Last.fm is configured
		if m.isLastfmLinked() && m.Radio != nil {
			return playlist.RepeatRadio
		}
		return playlist.RepeatOff
	case playlist.RepeatRadio:
		return playlist.RepeatOff
	default:
		return playlist.RepeatOff
	}
}

// handleRadioTransition handles enabling/disabling radio when transitioning modes.
func (m *Model) handleRadioTransition(from, to playlist.RepeatMode) tea.Cmd {
	if m.Radio == nil {
		return nil
	}

	// Leaving radio mode
	if from == playlist.RepeatRadio && to != playlist.RepeatRadio {
		m.Radio.Disable()
		return nil
	}

	// Entering radio mode
	if from != playlist.RepeatRadio && to == playlist.RepeatRadio {
		m.Radio.Enable()
		if track := m.Playback.CurrentTrack(); track != nil {
			m.Radio.SetSeed(track.Artist)
		}
		return func() tea.Msg {
			return RadioToggledMsg{Enabled: true}
		}
	}

	return nil
}

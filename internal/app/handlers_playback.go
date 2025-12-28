// internal/app/handlers_playback.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/playback"
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
		if !m.PlaybackService.QueueIsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(0))
		}
		return handler.HandledNoCmd
	case keymap.ActionLastTrack:
		if !m.PlaybackService.QueueIsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(m.PlaybackService.QueueLen() - 1))
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
	currentMode := m.PlaybackService.RepeatMode()

	// Determine next mode
	nextMode := m.nextRepeatMode(currentMode)

	// Handle radio state transitions
	cmd := m.handleRadioTransition(currentMode, nextMode)

	m.PlaybackService.SetRepeatMode(nextMode)
	m.SaveQueueState()

	return cmd
}

// nextRepeatMode returns the next repeat mode in the cycle.
func (m *Model) nextRepeatMode(current playback.RepeatMode) playback.RepeatMode {
	switch current {
	case playback.RepeatOff:
		return playback.RepeatAll
	case playback.RepeatAll:
		return playback.RepeatOne
	case playback.RepeatOne:
		// Only go to Radio mode if Last.fm is configured
		if m.isLastfmLinked() && m.Radio != nil {
			return playback.RepeatRadio
		}
		return playback.RepeatOff
	case playback.RepeatRadio:
		return playback.RepeatOff
	default:
		return playback.RepeatOff
	}
}

// handleRadioTransition handles enabling/disabling radio when transitioning modes.
func (m *Model) handleRadioTransition(from, to playback.RepeatMode) tea.Cmd {
	if m.Radio == nil {
		return nil
	}

	// Leaving radio mode
	if from == playback.RepeatRadio && to != playback.RepeatRadio {
		m.Radio.Disable()
		return nil
	}

	// Entering radio mode
	if from != playback.RepeatRadio && to == playback.RepeatRadio {
		m.Radio.Enable()
		if track := m.PlaybackService.CurrentTrack(); track != nil {
			m.Radio.SetSeed(track.Artist)
		}
		return func() tea.Msg {
			return RadioToggledMsg{Enabled: true}
		}
	}

	return nil
}

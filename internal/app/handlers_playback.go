// internal/app/handlers_playback.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/keymap"
)

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S.
func (m *Model) handlePlaybackKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling playback actions
	case keymap.ActionPlayPause:
		return handler.Handled(m.HandleSpaceAction())
	case keymap.ActionStop:
		m.Playback.Stop()
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
		m.Playback.Queue().CycleRepeatMode()
		m.SaveQueueState()
		return handler.HandledNoCmd
	case keymap.ActionToggleShuffle:
		m.Playback.Queue().ToggleShuffle()
		m.SaveQueueState()
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

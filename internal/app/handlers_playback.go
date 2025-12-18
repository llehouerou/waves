// internal/app/handlers_playback.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
)

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S.
func (m *Model) handlePlaybackKeys(key string) handler.Result {
	switch key {
	case " ":
		// Space toggles play/pause immediately
		return handler.Handled(m.HandleSpaceAction())
	case "s":
		m.Playback.Stop()
		m.ResizeComponents()
		return handler.HandledNoCmd
	case "pgdown":
		return handler.Handled(m.AdvanceToNextTrack())
	case "pgup":
		return handler.Handled(m.GoToPreviousTrack())
	case "home":
		if !m.Playback.Queue().IsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(0))
		}
		return handler.HandledNoCmd
	case "end":
		if !m.Playback.Queue().IsEmpty() {
			return handler.Handled(m.JumpToQueueIndex(m.Playback.Queue().Len() - 1))
		}
		return handler.HandledNoCmd
	case "v":
		m.TogglePlayerDisplayMode()
		return handler.HandledNoCmd
	case "shift+left":
		m.handleSeek(-5)
		return handler.HandledNoCmd
	case "shift+right":
		m.handleSeek(5)
		return handler.HandledNoCmd
	case "alt+shift+left":
		m.handleSeek(-15)
		return handler.HandledNoCmd
	case "alt+shift+right":
		m.handleSeek(15)
		return handler.HandledNoCmd
	case "R":
		m.Playback.Queue().CycleRepeatMode()
		m.SaveQueueState()
		return handler.HandledNoCmd
	case "S":
		m.Playback.Queue().ToggleShuffle()
		m.SaveQueueState()
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

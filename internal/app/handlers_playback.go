// internal/app/handlers_playback.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S.
func (m *Model) handlePlaybackKeys(key string) (bool, tea.Cmd) {
	switch key {
	case " ":
		// Space toggles play/pause immediately
		return true, m.HandleSpaceAction()
	case "s":
		m.Playback.Stop()
		m.ResizeComponents()
		return true, nil
	case "pgdown":
		return true, m.AdvanceToNextTrack()
	case "pgup":
		return true, m.GoToPreviousTrack()
	case "home":
		if !m.Playback.Queue().IsEmpty() {
			return true, m.JumpToQueueIndex(0)
		}
		return true, nil
	case "end":
		if !m.Playback.Queue().IsEmpty() {
			return true, m.JumpToQueueIndex(m.Playback.Queue().Len() - 1)
		}
		return true, nil
	case "v":
		m.TogglePlayerDisplayMode()
		return true, nil
	case "shift+left":
		m.handleSeek(-5)
		return true, nil
	case "shift+right":
		m.handleSeek(5)
		return true, nil
	case "alt+shift+left":
		m.handleSeek(-15)
		return true, nil
	case "alt+shift+right":
		m.handleSeek(15)
		return true, nil
	case "R":
		m.Playback.Queue().CycleRepeatMode()
		m.SaveQueueState()
		return true, nil
	case "S":
		m.Playback.Queue().ToggleShuffle()
		m.SaveQueueState()
		return true, nil
	}
	return false, nil
}

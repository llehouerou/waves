// internal/app/handlers_queue.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleQueueHistoryKeys handles ctrl+z (undo) and ctrl+shift+z (redo).
func (m *Model) handleQueueHistoryKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "ctrl+z":
		if m.Playback.Queue().Undo() {
			m.SaveQueueState()
			m.Layout.QueuePanel().SyncCursor()
		}
		return true, nil
	case "ctrl+shift+z":
		if m.Playback.Queue().Redo() {
			m.SaveQueueState()
			m.Layout.QueuePanel().SyncCursor()
		}
		return true, nil
	}
	return false, nil
}

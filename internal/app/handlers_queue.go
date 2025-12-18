// internal/app/handlers_queue.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
)

// handleQueueHistoryKeys handles ctrl+z (undo) and ctrl+shift+z (redo).
func (m *Model) handleQueueHistoryKeys(key string) handler.Result {
	switch key {
	case "ctrl+z":
		if m.Playback.Queue().Undo() {
			m.SaveQueueState()
			m.Layout.QueuePanel().SyncCursor()
		}
		return handler.HandledNoCmd
	case "ctrl+shift+z":
		if m.Playback.Queue().Redo() {
			m.SaveQueueState()
			m.Layout.QueuePanel().SyncCursor()
		}
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

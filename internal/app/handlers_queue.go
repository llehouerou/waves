// internal/app/handlers_queue.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/keymap"
)

// handleQueueHistoryKeys handles ctrl+z (undo) and ctrl+shift+z (redo).
func (m *Model) handleQueueHistoryKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling history actions
	case keymap.ActionUndo:
		if m.PlaybackService.Undo() {
			m.SaveQueueState()
			m.Layout.QueuePanel().SyncCursor()
		}
		return handler.HandledNoCmd
	case keymap.ActionRedo:
		if m.PlaybackService.Redo() {
			m.SaveQueueState()
			m.Layout.QueuePanel().SyncCursor()
		}
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

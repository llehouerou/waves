// internal/app/handlers_queue.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/keymap"
)

// handleQueueHistoryKeys handles ctrl+z (undo) and ctrl+y (redo).
func (m *Model) handleQueueHistoryKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling history actions
	case keymap.ActionUndo:
		if m.Core.Playback().Undo() {
			m.Layout.QueuePanel().SyncCursor()
		}
		return handler.HandledNoCmd
	case keymap.ActionRedo:
		if m.Core.Playback().Redo() {
			m.Layout.QueuePanel().SyncCursor()
		}
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

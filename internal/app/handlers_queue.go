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
			m.queueEdited()
		}
		return handler.HandledNoCmd
	case keymap.ActionRedo:
		if m.Core.Playback().Redo() {
			m.queueEdited()
		}
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// queueEdited tells the queue panel about an undo, a redo or a replace the
// app just made, without waiting for the event: the marks of the entries it
// took out are dropped before a next undo or redo can bring the entries back.
// The cursor goes to the track playing. The panel's own edits only call
// QueueChanged: the cursor stays where the panel put it.
func (m *Model) queueEdited() {
	m.Layout.QueuePanel().QueueChanged()
	m.Layout.QueuePanel().SyncCursor()
}

// internal/app/handlers.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/keymap"
)

// handleQuitKeys handles q and ctrl+c.
func (m *Model) handleQuitKeys(key string) handler.Result {
	if m.Keys.Resolve(key) != keymap.ActionQuit {
		return handler.NotHandled
	}
	// Leaves the session bus, stops playback, saves the queue and the volume
	// a last time, and closes the database.
	_ = m.Core.Close() //nolint:errcheck // Ignore errors during shutdown; app is exiting
	return handler.Handled(tea.Quit)
}

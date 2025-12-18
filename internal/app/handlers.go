// internal/app/handlers.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
)

// handleQuitKeys handles q and ctrl+c.
func (m *Model) handleQuitKeys(key string) handler.Result {
	if key != "q" && key != "ctrl+c" {
		return handler.NotHandled
	}
	m.Playback.Stop()
	m.SaveQueueState()
	m.StateMgr.Close()
	return handler.Handled(tea.Quit)
}

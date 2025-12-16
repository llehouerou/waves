// internal/app/handlers.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleQuitKeys handles q and ctrl+c.
func (m *Model) handleQuitKeys(key string) (bool, tea.Cmd) {
	if key != "q" && key != "ctrl+c" {
		return false, nil
	}
	m.Playback.Stop()
	m.SaveQueueState()
	m.StateMgr.Close()
	return true, tea.Quit
}

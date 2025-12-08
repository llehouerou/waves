// internal/app/update_input.go
package app

import tea "github.com/charmbracelet/bubbletea"

// handleInputMsg routes input-related messages.
func (m Model) handleInputMsg(msg InputMessage) (tea.Model, tea.Cmd) {
	if _, ok := msg.(KeySequenceTimeoutMsg); ok {
		return m.handleKeySequenceTimeout()
	}
	return m, nil
}

// handleKeySequenceTimeout handles timeout for key sequences like space.
func (m Model) handleKeySequenceTimeout() (tea.Model, tea.Cmd) {
	if m.Input.IsKeySequence(" ") {
		m.Input.ClearKeySequence()
		if cmd := m.HandleSpaceAction(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

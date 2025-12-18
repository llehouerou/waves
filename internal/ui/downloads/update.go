package downloads

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages for the downloads view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.IsFocused() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		switch msg.Button { //nolint:exhaustive // Only handling specific mouse events
		case tea.MouseButtonWheelUp:
			m.moveCursor(-1)
		case tea.MouseButtonWheelDown:
			m.moveCursor(1)
		case tea.MouseButtonMiddle:
			// Middle click: toggle expanded view (same as Enter)
			if msg.Action == tea.MouseActionPress {
				m.toggleExpanded()
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.moveCursor(1)
		case "k", "up":
			m.moveCursor(-1)
		case "g":
			m.cursor.JumpStart()
		case "G":
			if len(m.downloads) > 0 {
				m.cursor.JumpEnd(len(m.downloads), m.listHeight())
			}
		case "enter":
			// Toggle expanded view for selected download
			m.toggleExpanded()
		case "i":
			// Open import popup for completed/verified downloads
			if d := m.SelectedDownload(); d != nil && m.isReadyForImport(d) {
				return m, func() tea.Msg {
					return ActionMsg(OpenImport{Download: d})
				}
			}
		case "d", "delete":
			// Delete selected download
			if d := m.SelectedDownload(); d != nil {
				id := d.ID
				return m, func() tea.Msg {
					return ActionMsg(DeleteDownload{ID: id})
				}
			}
		case "D":
			// Clear all completed downloads
			return m, func() tea.Msg {
				return ActionMsg(ClearCompleted{})
			}
		case "r":
			// Request immediate refresh
			return m, func() tea.Msg {
				return ActionMsg(RefreshRequest{})
			}
		}
	}

	return m, nil
}

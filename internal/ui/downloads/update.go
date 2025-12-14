package downloads

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages for the downloads view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Handle wheel scroll
		if msg.Button == tea.MouseButtonWheelUp {
			m.moveCursor(-1)
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			m.moveCursor(1)
			return m, nil
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.moveCursor(1)
		case "k", "up":
			m.moveCursor(-1)
		case "g":
			m.cursor = 0
			m.offset = 0
		case "G":
			if len(m.downloads) > 0 {
				m.cursor = len(m.downloads) - 1
				m.ensureCursorVisible()
			}
		case "enter":
			// Toggle expanded view for selected download
			m.toggleExpanded()
		case "d", "delete":
			// Delete selected download
			if d := m.SelectedDownload(); d != nil {
				return m, func() tea.Msg {
					return DeleteDownloadMsg{ID: d.ID}
				}
			}
		case "D":
			// Clear all completed downloads
			return m, func() tea.Msg {
				return ClearCompletedMsg{}
			}
		case "r":
			// Request immediate refresh
			return m, func() tea.Msg {
				return RefreshRequestMsg{}
			}
		}
	}

	return m, nil
}

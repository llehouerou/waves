package downloads

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

// Update handles messages for the downloads view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.IsFocused() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		result, _ := m.cursor.HandleMouse(msg, len(m.downloads), m.listHeight(), ui.PanelOverhead-1)
		switch result { //nolint:exhaustive // Only handling specific mouse results
		case cursor.MouseScrolled:
			return m, nil
		case cursor.MouseMiddleClick:
			m.toggleExpanded()
		}
		return m, nil

	case tea.KeyMsg:
		// Handle common list navigation keys via cursor
		if m.cursor.HandleKey(msg.String(), len(m.downloads), m.listHeight()) {
			return m, nil
		}
		switch msg.String() {
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

package downloads

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/list"
)

// Update handles messages for the downloads view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// Delegate to list for common handling (navigation, enter, delete, mouse)
	result := m.list.Update(msg, m.list.Len())
	switch result.Action { //nolint:exhaustive // Only handling specific actions
	case list.ActionEnter, list.ActionMiddleClick:
		m.toggleExpanded()
		return m, nil
	case list.ActionDelete:
		if d := m.SelectedDownload(); d != nil {
			id := d.ID
			return m, func() tea.Msg {
				return ActionMsg(DeleteDownload{ID: id})
			}
		}
	}

	// Handle custom keys (only if focused)
	if key, ok := msg.(tea.KeyMsg); ok && m.IsFocused() {
		switch key.String() {
		case "i":
			// Open import popup for completed/verified downloads
			if d := m.SelectedDownload(); d != nil && m.isReadyForImport(d) {
				return m, func() tea.Msg {
					return ActionMsg(OpenImport{Download: d})
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

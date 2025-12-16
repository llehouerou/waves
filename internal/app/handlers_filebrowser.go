// internal/app/handlers_filebrowser.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleFileBrowserKeys handles file browser specific keys (d for delete).
func (m *Model) handleFileBrowserKeys(key string) (bool, tea.Cmd) {
	if m.Navigation.ViewMode() != ViewFileBrowser || !m.Navigation.IsNavigatorFocused() {
		return false, nil
	}

	if key != "d" {
		return false, nil
	}

	selected := m.Navigation.FileNav().Selected()
	if selected == nil {
		return false, nil
	}

	// Build confirmation message
	itemType := "file"
	if selected.IsContainer() {
		itemType = "folder"
	}

	m.Popups.ShowConfirm(
		"Delete",
		"Delete "+itemType+" \""+selected.DisplayName()+"\"?",
		FileDeleteContext{
			Path:  selected.ID(),
			Name:  selected.DisplayName(),
			IsDir: selected.IsContainer(),
		},
	)
	return true, nil
}

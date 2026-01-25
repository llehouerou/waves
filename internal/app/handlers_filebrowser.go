// internal/app/handlers_filebrowser.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/keymap"
)

// handleFileBrowserKeys handles file browser specific keys (d for delete).
func (m *Model) handleFileBrowserKeys(key string) handler.Result {
	if m.Navigation.ViewMode() != navctl.ViewFileBrowser || !m.Navigation.IsNavigatorFocused() {
		return handler.NotHandled
	}

	if m.Keys.Resolve(key) != keymap.ActionDelete {
		return handler.NotHandled
	}

	selected := m.Navigation.FileNav().Selected()
	if selected == nil {
		return handler.NotHandled
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
	return handler.HandledNoCmd
}

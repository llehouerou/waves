package downloads

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/ui/list"
)

// keys resolves downloads-view keys against their own context, so keys shared
// with other views ("i", "ctrl+r") don't collide in the global resolver.
var keys = keymap.NewResolver(keymap.ByContext(keymap.ContextDownloads))

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

	// Handle view keys (only if focused), resolved against the downloads context
	if key, ok := msg.(tea.KeyMsg); ok && m.IsFocused() {
		switch keys.Resolve(key.String()) { //nolint:exhaustive // Only handling downloads actions
		case keymap.ActionImportDownload:
			// Open import popup for completed/verified downloads
			d := m.SelectedDownload()
			if m.isReadyForImport(d) {
				return m, func() tea.Msg {
					return ActionMsg(OpenImport{Download: d})
				}
			}
			// Show why import is not possible
			if reason := m.importBlockedReason(d); reason != "" {
				return m, func() tea.Msg {
					return ActionMsg(ImportNotReady{Reason: reason})
				}
			}
		case keymap.ActionClearCompleted:
			return m, func() tea.Msg {
				return ActionMsg(ClearCompleted{})
			}
		case keymap.ActionRefreshDownloads:
			return m, func() tea.Msg {
				return ActionMsg(RefreshRequest{})
			}
		case keymap.ActionRetryDownload:
			// Retry failed files of the selected download (no-op if none)
			if d := m.SelectedDownload(); d != nil && len(d.FailedFiles()) > 0 {
				return m, func() tea.Msg {
					return ActionMsg(RetryFailed{Download: d})
				}
			}
		}
	}

	return m, nil
}

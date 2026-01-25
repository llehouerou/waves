// internal/app/handlers_navigation.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/keymap"
)

// handleViewKeys handles F1, F2, F3, F4 view switching.
func (m *Model) handleViewKeys(key string) handler.Result {
	var newMode navctl.ViewMode
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling view switching actions
	case keymap.ActionViewLibrary:
		newMode = navctl.ViewLibrary
	case keymap.ActionViewFileBrowser:
		newMode = navctl.ViewFileBrowser
	case keymap.ActionViewPlaylists:
		newMode = navctl.ViewPlaylists
	case keymap.ActionViewDownloads:
		newMode = navctl.ViewDownloads
	default:
		return handler.NotHandled
	}

	var cmd tea.Cmd
	if m.Navigation.ViewMode() != newMode {
		m.Navigation.SetViewMode(newMode)
		m.SetFocus(navctl.FocusNavigator)
		m.SaveNavigationState()

		// Start downloads refresh when switching to downloads view (if configured)
		if newMode == navctl.ViewDownloads && m.HasSlskdConfig {
			cmd = m.loadAndRefreshDownloads()
		}
	}
	return handler.Handled(cmd)
}

// loadAndRefreshDownloads loads downloads from DB and starts refresh tick.
func (m *Model) loadAndRefreshDownloads() tea.Cmd {
	// Load current downloads from database
	downloads, err := m.Downloads.List()
	if err == nil {
		m.DownloadsView.SetDownloads(downloads)
	}

	// Start periodic refresh
	return func() tea.Msg {
		return DownloadsRefreshMsg{}
	}
}

// handleFocusKeys handles tab and p (queue toggle).
func (m *Model) handleFocusKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling focus actions
	case keymap.ActionToggleQueue:
		m.Layout.ToggleQueue()
		if !m.Layout.IsQueueVisible() && m.Navigation.IsQueueFocused() {
			m.SetFocus(navctl.FocusNavigator)
		}
		m.ResizeComponents()
		m.Layout.QueuePanel().SyncCursor()
		return handler.HandledNoCmd
	case keymap.ActionSwitchFocus:
		if m.Layout.IsQueueVisible() {
			if m.Navigation.IsQueueFocused() {
				m.SetFocus(navctl.FocusNavigator)
			} else {
				m.SetFocus(navctl.FocusQueue)
			}
		}
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// handleHelpKey handles '?' to show help popup.
func (m *Model) handleHelpKey(key string) handler.Result {
	if m.Keys.Resolve(key) != keymap.ActionHelp {
		return handler.NotHandled
	}
	m.Popups.ShowHelp(m.applicableContexts())
	return handler.HandledNoCmd
}

// applicableContexts returns the binding contexts relevant to the current state.
func (m *Model) applicableContexts() []string {
	contexts := []string{"global", "playback"}

	switch m.Navigation.Focus() {
	case navctl.FocusNavigator:
		contexts = append(contexts, "navigator")
		switch m.Navigation.ViewMode() {
		case navctl.ViewPlaylists:
			contexts = append(contexts, "playlist", "playlist-track")
		case navctl.ViewLibrary:
			contexts = append(contexts, "library")
			if m.Navigation.IsAlbumViewActive() {
				contexts = append(contexts, "albumview")
			}
		case navctl.ViewFileBrowser:
			contexts = append(contexts, "filebrowser")
		case navctl.ViewDownloads:
			contexts = append(contexts, "downloads")
		}
	case navctl.FocusQueue:
		contexts = append(contexts, "queue")
	}

	return contexts
}

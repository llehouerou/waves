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

		// Show the downloads as recorded, then as slskd sees them now
		if newMode == navctl.ViewDownloads && m.Core.Downloads().Configured() {
			cmd = m.loadAndRefreshDownloads()
		}
	}
	return handler.Handled(cmd)
}

// loadAndRefreshDownloads shows the downloads as recorded and asks the core
// for a Sync now: what it changes comes back as an event of the core.
func (m *Model) loadAndRefreshDownloads() tea.Cmd {
	m.Core.Downloads().Sync()
	return loadDownloadsCmd(m.Core.Downloads())
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
	m.Popups.ShowHelp(m.helpScope())
	return handler.HandledNoCmd
}

// helpScope returns what the help lists in the current state: the binding
// contexts that apply, and the actions of those contexts that do nothing in
// the view shown.
func (m *Model) helpScope() (contexts []string, inactive []keymap.Action) {
	contexts = []string{"global", "playback"}
	if !m.Navigation.ViewMode().HasSearch() {
		inactive = []keymap.Action{keymap.ActionSearch, keymap.ActionDeepSearch}
	}

	switch m.Navigation.Focus() {
	case navctl.FocusNavigator:
		switch m.Navigation.ViewMode() {
		case navctl.ViewPlaylists:
			contexts = append(contexts, "navigator", "playlist", "playlist-track")
		case navctl.ViewLibrary:
			contexts = append(contexts, "navigator", "library")
			if m.Navigation.IsAlbumViewActive() {
				contexts = append(contexts, "albumview")
			}
		case navctl.ViewFileBrowser:
			contexts = append(contexts, "navigator", "filebrowser")
		case navctl.ViewDownloads:
			// The downloads view is no navigator: its context holds its keys.
			contexts = append(contexts, "downloads")
		}
	case navctl.FocusQueue:
		contexts = append(contexts, "queue")
	}

	return contexts, inactive
}

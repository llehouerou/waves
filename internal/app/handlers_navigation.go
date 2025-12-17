// internal/app/handlers_navigation.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleViewKeys handles F1, F2, F3, F4 view switching.
func (m *Model) handleViewKeys(key string) (bool, tea.Cmd) {
	var newMode ViewMode
	switch key {
	case "f1":
		newMode = ViewLibrary
	case "f2":
		newMode = ViewFileBrowser
	case "f3":
		newMode = ViewPlaylists
	case "f4":
		// F4 requires slskd config
		if !m.HasSlskdConfig {
			return false, nil
		}
		newMode = ViewDownloads
	default:
		return false, nil
	}

	var cmd tea.Cmd
	if m.Navigation.ViewMode() != newMode {
		m.Navigation.SetViewMode(newMode)
		m.SetFocus(FocusNavigator)
		m.SaveNavigationState()

		// Start downloads refresh when switching to downloads view
		if newMode == ViewDownloads {
			cmd = m.loadAndRefreshDownloads()
		}
	}
	return true, cmd
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
func (m *Model) handleFocusKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "p":
		m.Layout.ToggleQueue()
		if !m.Layout.IsQueueVisible() && m.Navigation.IsQueueFocused() {
			m.SetFocus(FocusNavigator)
		}
		m.ResizeComponents()
		return true, nil
	case "tab":
		if m.Layout.IsQueueVisible() {
			if m.Navigation.IsQueueFocused() {
				m.SetFocus(FocusNavigator)
			} else {
				m.SetFocus(FocusQueue)
			}
		}
		return true, nil
	}
	return false, nil
}

// handleHelpKey handles '?' to show help popup.
func (m *Model) handleHelpKey(key string) (bool, tea.Cmd) {
	if key != "?" {
		return false, nil
	}
	m.Popups.ShowHelp(m.applicableContexts())
	return true, nil
}

// applicableContexts returns the binding contexts relevant to the current state.
func (m *Model) applicableContexts() []string {
	contexts := []string{"global", "playback"}

	switch m.Navigation.Focus() {
	case FocusNavigator:
		contexts = append(contexts, "navigator")
		switch m.Navigation.ViewMode() {
		case ViewPlaylists:
			contexts = append(contexts, "playlist", "playlist-track")
		case ViewLibrary:
			contexts = append(contexts, "library")
			if m.Navigation.IsAlbumViewActive() {
				contexts = append(contexts, "albumview")
			}
		case ViewFileBrowser:
			contexts = append(contexts, "filebrowser")
		case ViewDownloads:
			contexts = append(contexts, "downloads")
		}
	case FocusQueue:
		contexts = append(contexts, "queue")
	}

	return contexts
}

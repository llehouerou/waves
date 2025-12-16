// internal/app/handlers_navigator.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/search"
)

// handleNavigatorActionKeys handles enter, a, /, ctrl+a.
func (m *Model) handleNavigatorActionKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "/":
		switch m.Navigation.ViewMode() {
		case ViewFileBrowser:
			m.Input.StartLocalSearch(m.CurrentDirSearchItems())
		case ViewLibrary:
			m.Input.StartLocalSearch(m.CurrentLibrarySearchItems())
		case ViewPlaylists:
			m.Input.StartLocalSearch(m.CurrentPlaylistSearchItems())
		case ViewDownloads:
			// No local search for downloads view
		}
		return true, nil
	case "enter": //nolint:goconst // constant is in test file
		// Album view handles its own enter key
		if m.Navigation.IsAlbumViewActive() {
			return false, nil
		}
		return m.handleEnterKey()
	case "a":
		// Album view handles its own 'a' key
		if m.Navigation.IsAlbumViewActive() {
			return false, nil
		}
		if m.Navigation.IsNavigatorFocused() {
			if cmd := m.HandleQueueAction(QueueAdd); cmd != nil {
				return true, cmd
			}
		}
	case "ctrl+a":
		if m.Navigation.IsNavigatorFocused() && m.Navigation.ViewMode() == ViewLibrary {
			return m.handleAddToPlaylist()
		}
	}
	return false, nil
}

// handleEnterKey handles the enter key for navigator views (not album view).
func (m *Model) handleEnterKey() (bool, tea.Cmd) {
	if !m.Navigation.IsNavigatorFocused() {
		return false, nil
	}
	// Container selected: replace queue with contents, play first track
	// Track selected: replace queue with parent container, play selected track
	if m.isSelectedItemContainer() {
		if cmd := m.HandleQueueAction(QueueReplace); cmd != nil {
			return true, cmd
		}
	} else if cmd := m.HandleContainerAndPlay(); cmd != nil {
		return true, cmd
	}
	return false, nil
}

// handleAddToPlaylist initiates the add-to-playlist flow.
func (m *Model) handleAddToPlaylist() (bool, tea.Cmd) {
	selected := m.Navigation.LibraryNav().Selected()
	if selected == nil {
		return true, nil
	}

	// Collect track IDs to add
	trackIDs, err := m.Library.CollectTrackIDs(*selected)
	if err != nil || len(trackIDs) == 0 {
		return true, nil
	}

	// Get playlists for search
	items, err := m.Playlists.AllForAddToPlaylist()
	if err != nil || len(items) == 0 {
		return true, nil
	}

	// Convert to search items
	searchItems := make([]search.Item, len(items))
	for i, item := range items {
		searchItems[i] = item
	}

	m.Input.StartAddToPlaylistSearch(trackIDs, searchItems)
	return true, nil
}

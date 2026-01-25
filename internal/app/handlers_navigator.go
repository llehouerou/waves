// internal/app/handlers_navigator.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/search"
)

// handleNavigatorActionKeys handles enter, a, /, ctrl+a.
func (m *Model) handleNavigatorActionKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling navigator actions
	case keymap.ActionSearch:
		switch m.Navigation.ViewMode() {
		case navctl.ViewFileBrowser:
			m.Input.StartLocalSearch(m.CurrentDirSearchItems())
		case navctl.ViewLibrary:
			// In album view, use album search (same as ff)
			if m.Navigation.IsAlbumViewActive() {
				searchFn := func(query string) ([]search.Item, error) {
					results, err := m.Library.SearchAlbumsFTS(query)
					if err != nil {
						return nil, err
					}
					items := make([]search.Item, len(results))
					for i, r := range results {
						items[i] = library.SearchItem{Result: r}
					}
					return items, nil
				}
				m.Input.StartDeepSearchWithFunc(searchFn)
			} else {
				m.Input.StartLocalSearch(m.CurrentLibrarySearchItems())
			}
		case navctl.ViewPlaylists:
			m.Input.StartLocalSearch(m.CurrentPlaylistSearchItems())
		case navctl.ViewDownloads:
			// No local search for downloads view
		}
		return handler.HandledNoCmd
	case keymap.ActionSelect:
		// Album view handles its own enter key
		if m.Navigation.IsAlbumViewActive() {
			return handler.NotHandled
		}
		return m.handleEnterKey()
	case keymap.ActionAdd:
		// Album view handles its own 'a' key
		if m.Navigation.IsAlbumViewActive() {
			return handler.NotHandled
		}
		if m.Navigation.IsNavigatorFocused() {
			if cmd := m.HandleQueueAction(QueueAdd); cmd != nil {
				return handler.Handled(cmd)
			}
		}
	case keymap.ActionAddToPlaylist:
		if m.Navigation.IsNavigatorFocused() && m.Navigation.ViewMode() == navctl.ViewLibrary {
			return m.handleAddToPlaylist()
		}
	}
	return handler.NotHandled
}

// handleEnterKey handles the enter key for navigator views (not album view).
func (m *Model) handleEnterKey() handler.Result {
	if !m.Navigation.IsNavigatorFocused() {
		return handler.NotHandled
	}
	// Container selected: replace queue with contents, play first track
	// Track selected: replace queue with parent container, play selected track
	if m.isSelectedItemContainer() {
		if cmd := m.HandleQueueAction(QueueReplace); cmd != nil {
			return handler.Handled(cmd)
		}
	} else if cmd := m.HandleContainerAndPlay(); cmd != nil {
		return handler.Handled(cmd)
	}
	return handler.NotHandled
}

// handleAddToPlaylist initiates the add-to-playlist flow.
func (m *Model) handleAddToPlaylist() handler.Result {
	selected := m.Navigation.LibraryNav().Selected()
	if selected == nil {
		return handler.HandledNoCmd
	}

	// Collect track IDs to add
	trackIDs, err := m.Library.CollectTrackIDs(*selected)
	if err != nil || len(trackIDs) == 0 {
		return handler.HandledNoCmd
	}

	// Get playlists for search
	items, err := m.Playlists.AllForAddToPlaylist()
	if err != nil || len(items) == 0 {
		return handler.HandledNoCmd
	}

	// Convert to search items
	searchItems := make([]search.Item, len(items))
	for i, item := range items {
		searchItems[i] = item
	}

	m.Input.StartAddToPlaylistSearch(trackIDs, searchItems)
	return handler.HandledNoCmd
}

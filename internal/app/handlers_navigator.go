// internal/app/handlers_navigator.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
)

// keyForBrowserRight is a synthetic key message used to move the browser column right.
var keyForBrowserRight = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}

// handleNavigatorActionKeys handles enter, a, /, ctrl+a.
func (m *Model) handleNavigatorActionKeys(key string) handler.Result {
	switch m.Keys.Resolve(key) { //nolint:exhaustive // only handling navigator actions
	case keymap.ActionSearch:
		switch m.Navigation.ViewMode() {
		case navctl.ViewFileBrowser:
			m.Input.StartLocalSearch(m.CurrentDirSearchItems())
		case navctl.ViewLibrary:
			if m.Navigation.IsAlbumViewActive() || m.Navigation.IsBrowserViewActive() {
				// Album view and browser view use FTS deep search
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
		// Browser mode: handle enter based on active column
		if m.Navigation.IsBrowserViewActive() {
			return m.handleBrowserEnterKey()
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

// handleBrowserEnterKey handles the enter key for browser mode.
func (m *Model) handleBrowserEnterKey() handler.Result {
	if !m.Navigation.IsNavigatorFocused() {
		return handler.NotHandled
	}
	browser := m.Navigation.LibraryBrowser()
	switch browser.ActiveColumn() {
	case librarybrowser.ColumnArtists:
		// Move to albums column
		cmd := m.Navigation.UpdateActiveNavigator(keyForBrowserRight)
		if cmd != nil {
			return handler.Handled(cmd)
		}
		return handler.HandledNoCmd
	case librarybrowser.ColumnAlbums:
		// Replace queue with album tracks and play
		if cmd := m.HandleQueueAction(QueueReplace); cmd != nil {
			return handler.Handled(cmd)
		}
		return handler.HandledNoCmd
	case librarybrowser.ColumnTracks:
		// Play from selected track
		if cmd := m.HandleContainerAndPlay(); cmd != nil {
			return handler.Handled(cmd)
		}
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// handleAddToPlaylist initiates the add-to-playlist flow.
func (m *Model) handleAddToPlaylist() handler.Result {
	trackIDs := m.collectTrackIDsForPlaylist()
	if len(trackIDs) == 0 {
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

// collectTrackIDsForPlaylist collects track IDs from the current selection for add-to-playlist.
func (m *Model) collectTrackIDsForPlaylist() []int64 {
	if m.Navigation.IsBrowserViewActive() {
		tracks, err := m.collectTracksFromBrowser()
		if err != nil || len(tracks) == 0 {
			return nil
		}
		var ids []int64
		for _, t := range tracks {
			if t.ID > 0 {
				ids = append(ids, t.ID)
			}
		}
		return ids
	}

	selected := m.Navigation.LibraryNav().Selected()
	if selected == nil {
		return nil
	}
	trackIDs, err := m.Library.CollectTrackIDs(*selected)
	if err != nil {
		return nil
	}
	return trackIDs
}

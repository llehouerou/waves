// internal/app/components.go
package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/search"
)

// ResizeComponents updates all component sizes based on current dimensions.
func (m *Model) ResizeComponents() {
	navWidth := m.Layout.NavigatorWidth()
	navHeight := m.NavigatorHeight()

	navSizeMsg := tea.WindowSizeMsg{Width: navWidth, Height: navHeight}
	m.Navigation.ResizeNavigators(navSizeMsg)

	m.Layout.ResizeQueuePanel(navHeight)

	// Update popup dimensions
	m.Popups.SetSize(m.Layout.Width(), m.Layout.Height())
	if m.Popups.IsVisible(PopupHelp) {
		m.Popups.Help().SetSize(m.Layout.Width(), m.Layout.Height())
	}
}

// SetFocus changes focus to the specified target and updates all components.
func (m *Model) SetFocus(target FocusTarget) {
	m.Navigation.SetFocus(target)
	m.Layout.QueuePanel().SetFocused(target == FocusQueue)
}

// HandleLibrarySearchResult navigates to the selected search result.
// For artists/albums: navigates inside to show children.
// For tracks: navigates to parent album and focuses on the track.
func (m *Model) HandleLibrarySearchResult(result library.SearchResult) {
	switch result.Type {
	case library.ResultArtist:
		id := "library:artist:" + result.Artist
		m.Navigation.LibraryNav().NavigateTo(id)
	case library.ResultAlbum:
		id := "library:album:" + result.Artist + ":" + result.Album
		m.Navigation.LibraryNav().NavigateTo(id)
	case library.ResultTrack:
		id := fmt.Sprintf("library:track:%d", result.TrackID)
		m.Navigation.LibraryNav().FocusByID(id)
	}
}

// CurrentDirSearchItems returns current directory items as search items.
func (m *Model) CurrentDirSearchItems() []search.Item {
	nodes := m.Navigation.FileNav().CurrentItems()
	items := make([]search.Item, len(nodes))
	for i, node := range nodes {
		items[i] = navigator.FileItem{
			Path:    node.ID(),
			RelPath: node.DisplayName(),
			IsDir:   node.IsContainer(),
		}
	}
	return items
}

// CurrentLibrarySearchItems returns current level library items for local search.
func (m *Model) CurrentLibrarySearchItems() []search.Item {
	nodes := m.Navigation.LibraryNav().CurrentItems()
	items := make([]search.Item, len(nodes))
	for i, node := range nodes {
		items[i] = library.NodeItem{Node: node}
	}
	return items
}

// CurrentPlaylistSearchItems returns current level playlist items for local search.
func (m *Model) CurrentPlaylistSearchItems() []search.Item {
	nodes := m.Navigation.PlaylistNav().CurrentItems()
	items := make([]search.Item, len(nodes))
	for i, node := range nodes {
		items[i] = playlists.NodeItem{Node: node}
	}
	return items
}

// AllPlaylistSearchItems returns all playlists and tracks for deep search.
func (m *Model) AllPlaylistSearchItems() []search.Item {
	results, err := m.Playlists.AllDeepSearchItems()
	if err != nil {
		return nil
	}
	items := make([]search.Item, len(results))
	for i, r := range results {
		items[i] = r
	}
	return items
}

// refreshLibraryNavigator refreshes the library navigator data.
// If preserveSelection is true, attempts to restore the previous selection.
func (m *Model) refreshLibraryNavigator(preserveSelection bool) {
	m.Navigation.RefreshLibrary(preserveSelection)
}

// refreshPlaylistNavigator refreshes the playlist navigator data.
// If preserveSelection is true, attempts to restore the previous selection.
func (m *Model) refreshPlaylistNavigator(preserveSelection bool) {
	m.Navigation.RefreshPlaylists(preserveSelection)
}

// selectedNode returns the currently selected node from the active navigator.
// Returns nil if no item is selected.
func (m *Model) selectedNode() navigator.Node {
	return m.Navigation.CurrentNavigator()
}

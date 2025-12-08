// internal/app/components.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/search"
)

// ResizeComponents updates all component sizes based on current dimensions.
func (m *Model) ResizeComponents() {
	navWidth := m.NavigatorWidth()
	navHeight := m.NavigatorHeight()

	navSizeMsg := tea.WindowSizeMsg{Width: navWidth, Height: navHeight}
	m.FileNavigator, _ = m.FileNavigator.Update(navSizeMsg)
	m.LibraryNavigator, _ = m.LibraryNavigator.Update(navSizeMsg)
	m.PlaylistNavigator, _ = m.PlaylistNavigator.Update(navSizeMsg)

	if m.QueueVisible {
		m.QueuePanel.SetSize(m.QueueWidth(), navHeight)
	}

	// Update popup dimensions
	m.Popups.SetSize(m.Width, m.Height)
	if m.Popups.IsHelpVisible() {
		m.Popups.Help().SetSize(m.Width, m.Height)
	}
}

// SetFocus changes focus to the specified target.
func (m *Model) SetFocus(target FocusTarget) {
	m.Focus = target
	navFocused := target == FocusNavigator
	m.FileNavigator.SetFocused(navFocused)
	m.LibraryNavigator.SetFocused(navFocused)
	m.PlaylistNavigator.SetFocused(navFocused)
	m.QueuePanel.SetFocused(target == FocusQueue)
}

// HandleLibrarySearchResult navigates to the selected search result.
func (m *Model) HandleLibrarySearchResult(result library.SearchResult) {
	switch result.Type {
	case library.ResultArtist:
		id := "library:artist:" + result.Artist
		m.LibraryNavigator.FocusByID(id)
	case library.ResultAlbum:
		id := "library:album:" + result.Artist + ":" + result.Album
		m.LibraryNavigator.FocusByID(id)
	case library.ResultTrack:
		if result.Path != "" && player.IsMusicFile(result.Path) {
			m.PlayTrack(result.Path)
		}
	}
}

// CurrentDirSearchItems returns current directory items as search items.
func (m *Model) CurrentDirSearchItems() []search.Item {
	nodes := m.FileNavigator.CurrentItems()
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
	nodes := m.LibraryNavigator.CurrentItems()
	items := make([]search.Item, len(nodes))
	for i, node := range nodes {
		items[i] = library.NodeItem{Node: node}
	}
	return items
}

// AllLibrarySearchItems returns all library items for deep search.
func (m *Model) AllLibrarySearchItems() []search.Item {
	results, err := m.Library.AllSearchItems()
	if err != nil {
		return nil
	}
	items := make([]search.Item, len(results))
	for i, r := range results {
		items[i] = library.SearchItem{Result: r}
	}
	return items
}

// refreshLibraryNavigator refreshes the library navigator data.
// If preserveSelection is true, attempts to restore the previous selection.
func (m *Model) refreshLibraryNavigator(preserveSelection bool) {
	var selectedID string
	if preserveSelection {
		selectedID = m.LibraryNavigator.SelectedID()
	}
	m.LibraryNavigator.Refresh()
	if selectedID != "" {
		m.LibraryNavigator.SelectByID(selectedID)
	}
	m.LibraryNavigator.SetFocused(m.Focus == FocusNavigator && m.ViewMode == ViewLibrary)
}

// refreshPlaylistNavigator refreshes the playlist navigator data.
// If preserveSelection is true, attempts to restore the previous selection.
func (m *Model) refreshPlaylistNavigator(preserveSelection bool) {
	var selectedID string
	if preserveSelection {
		selectedID = m.PlaylistNavigator.SelectedID()
	}
	m.PlaylistNavigator.Refresh()
	if selectedID != "" {
		m.PlaylistNavigator.SelectByID(selectedID)
	}
	m.PlaylistNavigator.SetFocused(m.Focus == FocusNavigator && m.ViewMode == ViewPlaylists)
}

// updateActiveNavigator routes a message to the active navigator based on ViewMode.
// It updates the appropriate navigator field and returns the resulting command.
func (m *Model) updateActiveNavigator(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.ViewMode {
	case ViewFileBrowser:
		m.FileNavigator, cmd = m.FileNavigator.Update(msg)
	case ViewLibrary:
		m.LibraryNavigator, cmd = m.LibraryNavigator.Update(msg)
	case ViewPlaylists:
		m.PlaylistNavigator, cmd = m.PlaylistNavigator.Update(msg)
	}
	return cmd
}

// selectedNode returns the currently selected node from the active navigator.
// Returns nil if no item is selected.
func (m *Model) selectedNode() navigator.Node {
	switch m.ViewMode {
	case ViewFileBrowser:
		if sel := m.FileNavigator.Selected(); sel != nil {
			return *sel
		}
	case ViewLibrary:
		if sel := m.LibraryNavigator.Selected(); sel != nil {
			return *sel
		}
	case ViewPlaylists:
		if sel := m.PlaylistNavigator.Selected(); sel != nil {
			return *sel
		}
	}
	return nil
}

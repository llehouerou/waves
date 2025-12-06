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

// CurrentLibrarySearchItems returns all library items for global search.
func (m *Model) CurrentLibrarySearchItems() []search.Item {
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

// IsValidSequencePrefix checks if pending keys could lead to a valid sequence.
func IsValidSequencePrefix(pending string) bool {
	validSequences := []string{" ff", " lr"}
	for _, seq := range validSequences {
		if len(pending) <= len(seq) && seq[:len(pending)] == pending {
			return true
		}
	}
	return false
}

// internal/app/navigation_manager.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/ui/albumview"
)

// LibrarySubMode represents sub-modes within the Library view.
type LibrarySubMode int

const (
	LibraryModeMiller LibrarySubMode = iota // Default Miller columns
	LibraryModeAlbum                        // Album view
)

// NavigationManager manages view modes, focus state, and navigators.
type NavigationManager struct {
	viewMode       ViewMode
	librarySubMode LibrarySubMode
	focus          FocusTarget
	fileNav        navigator.Model[navigator.FileNode]
	libraryNav     navigator.Model[library.Node]
	playlistNav    navigator.Model[playlists.Node]
	albumView      albumview.Model
}

// NewNavigationManager creates a new NavigationManager with default state.
func NewNavigationManager() NavigationManager {
	return NavigationManager{
		viewMode: ViewLibrary,
		focus:    FocusNavigator,
	}
}

// --- View Mode ---

// ViewMode returns the current view mode.
func (n *NavigationManager) ViewMode() ViewMode {
	return n.viewMode
}

// SetViewMode changes the view mode.
func (n *NavigationManager) SetViewMode(mode ViewMode) {
	n.viewMode = mode
}

// --- Library Sub-Mode ---

// LibrarySubMode returns the current library sub-mode.
func (n *NavigationManager) LibrarySubMode() LibrarySubMode {
	return n.librarySubMode
}

// SetLibrarySubMode changes the library sub-mode.
func (n *NavigationManager) SetLibrarySubMode(mode LibrarySubMode) {
	n.librarySubMode = mode
}

// ToggleLibrarySubMode toggles between Miller and Album view.
func (n *NavigationManager) ToggleLibrarySubMode() {
	if n.librarySubMode == LibraryModeMiller {
		n.librarySubMode = LibraryModeAlbum
	} else {
		n.librarySubMode = LibraryModeMiller
	}
}

// IsAlbumViewActive returns true if the album view is currently active.
func (n *NavigationManager) IsAlbumViewActive() bool {
	return n.viewMode == ViewLibrary && n.librarySubMode == LibraryModeAlbum
}

// --- Focus ---

// Focus returns the current focus target.
func (n *NavigationManager) Focus() FocusTarget {
	return n.focus
}

// SetFocus changes focus to the specified target and updates navigator focus states.
func (n *NavigationManager) SetFocus(target FocusTarget) {
	n.focus = target
	navFocused := target == FocusNavigator
	n.fileNav.SetFocused(navFocused)
	n.libraryNav.SetFocused(navFocused && n.librarySubMode == LibraryModeMiller)
	n.playlistNav.SetFocused(navFocused)
	n.albumView.SetFocused(navFocused && n.librarySubMode == LibraryModeAlbum)
}

// IsNavigatorFocused returns true if a navigator has focus.
func (n *NavigationManager) IsNavigatorFocused() bool {
	return n.focus == FocusNavigator
}

// IsQueueFocused returns true if the queue panel has focus.
func (n *NavigationManager) IsQueueFocused() bool {
	return n.focus == FocusQueue
}

// --- Navigator Accessors ---

// FileNav returns a pointer to the file navigator.
func (n *NavigationManager) FileNav() *navigator.Model[navigator.FileNode] {
	return &n.fileNav
}

// LibraryNav returns a pointer to the library navigator.
func (n *NavigationManager) LibraryNav() *navigator.Model[library.Node] {
	return &n.libraryNav
}

// PlaylistNav returns a pointer to the playlist navigator.
func (n *NavigationManager) PlaylistNav() *navigator.Model[playlists.Node] {
	return &n.playlistNav
}

// AlbumView returns a pointer to the album view.
func (n *NavigationManager) AlbumView() *albumview.Model {
	return &n.albumView
}

// SetFileNav sets the file navigator.
func (n *NavigationManager) SetFileNav(nav navigator.Model[navigator.FileNode]) {
	n.fileNav = nav
}

// SetLibraryNav sets the library navigator.
func (n *NavigationManager) SetLibraryNav(nav navigator.Model[library.Node]) {
	n.libraryNav = nav
}

// SetPlaylistNav sets the playlist navigator.
func (n *NavigationManager) SetPlaylistNav(nav navigator.Model[playlists.Node]) {
	n.playlistNav = nav
}

// SetAlbumView sets the album view model.
func (n *NavigationManager) SetAlbumView(av albumview.Model) {
	n.albumView = av
}

// --- Navigation Helpers ---

// CurrentNavigator returns the currently active navigator based on view mode.
// Returns the appropriate navigator wrapped as a generic interface.
func (n *NavigationManager) CurrentNavigator() navigator.Node {
	switch n.viewMode {
	case ViewFileBrowser:
		if sel := n.fileNav.Selected(); sel != nil {
			return *sel
		}
	case ViewLibrary:
		if sel := n.libraryNav.Selected(); sel != nil {
			return *sel
		}
	case ViewPlaylists:
		if sel := n.playlistNav.Selected(); sel != nil {
			return *sel
		}
	case ViewDownloads:
		// Downloads view doesn't have a navigator
		return nil
	}
	return nil
}

// UpdateActiveNavigator routes a message to the active navigator based on view mode.
func (n *NavigationManager) UpdateActiveNavigator(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch n.viewMode {
	case ViewFileBrowser:
		n.fileNav, cmd = n.fileNav.Update(msg)
	case ViewLibrary:
		if n.librarySubMode == LibraryModeAlbum {
			n.albumView, cmd = n.albumView.Update(msg)
		} else {
			n.libraryNav, cmd = n.libraryNav.Update(msg)
		}
	case ViewPlaylists:
		n.playlistNav, cmd = n.playlistNav.Update(msg)
	case ViewDownloads:
		// Downloads view is handled separately, not via navigator
	}
	return cmd
}

// ResizeNavigators updates all navigator sizes.
func (n *NavigationManager) ResizeNavigators(msg tea.WindowSizeMsg) {
	n.fileNav, _ = n.fileNav.Update(msg)
	n.libraryNav, _ = n.libraryNav.Update(msg)
	n.playlistNav, _ = n.playlistNav.Update(msg)
	n.albumView.SetSize(msg.Width, msg.Height)
}

// RefreshLibrary refreshes the library navigator data.
// If preserveSelection is true, attempts to restore the previous selection.
func (n *NavigationManager) RefreshLibrary(preserveSelection bool) {
	var selectedID string
	if preserveSelection {
		selectedID = n.libraryNav.SelectedID()
	}
	n.libraryNav.Refresh()
	if selectedID != "" {
		n.libraryNav.SelectByID(selectedID)
	}
	n.libraryNav.SetFocused(n.focus == FocusNavigator && n.viewMode == ViewLibrary)
}

// RefreshPlaylists refreshes the playlist navigator data.
// If preserveSelection is true, attempts to restore the previous selection.
func (n *NavigationManager) RefreshPlaylists(preserveSelection bool) {
	var selectedID string
	if preserveSelection {
		selectedID = n.playlistNav.SelectedID()
	}
	n.playlistNav.Refresh()
	if selectedID != "" {
		n.playlistNav.SelectByID(selectedID)
	}
	n.playlistNav.SetFocused(n.focus == FocusNavigator && n.viewMode == ViewPlaylists)
}

// --- View Rendering ---

// RenderActiveNavigator returns the view for the currently active navigator.
func (n *NavigationManager) RenderActiveNavigator() string {
	switch n.viewMode {
	case ViewFileBrowser:
		return n.fileNav.View()
	case ViewPlaylists:
		return n.playlistNav.View()
	case ViewLibrary:
		if n.librarySubMode == LibraryModeAlbum {
			return n.albumView.View()
		}
		return n.libraryNav.View()
	case ViewDownloads:
		// Downloads view is rendered separately
		return ""
	}
	return ""
}

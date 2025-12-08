// internal/app/handlers.go
package app

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/search"
)

// handleQuitKeys handles q and ctrl+c.
func (m *Model) handleQuitKeys(key string) (bool, tea.Cmd) {
	if key != "q" && key != "ctrl+c" {
		return false, nil
	}
	m.Playback.Stop()
	m.SaveQueueState()
	m.StateMgr.Close()
	return true, tea.Quit
}

// handleViewKeys handles F1, F2, F3 view switching.
func (m *Model) handleViewKeys(key string) (bool, tea.Cmd) {
	var newMode ViewMode
	switch key {
	case "f1":
		newMode = ViewLibrary
	case "f2":
		newMode = ViewFileBrowser
	case "f3":
		newMode = ViewPlaylists
	default:
		return false, nil
	}

	if m.Navigation.ViewMode() != newMode {
		m.Navigation.SetViewMode(newMode)
		m.SetFocus(FocusNavigator)
		m.SaveNavigationState()
	}
	return true, nil
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
		case ViewFileBrowser:
			// no extra contexts
		}
	case FocusQueue:
		contexts = append(contexts, "queue")
	}

	return contexts
}

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S.
func (m *Model) handlePlaybackKeys(key string) (bool, tea.Cmd) {
	switch key {
	case " ":
		// Space toggles play/pause immediately
		return true, m.HandleSpaceAction()
	case "s":
		m.Playback.Stop()
		m.ResizeComponents()
		return true, nil
	case "pgdown":
		return true, m.AdvanceToNextTrack()
	case "pgup":
		return true, m.GoToPreviousTrack()
	case "home":
		if !m.Playback.Queue().IsEmpty() {
			return true, m.JumpToQueueIndex(0)
		}
		return true, nil
	case "end":
		if !m.Playback.Queue().IsEmpty() {
			return true, m.JumpToQueueIndex(m.Playback.Queue().Len() - 1)
		}
		return true, nil
	case "v":
		m.TogglePlayerDisplayMode()
		return true, nil
	case "shift+left":
		m.handleSeek(-5)
		return true, nil
	case "shift+right":
		m.handleSeek(5)
		return true, nil
	case "alt+shift+left":
		m.handleSeek(-15)
		return true, nil
	case "alt+shift+right":
		m.handleSeek(15)
		return true, nil
	case "R":
		m.Playback.Queue().CycleRepeatMode()
		m.SaveQueueState()
		return true, nil
	case "S":
		m.Playback.Queue().ToggleShuffle()
		m.SaveQueueState()
		return true, nil
	}
	return false, nil
}

// handleNavigatorActionKeys handles enter, a, r, alt+enter, /, ctrl+a.
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
		}
		return true, nil
	case "enter":
		if m.Navigation.IsNavigatorFocused() {
			if cmd := m.HandleQueueAction(QueueAddAndPlay); cmd != nil {
				return true, cmd
			}
		}
	case "alt+enter":
		if m.Navigation.IsNavigatorFocused() && m.Navigation.ViewMode().SupportsContainerPlay() {
			if cmd := m.HandleContainerAndPlay(); cmd != nil {
				return true, cmd
			}
		}
	case "a":
		if m.Navigation.IsNavigatorFocused() {
			if cmd := m.HandleQueueAction(QueueAdd); cmd != nil {
				return true, cmd
			}
		}
	case "r":
		if m.Navigation.IsNavigatorFocused() {
			if cmd := m.HandleQueueAction(QueueReplace); cmd != nil {
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

// handlePlaylistKeys handles playlist-specific keys (n/N/ctrl+r/ctrl+d/d/J/K).
func (m *Model) handlePlaylistKeys(key string) (bool, tea.Cmd) {
	if m.Navigation.ViewMode() != ViewPlaylists || !m.Navigation.IsNavigatorFocused() {
		return false, nil
	}

	selected := m.Navigation.PlaylistNav().Selected()
	current := m.Navigation.PlaylistNav().Current()

	// Creation keys (n/N) - not available inside a playlist
	if key == "n" || key == "N" {
		if current.Level() == playlists.LevelPlaylist {
			return false, nil
		}
		parentFolderID := m.getPlaylistParentFolder(current)
		return m.handlePlaylistCreate(key, parentFolderID)
	}

	// Rename (ctrl+r)
	if key == "ctrl+r" {
		return m.handlePlaylistRename(selected)
	}

	// Delete playlist/folder (ctrl+d)
	if key == "ctrl+d" {
		return m.handlePlaylistDelete(selected)
	}

	// Track operations (d/J/K)
	if key == "d" || key == "J" || key == "K" {
		return m.handlePlaylistTrackOps(key, selected)
	}

	return false, nil
}

// getPlaylistParentFolder returns the parent folder ID for creating new items.
func (m *Model) getPlaylistParentFolder(current playlists.Node) *int64 {
	switch current.Level() {
	case playlists.LevelRoot:
		return nil
	case playlists.LevelFolder:
		return current.FolderID()
	case playlists.LevelPlaylist:
		return current.ParentFolderID()
	case playlists.LevelTrack:
		// Should not happen - tracks are not containers
		return nil
	}
	return nil
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func formatInt(n int) string {
	return strconv.Itoa(n)
}

// refreshPlaylistNavigatorInPlace refreshes the playlist navigator without returning.
func (m *Model) refreshPlaylistNavigatorInPlace() {
	m.Navigation.PlaylistNav().Refresh()
}

// handleLibraryKeys handles library-specific keys (d for delete).
func (m *Model) handleLibraryKeys(key string) (bool, tea.Cmd) {
	if m.Navigation.ViewMode() != ViewLibrary || !m.Navigation.IsNavigatorFocused() {
		return false, nil
	}

	selected := m.Navigation.LibraryNav().Selected()
	if selected == nil {
		return false, nil
	}

	if key != "d" {
		return false, nil
	}

	// Delete track - only at track level
	if selected.Level() != library.LevelTrack {
		return false, nil
	}

	track := selected.Track()
	if track == nil {
		return true, nil
	}

	m.Popups.ShowConfirmWithOptions(
		"Delete Track",
		"Delete \""+track.Title+"\"?",
		[]string{"Remove from library", "Delete from disk", "Cancel"},
		LibraryDeleteContext{
			TrackID:   track.ID,
			TrackPath: track.Path,
			Title:     track.Title,
		},
	)
	return true, nil
}

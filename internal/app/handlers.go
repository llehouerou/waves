// internal/app/handlers.go
package app

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/search"
)

// handleQuitKeys handles q and ctrl+c.
func (m *Model) handleQuitKeys(key string) (bool, tea.Cmd) {
	if key != "q" && key != "ctrl+c" {
		return false, nil
	}
	m.Player.Stop()
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

	if m.ViewMode != newMode {
		m.ViewMode = newMode
		m.SetFocus(FocusNavigator)
		m.SaveNavigationState()
	}
	return true, nil
}

// handleFocusKeys handles tab and p (queue toggle).
func (m *Model) handleFocusKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "p":
		m.QueueVisible = !m.QueueVisible
		if !m.QueueVisible && m.Focus == FocusQueue {
			m.SetFocus(FocusNavigator)
		}
		m.ResizeComponents()
		return true, nil
	case "tab":
		if m.QueueVisible {
			if m.Focus == FocusQueue {
				m.SetFocus(FocusNavigator)
			} else {
				m.SetFocus(FocusQueue)
			}
		}
		return true, nil
	}
	return false, nil
}

// handleGPrefixKey handles 'g' key to start a key sequence.
func (m *Model) handleGPrefixKey(key string) (bool, tea.Cmd) {
	if key == "g" && m.Focus == FocusNavigator {
		m.PendingKeys = "g"
		return true, nil
	}
	return false, nil
}

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S.
func (m *Model) handlePlaybackKeys(key string) (bool, tea.Cmd) {
	switch key {
	case " ":
		// Space toggles play/pause immediately
		return true, m.HandleSpaceAction()
	case "s":
		m.Player.Stop()
		m.ResizeComponents()
		return true, nil
	case "pgdown":
		return true, m.AdvanceToNextTrack()
	case "pgup":
		return true, m.GoToPreviousTrack()
	case "home":
		if !m.Queue.IsEmpty() {
			return true, m.JumpToQueueIndex(0)
		}
		return true, nil
	case "end":
		if !m.Queue.IsEmpty() {
			return true, m.JumpToQueueIndex(m.Queue.Len() - 1)
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
	case "R":
		m.Queue.CycleRepeatMode()
		m.SaveQueueState()
		return true, nil
	case "S":
		m.Queue.ToggleShuffle()
		m.SaveQueueState()
		return true, nil
	}
	return false, nil
}

// handleNavigatorActionKeys handles enter, a, r, alt+enter, /, ctrl+a.
func (m *Model) handleNavigatorActionKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "/":
		m.SearchMode = true
		if m.ViewMode == ViewFileBrowser {
			m.Search.SetItems(m.CurrentDirSearchItems())
		} else {
			m.Search.SetItems(m.CurrentLibrarySearchItems())
		}
		m.Search.SetLoading(false)
		return true, nil
	case "enter":
		if m.Focus == FocusNavigator {
			if cmd := m.HandleQueueAction(QueueAddAndPlay); cmd != nil {
				return true, cmd
			}
		}
	case "alt+enter":
		if m.Focus == FocusNavigator && (m.ViewMode == ViewLibrary || m.ViewMode == ViewPlaylists) {
			if cmd := m.HandleContainerAndPlay(); cmd != nil {
				return true, cmd
			}
		}
	case "a":
		if m.Focus == FocusNavigator {
			if cmd := m.HandleQueueAction(QueueAdd); cmd != nil {
				return true, cmd
			}
		}
	case "r":
		if m.Focus == FocusNavigator {
			if cmd := m.HandleQueueAction(QueueReplace); cmd != nil {
				return true, cmd
			}
		}
	case "ctrl+a":
		if m.Focus == FocusNavigator && m.ViewMode == ViewLibrary {
			return m.handleAddToPlaylist()
		}
	}
	return false, nil
}

// handleAddToPlaylist initiates the add-to-playlist flow.
func (m *Model) handleAddToPlaylist() (bool, tea.Cmd) {
	selected := m.LibraryNavigator.Selected()
	if selected == nil {
		return true, nil
	}

	// Collect track IDs to add
	trackIDs, err := m.Library.CollectTrackIDs(*selected)
	if err != nil || len(trackIDs) == 0 {
		return true, nil
	}

	// Get playlists for search
	items, err := m.Playlists.AllForSearch()
	if err != nil || len(items) == 0 {
		return true, nil
	}

	// Convert to search items
	searchItems := make([]search.Item, len(items))
	for i, item := range items {
		searchItems[i] = item
	}

	m.AddToPlaylistMode = true
	m.AddToPlaylistTracks = trackIDs
	m.Search.SetItems(searchItems)
	m.Search.SetLoading(false)
	return true, nil
}

// handlePlaylistKeys handles playlist-specific keys (n/N/R/D).
func (m *Model) handlePlaylistKeys(key string) (bool, tea.Cmd) {
	if m.ViewMode != ViewPlaylists || m.Focus != FocusNavigator {
		return false, nil
	}

	selected := m.PlaylistNavigator.Selected()
	current := m.PlaylistNavigator.Current()

	// Get parent folder for creation based on current container
	var parentFolderID *int64
	switch current.Level() {
	case playlists.LevelRoot:
		parentFolderID = nil
	case playlists.LevelFolder:
		parentFolderID = current.FolderID()
	case playlists.LevelPlaylist:
		// Inside a playlist - use the playlist's containing folder
		parentFolderID = current.ParentFolderID()
	case playlists.LevelTrack:
		// Should not happen - tracks are not containers
		parentFolderID = nil
	}

	switch key {
	case "n":
		// Create new playlist - not available inside a playlist
		if current.Level() == playlists.LevelPlaylist {
			return false, nil
		}
		m.InputMode = InputNewPlaylist
		m.TextInput.Start("New Playlist", "", PlaylistInputContext{
			Mode:     InputNewPlaylist,
			FolderID: parentFolderID,
		}, m.Width, m.Height)
		return true, nil

	case "N":
		// Create new folder - not available inside a playlist
		if current.Level() == playlists.LevelPlaylist {
			return false, nil
		}
		m.InputMode = InputNewFolder
		m.TextInput.Start("New Folder", "", PlaylistInputContext{
			Mode:     InputNewFolder,
			FolderID: parentFolderID,
		}, m.Width, m.Height)
		return true, nil

	case "ctrl+r":
		// Rename selected item
		if selected == nil {
			return true, nil
		}
		level := selected.Level()
		if level == playlists.LevelRoot || level == playlists.LevelTrack {
			// Can't rename root or tracks
			return true, nil
		}

		var isFolder bool
		var itemID int64
		var currentName string

		if folderID := selected.FolderID(); folderID != nil && level == playlists.LevelFolder {
			// It's a folder
			isFolder = true
			itemID = *folderID
			currentName = selected.DisplayName()
		} else if playlistID := selected.PlaylistID(); playlistID != nil {
			// It's a playlist
			isFolder = false
			itemID = *playlistID
			currentName = selected.DisplayName()
		} else {
			return true, nil
		}

		m.InputMode = InputRename
		m.TextInput.Start("Rename", currentName, PlaylistInputContext{
			Mode:     InputRename,
			ItemID:   itemID,
			IsFolder: isFolder,
		}, m.Width, m.Height)
		return true, nil

	case "ctrl+d":
		// Delete selected item
		if selected == nil {
			return true, nil
		}
		level := selected.Level()
		if level == playlists.LevelRoot || level == playlists.LevelTrack {
			// Can't delete root or tracks (use track removal)
			return true, nil
		}

		var isFolder bool
		var itemID int64
		var itemName string
		var isEmpty bool

		if folderID := selected.FolderID(); folderID != nil && level == playlists.LevelFolder {
			isFolder = true
			itemID = *folderID
			itemName = selected.DisplayName()
			empty, err := m.Playlists.IsFolderEmpty(*folderID)
			if err != nil {
				m.ErrorMsg = err.Error()
				return true, nil
			}
			isEmpty = empty
		} else if playlistID := selected.PlaylistID(); playlistID != nil {
			isFolder = false
			itemID = *playlistID
			itemName = selected.DisplayName()
			empty, err := m.Playlists.IsPlaylistEmpty(*playlistID)
			if err != nil {
				m.ErrorMsg = err.Error()
				return true, nil
			}
			isEmpty = empty
		} else {
			return true, nil
		}

		// If not empty, ask for confirmation
		if !isEmpty {
			m.Confirm.Show("Delete", "Delete \""+itemName+"\"?", DeleteConfirmContext{
				ItemID:   itemID,
				IsFolder: isFolder,
			}, m.Width, m.Height)
			return true, nil
		}

		// Empty item, delete directly
		var err error
		if isFolder {
			err = m.Playlists.DeleteFolder(itemID)
		} else {
			err = m.Playlists.Delete(itemID)
		}

		if err != nil {
			m.ErrorMsg = err.Error()
			return true, nil
		}

		// Refresh navigator
		m.refreshPlaylistNavigatorInPlace()
		return true, nil

	case "d":
		// Delete track from playlist
		if selected == nil || selected.Level() != playlists.LevelTrack {
			return false, nil
		}
		playlistID := selected.PlaylistID()
		if playlistID == nil {
			return true, nil
		}
		if err := m.Playlists.RemoveTrack(*playlistID, selected.Position()); err != nil {
			m.ErrorMsg = err.Error()
			return true, nil
		}
		m.refreshPlaylistNavigatorInPlace()
		return true, nil

	case "J":
		// Move track down in playlist
		if selected == nil || selected.Level() != playlists.LevelTrack {
			return false, nil
		}
		playlistID := selected.PlaylistID()
		if playlistID == nil {
			return true, nil
		}
		newPositions, err := m.Playlists.MoveIndices(*playlistID, []int{selected.Position()}, 1)
		if err != nil {
			m.ErrorMsg = err.Error()
			return true, nil
		}
		m.refreshPlaylistNavigatorInPlace()
		// Focus the moved track at its new position
		if len(newPositions) > 0 {
			newID := "playlists:track:" + formatInt64(*playlistID) + ":" + formatInt(newPositions[0])
			m.PlaylistNavigator.FocusByID(newID)
		}
		return true, nil

	case "K":
		// Move track up in playlist
		if selected == nil || selected.Level() != playlists.LevelTrack {
			return false, nil
		}
		playlistID := selected.PlaylistID()
		if playlistID == nil {
			return true, nil
		}
		newPositions, err := m.Playlists.MoveIndices(*playlistID, []int{selected.Position()}, -1)
		if err != nil {
			m.ErrorMsg = err.Error()
			return true, nil
		}
		m.refreshPlaylistNavigatorInPlace()
		// Focus the moved track at its new position
		if len(newPositions) > 0 {
			newID := "playlists:track:" + formatInt64(*playlistID) + ":" + formatInt(newPositions[0])
			m.PlaylistNavigator.FocusByID(newID)
		}
		return true, nil
	}

	return false, nil
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func formatInt(n int) string {
	return strconv.Itoa(n)
}

// refreshPlaylistNavigatorInPlace refreshes the playlist navigator without returning.
func (m *Model) refreshPlaylistNavigatorInPlace() {
	m.PlaylistNavigator.Refresh()
}

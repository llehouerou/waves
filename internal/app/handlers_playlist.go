// internal/app/handlers_playlist.go
package app

import (
	"errors"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/playlists"
)

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

// handlePlaylistCreate handles "n" (new playlist) and "N" (new folder) keys.
func (m *Model) handlePlaylistCreate(key string, parentFolderID *int64) (bool, tea.Cmd) {
	switch key {
	case "n":
		m.Popups.ShowTextInput(InputNewPlaylist, "New Playlist", "", PlaylistInputContext{
			Mode:     InputNewPlaylist,
			FolderID: parentFolderID,
		})
		return true, nil

	case "N":
		m.Popups.ShowTextInput(InputNewFolder, "New Folder", "", PlaylistInputContext{
			Mode:     InputNewFolder,
			FolderID: parentFolderID,
		})
		return true, nil
	}
	return false, nil
}

// handlePlaylistRename handles "ctrl+r" to rename a playlist or folder.
func (m *Model) handlePlaylistRename(selected *playlists.Node) (bool, tea.Cmd) {
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
		// Protect Favorites playlist from renaming
		if playlists.IsFavorites(*playlistID) {
			m.Popups.ShowError("Cannot rename Favorites playlist")
			return true, nil
		}
		isFolder = false
		itemID = *playlistID
		currentName = selected.DisplayName()
	} else {
		return true, nil
	}

	m.Popups.ShowTextInput(InputRename, "Rename", currentName, PlaylistInputContext{
		Mode:     InputRename,
		ItemID:   itemID,
		IsFolder: isFolder,
	})
	return true, nil
}

// handlePlaylistDelete handles "ctrl+d" to delete a playlist or folder.
func (m *Model) handlePlaylistDelete(selected *playlists.Node) (bool, tea.Cmd) {
	if selected == nil {
		return true, nil
	}

	level := selected.Level()
	if level == playlists.LevelRoot || level == playlists.LevelTrack {
		// Can't delete root or tracks (use track removal)
		return true, nil
	}

	isFolder, itemID, itemName, isEmpty, err := m.getPlaylistDeleteInfo(selected, level)
	if err != nil {
		if errors.Is(err, errFavoritesProtected) {
			m.Popups.ShowError("Cannot delete Favorites playlist")
		} else if !errors.Is(err, errNoAction) {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPlaylistDelete, err))
		}
		return true, nil
	}

	// If not empty, ask for confirmation
	if !isEmpty {
		m.Popups.ShowConfirm("Delete", "Delete \""+itemName+"\"?", DeleteConfirmContext{
			ItemID:   itemID,
			IsFolder: isFolder,
		})
		return true, nil
	}

	// Empty item, delete directly
	var delErr error
	if isFolder {
		delErr = m.Playlists.DeleteFolder(itemID)
	} else {
		delErr = m.Playlists.Delete(itemID)
	}

	if delErr != nil {
		op := errmsg.OpPlaylistDelete
		if isFolder {
			op = errmsg.OpFolderDelete
		}
		m.Popups.ShowError(errmsg.Format(op, delErr))
		return true, nil
	}

	// Refresh navigator
	m.refreshPlaylistNavigatorInPlace()
	return true, nil
}

// errNoAction is a sentinel error for actions that should be silently ignored.
var errNoAction = errors.New("")

// errFavoritesProtected is returned when trying to delete/rename the Favorites playlist.
var errFavoritesProtected = errors.New("favorites protected")

// getPlaylistDeleteInfo extracts the info needed for deletion.
// Returns errNoAction if the item can't be deleted (e.g., Favorites playlist).
func (m *Model) getPlaylistDeleteInfo(selected *playlists.Node, level playlists.Level) (isFolder bool, itemID int64, itemName string, isEmpty bool, err error) {
	if folderID := selected.FolderID(); folderID != nil && level == playlists.LevelFolder {
		empty, ferr := m.Playlists.IsFolderEmpty(*folderID)
		if ferr != nil {
			return false, 0, "", false, ferr
		}
		return true, *folderID, selected.DisplayName(), empty, nil
	}

	playlistID := selected.PlaylistID()
	if playlistID == nil {
		return false, 0, "", false, errNoAction
	}

	// Protect Favorites playlist from deletion
	if playlists.IsFavorites(*playlistID) {
		return false, 0, "", false, errFavoritesProtected
	}

	empty, perr := m.Playlists.IsPlaylistEmpty(*playlistID)
	if perr != nil {
		return false, 0, "", false, perr
	}
	return false, *playlistID, selected.DisplayName(), empty, nil
}

// handlePlaylistTrackOps handles "d" (remove track), "J" (move down), "K" (move up).
func (m *Model) handlePlaylistTrackOps(key string, selected *playlists.Node) (bool, tea.Cmd) {
	if selected == nil || selected.Level() != playlists.LevelTrack {
		return false, nil
	}

	playlistID := selected.PlaylistID()
	if playlistID == nil {
		return true, nil
	}

	switch key {
	case "d":
		if err := m.Playlists.RemoveTrack(*playlistID, selected.Position()); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPlaylistRemove, err))
			return true, nil
		}
		m.refreshPlaylistNavigatorInPlace()
		// If removing from Favorites, refresh favorites status in all navigators
		if playlists.IsFavorites(*playlistID) {
			m.RefreshFavorites()
		}
		return true, nil

	case "J":
		newPositions, err := m.Playlists.MoveIndices(*playlistID, []int{selected.Position()}, 1)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPlaylistMove, err))
			return true, nil
		}
		m.refreshPlaylistNavigatorInPlace()
		if len(newPositions) > 0 {
			newID := "playlists:track:" + formatInt64(*playlistID) + ":" + formatInt(newPositions[0])
			m.Navigation.PlaylistNav().FocusByID(newID)
		}
		return true, nil

	case "K":
		newPositions, err := m.Playlists.MoveIndices(*playlistID, []int{selected.Position()}, -1)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPlaylistMove, err))
			return true, nil
		}
		m.refreshPlaylistNavigatorInPlace()
		if len(newPositions) > 0 {
			newID := "playlists:track:" + formatInt64(*playlistID) + ":" + formatInt(newPositions[0])
			m.Navigation.PlaylistNav().FocusByID(newID)
		}
		return true, nil
	}

	return false, nil
}

// internal/app/handlers_playlist.go
package app

import (
	"errors"
	"strconv"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/playlists"
)

// handlePlaylistKeys handles playlist-specific keys (n/N/ctrl+r/ctrl+d/d/J/K).
func (m *Model) handlePlaylistKeys(key string) handler.Result {
	if m.Navigation.ViewMode() != navctl.ViewPlaylists || !m.Navigation.IsNavigatorFocused() {
		return handler.NotHandled
	}

	selected := m.Navigation.PlaylistNav().Selected()
	current := m.Navigation.PlaylistNav().Current()
	action := m.Keys.Resolve(key)

	// Creation keys (n/N) - not available inside a playlist
	if action == keymap.ActionNewPlaylist || action == keymap.ActionNewFolder {
		if current.Level() == playlists.LevelPlaylist {
			return handler.NotHandled
		}
		parentFolderID := m.getPlaylistParentFolder(current)
		return m.handlePlaylistCreate(action, parentFolderID)
	}

	// Rename (ctrl+r)
	if action == keymap.ActionRename {
		return m.handlePlaylistRename(selected)
	}

	// Delete playlist/folder (ctrl+d)
	if action == keymap.ActionDelete && key == "ctrl+d" {
		return m.handlePlaylistDelete(selected)
	}

	// Track operations (d/J/K)
	if action == keymap.ActionDelete || action == keymap.ActionMoveItemDown || action == keymap.ActionMoveItemUp {
		return m.handlePlaylistTrackOps(action, selected)
	}

	return handler.NotHandled
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
func (m *Model) handlePlaylistCreate(action keymap.Action, parentFolderID *int64) handler.Result {
	switch action { //nolint:exhaustive // only handling create actions
	case keymap.ActionNewPlaylist:
		m.Popups.ShowTextInput(InputNewPlaylist, "New Playlist", "", PlaylistInputContext{
			Mode:     InputNewPlaylist,
			FolderID: parentFolderID,
		})
		return handler.HandledNoCmd

	case keymap.ActionNewFolder:
		m.Popups.ShowTextInput(InputNewFolder, "New Folder", "", PlaylistInputContext{
			Mode:     InputNewFolder,
			FolderID: parentFolderID,
		})
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// handlePlaylistRename handles "ctrl+r" to rename a playlist or folder.
func (m *Model) handlePlaylistRename(selected *playlists.Node) handler.Result {
	if selected == nil {
		return handler.HandledNoCmd
	}

	level := selected.Level()
	if level == playlists.LevelRoot || level == playlists.LevelTrack {
		// Can't rename root or tracks
		return handler.HandledNoCmd
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
			return handler.HandledNoCmd
		}
		isFolder = false
		itemID = *playlistID
		currentName = selected.DisplayName()
	} else {
		return handler.HandledNoCmd
	}

	m.Popups.ShowTextInput(InputRename, "Rename", currentName, PlaylistInputContext{
		Mode:     InputRename,
		ItemID:   itemID,
		IsFolder: isFolder,
	})
	return handler.HandledNoCmd
}

// handlePlaylistDelete handles "ctrl+d" to delete a playlist or folder.
func (m *Model) handlePlaylistDelete(selected *playlists.Node) handler.Result {
	if selected == nil {
		return handler.HandledNoCmd
	}

	level := selected.Level()
	if level == playlists.LevelRoot || level == playlists.LevelTrack {
		// Can't delete root or tracks (use track removal)
		return handler.HandledNoCmd
	}

	isFolder, itemID, itemName, isEmpty, err := m.getPlaylistDeleteInfo(selected, level)
	if err != nil {
		if errors.Is(err, errFavoritesProtected) {
			m.Popups.ShowError("Cannot delete Favorites playlist")
		} else if !errors.Is(err, errNoAction) {
			m.Popups.ShowOpError(errmsg.OpPlaylistDelete, err)
		}
		return handler.HandledNoCmd
	}

	// If not empty, ask for confirmation
	if !isEmpty {
		m.Popups.ShowConfirm("Delete", "Delete \""+itemName+"\"?", DeleteConfirmContext{
			ItemID:   itemID,
			IsFolder: isFolder,
		})
		return handler.HandledNoCmd
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
		m.Popups.ShowOpError(op, delErr)
		return handler.HandledNoCmd
	}

	// Refresh navigator
	m.refreshPlaylistNavigatorInPlace()
	return handler.HandledNoCmd
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
func (m *Model) handlePlaylistTrackOps(action keymap.Action, selected *playlists.Node) handler.Result {
	if selected == nil || selected.Level() != playlists.LevelTrack {
		return handler.NotHandled
	}

	playlistID := selected.PlaylistID()
	if playlistID == nil {
		return handler.HandledNoCmd
	}

	switch action { //nolint:exhaustive // only handling track operations
	case keymap.ActionDelete:
		if err := m.Playlists.RemoveTrack(*playlistID, selected.Position()); err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaylistRemove, err)
			return handler.HandledNoCmd
		}
		m.refreshPlaylistNavigatorInPlace()
		// If removing from Favorites, refresh favorites status in all navigators
		if playlists.IsFavorites(*playlistID) {
			m.RefreshFavorites()
		}
		return handler.HandledNoCmd

	case keymap.ActionMoveItemDown:
		newPositions, err := m.Playlists.MoveIndices(*playlistID, []int{selected.Position()}, 1)
		if err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaylistMove, err)
			return handler.HandledNoCmd
		}
		m.refreshPlaylistNavigatorInPlace()
		if len(newPositions) > 0 {
			newID := "playlists:track:" + formatInt64(*playlistID) + ":" + formatInt(newPositions[0])
			m.Navigation.PlaylistNav().FocusByID(newID)
		}
		return handler.HandledNoCmd

	case keymap.ActionMoveItemUp:
		newPositions, err := m.Playlists.MoveIndices(*playlistID, []int{selected.Position()}, -1)
		if err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaylistMove, err)
			return handler.HandledNoCmd
		}
		m.refreshPlaylistNavigatorInPlace()
		if len(newPositions) > 0 {
			newID := "playlists:track:" + formatInt64(*playlistID) + ":" + formatInt(newPositions[0])
			m.Navigation.PlaylistNav().FocusByID(newID)
		}
		return handler.HandledNoCmd
	}

	return handler.NotHandled
}

// internal/app/update_messages.go
package app

import (
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/confirm"
	"github.com/llehouerou/waves/internal/ui/textinput"
)

func (m Model) handleSearchResult(msg search.ResultMsg) (tea.Model, tea.Cmd) {
	// Handle add-to-playlist mode
	if m.Input.IsAddToPlaylistSearch() {
		return m.handleAddToPlaylistResult(msg)
	}

	// Process the result before clearing search state
	if !msg.Canceled && msg.Item != nil {
		// Auto-focus navigator when selecting a search result
		m.SetFocus(FocusNavigator)

		switch item := msg.Item.(type) {
		case navigator.FileItem:
			m.Navigation.FileNav().NavigateTo(item.Path)
		case library.SearchItem:
			m.HandleLibrarySearchResult(item.Result)
		case library.NodeItem:
			m.Navigation.LibraryNav().FocusByID(item.Node.ID())
		case playlists.NodeItem:
			m.Navigation.PlaylistNav().FocusByID(item.Node.ID())
		case playlists.DeepSearchItem:
			// Navigate to the selected playlist or track (deep search result)
			m.Navigation.PlaylistNav().FocusByID(item.NodeID())
		}
	}

	m.Input.EndSearch()
	return m, nil
}

func (m Model) handleAddToPlaylistResult(msg search.ResultMsg) (tea.Model, tea.Cmd) {
	trackIDs := m.Input.AddToPlaylistTracks()
	m.Input.EndSearch()

	if msg.Canceled || msg.Item == nil {
		return m, nil
	}

	item, ok := msg.Item.(playlists.SearchItem)
	if !ok {
		return m, nil
	}

	if err := m.Playlists.AddTracks(item.ID, trackIDs); err != nil {
		m.Popups.ShowError(err.Error())
		return m, nil
	}

	// Update last used timestamp
	_ = m.Playlists.UpdateLastUsed(item.ID)

	// Refresh playlist navigator so new tracks are visible
	m.refreshPlaylistNavigator(true)

	return m, nil
}

func (m Model) handleTextInputResult(msg textinput.ResultMsg) (tea.Model, tea.Cmd) {
	m.Popups.Hide(PopupTextInput)

	if msg.Canceled || msg.Text == "" {
		return m, nil
	}

	ctx, ok := msg.Context.(PlaylistInputContext)
	if !ok {
		return m, nil
	}

	var navigateToID string

	switch ctx.Mode {
	case InputNone:
		// No action
	case InputNewPlaylist:
		id, err := m.Playlists.Create(ctx.FolderID, msg.Text)
		if err != nil {
			m.Popups.ShowError(err.Error())
			return m, nil
		}
		navigateToID = "playlists:playlist:" + strconv.FormatInt(id, 10)
	case InputNewFolder:
		id, err := m.Playlists.CreateFolder(ctx.FolderID, msg.Text)
		if err != nil {
			m.Popups.ShowError(err.Error())
			return m, nil
		}
		navigateToID = "playlists:folder:" + strconv.FormatInt(id, 10)
	case InputRename:
		var err error
		if ctx.IsFolder {
			err = m.Playlists.RenameFolder(ctx.ItemID, msg.Text)
		} else {
			err = m.Playlists.Rename(ctx.ItemID, msg.Text)
		}
		if err != nil {
			m.Popups.ShowError(err.Error())
			return m, nil
		}
	}

	// Refresh and navigate to newly created item
	m.Navigation.PlaylistNav().Refresh()
	if navigateToID != "" {
		m.Navigation.PlaylistNav().NavigateTo(navigateToID)
	}
	return m, nil
}

func (m Model) handleConfirmResult(msg confirm.ResultMsg) (tea.Model, tea.Cmd) {
	m.Popups.Hide(PopupConfirm)

	if !msg.Confirmed {
		return m, nil
	}

	// Handle library delete context
	if ctx, ok := msg.Context.(LibraryDeleteContext); ok {
		return m.handleLibraryDeleteConfirm(ctx, msg.SelectedOption)
	}

	// Handle file browser delete context
	if ctx, ok := msg.Context.(FileDeleteContext); ok {
		return m.handleFileDeleteConfirm(ctx)
	}

	// Handle playlist delete context
	ctx, ok := msg.Context.(DeleteConfirmContext)
	if !ok {
		return m, nil
	}

	var err error
	if ctx.IsFolder {
		err = m.Playlists.DeleteFolder(ctx.ItemID)
	} else {
		err = m.Playlists.Delete(ctx.ItemID)
	}
	if err != nil {
		m.Popups.ShowError(err.Error())
		return m, nil
	}

	// Refresh playlist navigator
	m.refreshPlaylistNavigator(true)
	return m, nil
}

func (m Model) handleLibraryDeleteConfirm(ctx LibraryDeleteContext, option int) (tea.Model, tea.Cmd) {
	switch option {
	case 0: // Remove from library only
		if err := m.Library.DeleteTrack(ctx.TrackID); err != nil {
			m.Popups.ShowError(err.Error())
			return m, nil
		}
	case 1: // Delete from disk
		if err := os.Remove(ctx.TrackPath); err != nil {
			m.Popups.ShowError("Failed to delete file: " + err.Error())
			return m, nil
		}
		if err := m.Library.DeleteTrack(ctx.TrackID); err != nil {
			m.Popups.ShowError(err.Error())
			return m, nil
		}
	default: // Cancel or unknown
		return m, nil
	}

	// Refresh library navigator
	m.Navigation.LibraryNav().Refresh()
	return m, nil
}

func (m Model) handleFileDeleteConfirm(ctx FileDeleteContext) (tea.Model, tea.Cmd) {
	var err error
	if ctx.IsDir {
		err = os.RemoveAll(ctx.Path)
	} else {
		err = os.Remove(ctx.Path)
	}
	if err != nil {
		m.Popups.ShowError("Failed to delete: " + err.Error())
		return m, nil
	}

	m.Navigation.FileNav().Refresh()
	return m, nil
}

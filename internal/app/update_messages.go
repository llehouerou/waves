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

func (m Model) handleScanResult(msg ScanResultMsg) (tea.Model, tea.Cmd) {
	m.Search.SetItems(msg.Items)
	m.Search.SetLoading(!msg.Done)
	if !msg.Done {
		return m, m.waitForScan()
	}
	return m, nil
}

func (m Model) handleSearchResult(msg search.ResultMsg) (tea.Model, tea.Cmd) {
	// Handle add-to-playlist mode
	if m.AddToPlaylistMode {
		return m.handleAddToPlaylistResult(msg)
	}

	m.SearchMode = false
	m.ScanChan = nil
	if m.CancelScan != nil {
		m.CancelScan()
		m.CancelScan = nil
	}
	if !msg.Canceled && msg.Item != nil {
		switch item := msg.Item.(type) {
		case navigator.FileItem:
			m.FileNavigator.NavigateTo(item.Path)
		case library.SearchItem:
			m.HandleLibrarySearchResult(item.Result)
		case library.NodeItem:
			m.LibraryNavigator.FocusByID(item.Node.ID())
		}
	}
	m.Search.Reset()
	return m, nil
}

func (m Model) handleAddToPlaylistResult(msg search.ResultMsg) (tea.Model, tea.Cmd) {
	m.AddToPlaylistMode = false
	trackIDs := m.AddToPlaylistTracks
	m.AddToPlaylistTracks = nil
	m.Search.Reset()

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
	m.Popups.HideTextInput()

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
	m.PlaylistNavigator.Refresh()
	if navigateToID != "" {
		m.PlaylistNavigator.NavigateTo(navigateToID)
	}
	return m, nil
}

func (m Model) handleConfirmResult(msg confirm.ResultMsg) (tea.Model, tea.Cmd) {
	m.Popups.HideConfirm()

	if !msg.Confirmed {
		return m, nil
	}

	// Handle library delete context
	if ctx, ok := msg.Context.(LibraryDeleteContext); ok {
		return m.handleLibraryDeleteConfirm(ctx, msg.SelectedOption)
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
	m.LibraryNavigator.Refresh()
	return m, nil
}

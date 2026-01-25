package app

import (
	"os"
	"slices"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/export"
	importpopup "github.com/llehouerou/waves/internal/importer/popup"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/navigator/sourceutil"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/retag"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/albumview"
	"github.com/llehouerou/waves/internal/ui/confirm"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
	exportui "github.com/llehouerou/waves/internal/ui/export"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/librarysources"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
	"github.com/llehouerou/waves/internal/ui/textinput"
)

// handleUIAction routes action messages to component-specific handlers.
func (m Model) handleUIAction(msg action.Msg) (tea.Model, tea.Cmd) {
	switch msg.Source {
	case "queuepanel":
		return m.handleQueuePanelAction(msg.Action)
	case "albumview":
		return m.handleAlbumViewAction(msg.Action)
	case "albumview.grouping":
		return m.handleAlbumGroupingAction(msg.Action)
	case "albumview.sorting":
		return m.handleAlbumSortingAction(msg.Action)
	case "albumview.presets":
		return m.handleAlbumPresetsAction(msg.Action)
	case "navigator":
		return m.handleNavigatorAction(msg.Action)
	case "search":
		return m.handleSearchAction(msg.Action)
	case "textinput":
		return m.handleTextInputAction(msg.Action)
	case "confirm":
		return m.handleConfirmAction(msg.Action)
	case "librarysources":
		return m.handleLibrarySourcesAction(msg.Action)
	case "helpbindings":
		return m.handleHelpBindingsAction(msg.Action)
	case "downloads":
		return m.handleDownloadsViewAction(msg.Action)
	case "download":
		return m.handleDownloadPopupAction(msg.Action)
	case "import":
		return m.handleImportPopupAction(msg.Action)
	case "retag":
		return m.handleRetagPopupAction(msg.Action)
	case exportui.Source:
		return m.handleExportPopupAction(msg.Action)
	}
	return m, nil
}

// handleQueuePanelAction handles actions from the queue panel.
func (m Model) handleQueuePanelAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case queuepanel.JumpToTrack:
		cmd := m.PlayTrackAtIndex(act.Index)
		return m, cmd
	case queuepanel.QueueChanged:
		m.SaveQueueState()
		// Clear preloaded track since queue order may have changed
		m.PlaybackService.Player().ClearPreload()
		return m, nil
	case queuepanel.ToggleFavorite:
		m.handleToggleFavorite(act.TrackIDs)
		return m, nil
	case queuepanel.AddToPlaylist:
		m.handleQueueAddToPlaylist(act.TrackIDs)
		return m, nil
	case queuepanel.GoToSource:
		m.handleGoToSource(act)
		return m, nil
	}
	return m, nil
}

// handleQueueAddToPlaylist starts the add-to-playlist flow for queue tracks.
func (m *Model) handleQueueAddToPlaylist(trackIDs []int64) {
	if len(trackIDs) == 0 {
		return
	}

	// Get playlists for search
	items, err := m.Playlists.AllForAddToPlaylist()
	if err != nil || len(items) == 0 {
		return
	}

	// Convert to search items
	searchItems := make([]search.Item, len(items))
	for i, item := range items {
		searchItems[i] = item
	}

	m.Input.StartAddToPlaylistSearch(trackIDs, searchItems)
}

// handleGoToSource navigates to the track's source location in the current view.
func (m *Model) handleGoToSource(act queuepanel.GoToSource) {
	switch m.Navigation.ViewMode() {
	case navctl.ViewLibrary:
		if m.goToSourceLibrary(act) {
			m.SetFocus(navctl.FocusNavigator)
		}
	case navctl.ViewFileBrowser:
		if act.Path != "" && m.Navigation.FileNav().FocusByID(act.Path) {
			m.SetFocus(navctl.FocusNavigator)
		}
	case navctl.ViewPlaylists:
		if act.TrackID > 0 {
			trackNodeID := sourceutil.FormatID("playlists", "track", sourceutil.FormatInt64(act.TrackID))
			if m.Navigation.PlaylistNav().FocusByID(trackNodeID) {
				m.SetFocus(navctl.FocusNavigator)
			}
		}
	case navctl.ViewDownloads:
		// Downloads view doesn't have a source location to navigate to
	}
}

// goToSourceLibrary handles go-to-source for library view modes.
// Returns true if navigation was successful.
func (m *Model) goToSourceLibrary(act queuepanel.GoToSource) bool {
	if act.TrackID == 0 {
		return false
	}

	if m.Navigation.LibrarySubMode() == navctl.LibraryModeAlbum {
		// Album view: select the album (need AlbumArtist from library)
		track, err := m.Library.TrackByID(act.TrackID)
		if err != nil || track == nil {
			return false
		}
		albumID := track.AlbumArtist + ":" + track.Album
		m.Navigation.AlbumView().SelectByID(albumID)
		return true
	}

	// Miller view: navigate to the track
	trackNodeID := sourceutil.FormatID("library", "track", sourceutil.FormatInt64(act.TrackID))
	return m.Navigation.LibraryNav().FocusByID(trackNodeID)
}

// handleAlbumViewAction handles actions from the album view.
func (m Model) handleAlbumViewAction(a action.Action) (tea.Model, tea.Cmd) {
	if act, ok := a.(albumview.QueueAlbum); ok {
		return m.handleAlbumViewQueueAction(act)
	}
	return m, nil
}

// handleAlbumViewQueueAction handles queueing albums.
func (m Model) handleAlbumViewQueueAction(act albumview.QueueAlbum) (tea.Model, tea.Cmd) {
	trackIDs, err := m.Library.AlbumTrackIDs(act.AlbumArtist, act.Album)
	if err != nil || len(trackIDs) == 0 {
		return m, nil
	}

	tracks := make([]playlist.Track, 0, len(trackIDs))
	for _, id := range trackIDs {
		t, err := m.Library.TrackByID(id)
		if err != nil {
			continue
		}
		tracks = append(tracks, playlist.Track{
			ID:          t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		})
	}

	if len(tracks) == 0 {
		return m, nil
	}

	if act.Replace {
		m.PlaybackService.ClearQueue()
	}
	m.PlaybackService.AddTracks(playback.TracksFromPlaylist(tracks)...)
	m.SaveQueueState()
	// Clear preloaded track since queue contents changed
	m.PlaybackService.Player().ClearPreload()

	if act.Replace {
		cmd := m.PlayTrackAtIndex(0)
		return m, cmd
	}

	return m, nil
}

// handleAlbumGroupingAction handles actions from the album grouping popup.
func (m Model) handleAlbumGroupingAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case albumview.GroupingApplied:
		m.Popups.Hide(popupctl.AlbumGrouping)
		av := m.Navigation.AlbumView()
		settings := av.Settings()
		settings.GroupFields = act.Fields
		settings.GroupSortOrder = act.SortOrder
		settings.GroupDateField = act.DateField
		settings.PresetName = "" // Clear preset when manually changed
		av.SetSettings(settings)
		_ = av.Refresh()
		m.SaveNavigationState()
		return m, nil
	case albumview.GroupingCanceled:
		m.Popups.Hide(popupctl.AlbumGrouping)
		return m, nil
	}
	return m, nil
}

// handleAlbumSortingAction handles actions from the album sorting popup.
func (m Model) handleAlbumSortingAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case albumview.SortingApplied:
		m.Popups.Hide(popupctl.AlbumSorting)
		av := m.Navigation.AlbumView()
		settings := av.Settings()
		settings.SortCriteria = act.Criteria
		settings.PresetName = "" // Clear preset when manually changed
		av.SetSettings(settings)
		_ = av.Refresh()
		m.SaveNavigationState()
		return m, nil
	case albumview.SortingCanceled:
		m.Popups.Hide(popupctl.AlbumSorting)
		return m, nil
	}
	return m, nil
}

// handleAlbumPresetsAction handles actions from the album presets popup.
func (m Model) handleAlbumPresetsAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case albumview.PresetLoaded:
		m.Popups.Hide(popupctl.AlbumPresets)
		av := m.Navigation.AlbumView()
		av.SetSettings(act.Settings)
		_ = av.Refresh()
		m.SaveNavigationState()
		return m, nil
	case albumview.PresetSaved:
		_, err := m.StateMgr.SaveAlbumPreset(act.Name, act.Settings)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPresetSave, err))
			return m, nil
		}
		m.Popups.Hide(popupctl.AlbumPresets)
		return m, nil
	case albumview.PresetDeleted:
		err := m.StateMgr.DeleteAlbumPreset(act.ID)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPresetDelete, err))
			return m, nil
		}
		// Refresh presets list in popup
		presets, err := m.StateMgr.ListAlbumPresets()
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPresetLoad, err))
			return m, nil
		}
		av := m.Navigation.AlbumView()
		cmd := m.Popups.ShowAlbumPresets(presets, av.Settings().Settings)
		return m, cmd
	case albumview.PresetsClosed:
		m.Popups.Hide(popupctl.AlbumPresets)
		return m, nil
	}
	return m, nil
}

// handleNavigatorAction handles actions from the navigator.
func (m Model) handleNavigatorAction(a action.Action) (tea.Model, tea.Cmd) {
	if _, ok := a.(navigator.NavigationChanged); ok {
		m.SaveNavigationState()
		return m, nil
	}
	return m, nil
}

// handleSearchAction handles actions from the search popup.
func (m Model) handleSearchAction(a action.Action) (tea.Model, tea.Cmd) {
	if act, ok := a.(search.Result); ok {
		return m.handleSearchResultAction(act)
	}
	return m, nil
}

// handleSearchResultAction handles the search result action.
func (m Model) handleSearchResultAction(act search.Result) (tea.Model, tea.Cmd) {
	// Handle add-to-playlist mode
	if m.Input.IsAddToPlaylistSearch() {
		return m.handleAddToPlaylistResult(act)
	}

	// Process the result before clearing search state
	if !act.Canceled && act.Item != nil {
		// Auto-focus navigator when selecting a search result
		m.SetFocus(navctl.FocusNavigator)
		m.navigateToSearchResult(act.Item)
	}

	m.Input.EndSearch()
	return m, nil
}

// handleAddToPlaylistResult handles search results in add-to-playlist mode.
func (m Model) handleAddToPlaylistResult(act search.Result) (tea.Model, tea.Cmd) {
	trackIDs := m.Input.AddToPlaylistTracks()
	m.Input.EndSearch()

	if act.Canceled || act.Item == nil {
		return m, nil
	}

	item, ok := act.Item.(playlists.SearchItem)
	if !ok {
		return m, nil
	}

	if err := m.Playlists.AddTracks(item.ID, trackIDs); err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpPlaylistAddTrack, err))
		return m, nil
	}

	// Update last used timestamp
	_ = m.Playlists.UpdateLastUsed(item.ID)

	// Refresh playlist navigator so new tracks are visible
	m.refreshPlaylistNavigator(true)

	return m, nil
}

// navigateToSearchResult navigates to the selected search result item.
func (m *Model) navigateToSearchResult(item search.Item) {
	switch item := item.(type) {
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

// handleTextInputAction handles actions from the text input popup.
func (m Model) handleTextInputAction(a action.Action) (tea.Model, tea.Cmd) {
	if act, ok := a.(textinput.Result); ok {
		return m.handleTextInputResultAction(act)
	}
	return m, nil
}

// handleTextInputResultAction handles the text input result action.
func (m Model) handleTextInputResultAction(act textinput.Result) (tea.Model, tea.Cmd) {
	m.Popups.Hide(popupctl.TextInput)

	if act.Canceled {
		return m, nil
	}

	ctx, ok := act.Context.(PlaylistInputContext)
	if !ok {
		return m, nil
	}

	return m.processPlaylistInput(ctx, act.Text)
}

// handleConfirmAction handles actions from the confirmation popup.
func (m Model) handleConfirmAction(a action.Action) (tea.Model, tea.Cmd) {
	if act, ok := a.(confirm.Result); ok {
		return m.handleConfirmResultAction(act)
	}
	return m, nil
}

// handleConfirmResultAction handles the confirmation result action.
func (m Model) handleConfirmResultAction(act confirm.Result) (tea.Model, tea.Cmd) {
	m.Popups.Hide(popupctl.Confirm)

	if !act.Confirmed {
		return m, nil
	}

	return m.processConfirmResult(act.Context, act.SelectedOption)
}

// handleLibrarySourcesAction handles actions from the library sources popup.
func (m Model) handleLibrarySourcesAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case librarysources.SourceAdded:
		return m.handleLibrarySourceAddedAction(act)
	case librarysources.SourceRemoved:
		return m.handleLibrarySourceRemovedAction(act)
	case librarysources.RequestTrackCount:
		count, err := m.Library.TrackCountBySource(act.Path)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpSourceLoad, err))
			return m, nil
		}
		m.Popups.LibrarySources().EnterConfirmMode(count)
		return m, nil
	case librarysources.Close:
		m.Popups.Hide(popupctl.LibrarySources)
		// Continue listening for scan progress if a scan is running
		return m, m.waitForLibraryScan()
	}
	return m, nil
}

// handleLibrarySourceAddedAction handles adding a library source.
func (m Model) handleLibrarySourceAddedAction(act librarysources.SourceAdded) (tea.Model, tea.Cmd) {
	// Check if source already exists
	exists, err := m.Library.SourceExists(act.Path)
	if err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpSourceAdd, err))
		return m, nil
	}
	if exists {
		m.Popups.ShowError("Source already exists")
		return m, nil
	}

	// Add source to library
	if err := m.Library.AddSource(act.Path); err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpSourceAdd, err))
		return m, nil
	}

	// Update popup with new sources
	sources, _ := m.Library.Sources()
	m.Popups.LibrarySources().SetSources(sources)
	m.HasLibrarySources = len(sources) > 0

	// Start scanning this source
	cmd := m.startLibraryScanForSource(act.Path)
	return m, cmd
}

// handleLibrarySourceRemovedAction handles removing a library source.
func (m Model) handleLibrarySourceRemovedAction(act librarysources.SourceRemoved) (tea.Model, tea.Cmd) {
	// Remove tracks from this source
	if err := m.Library.RemoveSource(act.Path); err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpSourceRemove, err))
		return m, nil
	}

	// Update popup with new sources
	sources, _ := m.Library.Sources()
	m.Popups.LibrarySources().SetSources(sources)
	m.HasLibrarySources = len(sources) > 0

	// Refresh library navigator and album view
	m.refreshLibraryNavigator(true)
	_ = m.Navigation.AlbumView().Refresh()
	return m, nil
}

// handleHelpBindingsAction handles actions from the help bindings popup.
func (m Model) handleHelpBindingsAction(a action.Action) (tea.Model, tea.Cmd) {
	if _, ok := a.(helpbindings.Close); ok {
		m.Popups.Hide(popupctl.Help)
		return m, nil
	}
	return m, nil
}

// handleDownloadsViewAction handles actions from the downloads view.
func (m Model) handleDownloadsViewAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case dlview.DeleteDownload:
		var client *slskd.Client
		if m.HasSlskdConfig {
			client = slskd.NewClient(m.Slskd.URL, m.Slskd.APIKey)
		}
		return m, DeleteDownloadCmd(DeleteDownloadParams{
			Manager:       m.Downloads,
			ID:            act.ID,
			SlskdClient:   client,
			CompletedPath: m.Slskd.CompletedPath,
		})

	case dlview.ClearCompleted:
		return m, ClearCompletedDownloadsCmd(m.Downloads)

	case dlview.RefreshRequest:
		if m.HasSlskdConfig {
			client := slskd.NewClient(m.Slskd.URL, m.Slskd.APIKey)
			return m, RefreshDownloadsCmd(m.Downloads, client, m.Slskd.CompletedPath)
		}
		return m, nil

	case dlview.OpenImport:
		if act.Download != nil && m.HasSlskdConfig {
			sources, err := m.Library.Sources()
			if err != nil {
				m.Popups.ShowError(errmsg.Format(errmsg.OpSourceLoad, err))
				return m, nil
			}
			mbClient := musicbrainz.NewClient()
			cmd := m.Popups.ShowImport(act.Download, m.Slskd.CompletedPath, sources, mbClient, m.RenameConfig)
			return m, cmd
		}
		return m, nil

	case dlview.ImportNotReady:
		m.Popups.ShowError("Cannot import: " + act.Reason)
		return m, nil
	}
	return m, nil
}

// handleDownloadPopupAction handles actions from the download popup.
func (m Model) handleDownloadPopupAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case download.Close:
		m.Popups.Hide(popupctl.Download)
		return m, nil

	case download.QueuedData:
		// Close the download popup
		m.Popups.Hide(popupctl.Download)

		// Switch to downloads view and focus it
		m.Navigation.SetViewMode(navctl.ViewDownloads)
		m.SetFocus(navctl.FocusNavigator)
		m.SaveNavigationState()

		// Persist the download to database and refresh downloads view
		createCmd := CreateDownloadCmd(m.Downloads, DownloadCreatedMsg{
			MBReleaseGroupID: act.MBReleaseGroupID,
			MBReleaseID:      act.MBReleaseID,
			MBArtistName:     act.MBArtistName,
			MBAlbumTitle:     act.MBAlbumTitle,
			MBReleaseYear:    act.MBReleaseYear,
			SlskdUsername:    act.SlskdUsername,
			SlskdDirectory:   act.SlskdDirectory,
			Files:            convertDownloadFilesFromAction(act.Files),
			MBReleaseGroup:   act.MBReleaseGroup,
			MBReleaseDetails: act.MBReleaseDetails,
		})
		refreshCmd := m.loadAndRefreshDownloads()
		return m, tea.Batch(createCmd, refreshCmd)
	}
	return m, nil
}

// handleImportPopupAction handles actions from the import popup.
func (m Model) handleImportPopupAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case importpopup.Close:
		m.Popups.Hide(popupctl.Import)
		return m, nil

	case importpopup.ImportComplete:
		// Import completed - add imported tracks to library
		var cmds []tea.Cmd

		// Add only the imported tracks to the library (no full refresh)
		if m.HasLibrarySources && len(act.ImportedPaths) > 0 {
			cmds = append(cmds, AddTracksToLibraryCmd(AddTracksToLibraryParams{
				Library:      m.Library,
				Paths:        act.ImportedPaths,
				DownloadID:   act.DownloadID,
				ArtistName:   act.ArtistName,
				AlbumName:    act.AlbumName,
				AllSucceeded: act.AllSucceeded,
			}))
		} else if act.AllSucceeded {
			// No tracks to add but import succeeded - send completion directly
			cmds = append(cmds, func() tea.Msg {
				return importpopup.LibraryRefreshedMsg{
					DownloadID:   act.DownloadID,
					ArtistName:   act.ArtistName,
					AlbumName:    act.AlbumName,
					AllSucceeded: act.AllSucceeded,
				}
			})
		}

		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// convertDownloadFilesFromAction converts download file info from action type to app type.
func convertDownloadFilesFromAction(files []download.FileInfo) []DownloadFile {
	result := make([]DownloadFile, len(files))
	for i, f := range files {
		result[i] = DownloadFile{
			Filename: f.Filename,
			Size:     f.Size,
		}
	}
	return result
}

// processPlaylistInput processes text input for playlist operations.
func (m Model) processPlaylistInput(ctx PlaylistInputContext, text string) (tea.Model, tea.Cmd) {
	if text == "" {
		return m, nil
	}

	var navigateToID string

	switch ctx.Mode {
	case InputNone:
		// No action
	case InputNewPlaylist:
		id, err := m.Playlists.Create(ctx.FolderID, text)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPlaylistCreate, err))
			return m, nil
		}
		navigateToID = "playlists:playlist:" + strconv.FormatInt(id, 10)
	case InputNewFolder:
		id, err := m.Playlists.CreateFolder(ctx.FolderID, text)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpFolderCreate, err))
			return m, nil
		}
		navigateToID = "playlists:folder:" + strconv.FormatInt(id, 10)
	case InputRename:
		var err error
		if ctx.IsFolder {
			err = m.Playlists.RenameFolder(ctx.ItemID, text)
		} else {
			err = m.Playlists.Rename(ctx.ItemID, text)
		}
		if err != nil {
			op := errmsg.OpPlaylistRename
			if ctx.IsFolder {
				op = errmsg.OpFolderRename
			}
			m.Popups.ShowError(errmsg.Format(op, err))
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

// processConfirmResult processes the confirmation dialog result.
func (m Model) processConfirmResult(context any, selectedOption int) (tea.Model, tea.Cmd) {
	// Handle library delete context
	if ctx, ok := context.(LibraryDeleteContext); ok {
		return m.handleLibraryDeleteConfirm(ctx, selectedOption)
	}

	// Handle file browser delete context
	if ctx, ok := context.(FileDeleteContext); ok {
		return m.handleFileDeleteConfirm(ctx)
	}

	// Handle playlist delete context
	ctx, ok := context.(DeleteConfirmContext)
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
		op := errmsg.OpPlaylistDelete
		if ctx.IsFolder {
			op = errmsg.OpFolderDelete
		}
		m.Popups.ShowError(errmsg.Format(op, err))
		return m, nil
	}

	// Refresh playlist navigator
	m.refreshPlaylistNavigator(true)
	return m, nil
}

// handleLibraryDeleteConfirm handles library track deletion confirmation.
func (m Model) handleLibraryDeleteConfirm(ctx LibraryDeleteContext, option int) (tea.Model, tea.Cmd) {
	switch option {
	case 0: // Remove from library only
		if err := m.Library.DeleteTrack(ctx.TrackID); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpLibraryDelete, err))
			return m, nil
		}
	case 1: // Delete from disk
		if err := os.Remove(ctx.TrackPath); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpFileDelete, err))
			return m, nil
		}
		if err := m.Library.DeleteTrack(ctx.TrackID); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpLibraryDelete, err))
			return m, nil
		}
	default: // Cancel or unknown
		return m, nil
	}

	// Refresh library navigator
	m.Navigation.LibraryNav().Refresh()
	return m, nil
}

// handleFileDeleteConfirm handles file browser file/directory deletion confirmation.
func (m Model) handleFileDeleteConfirm(ctx FileDeleteContext) (tea.Model, tea.Cmd) {
	var err error
	if ctx.IsDir {
		err = os.RemoveAll(ctx.Path)
	} else {
		err = os.Remove(ctx.Path)
	}
	if err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpFileDelete, err))
		return m, nil
	}

	m.Navigation.FileNav().Refresh()
	return m, nil
}

// startLibraryScanForSource starts a library scan for a specific source path.
func (m *Model) startLibraryScanForSource(path string) tea.Cmd {
	if m.LibraryScanCh != nil {
		return nil
	}

	ch := make(chan library.ScanProgress)
	m.LibraryScanCh = ch
	go func() {
		_ = m.Library.RefreshSource(path, ch)
	}()

	return m.waitForLibraryScan()
}

// handleRetagPopupAction handles actions from the retag popup.
func (m Model) handleRetagPopupAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case retag.Close:
		m.Popups.Hide(popupctl.Retag)
		return m, nil
	case retag.RequestStart:
		// Check if any track to be retagged is currently playing
		if info := m.PlaybackService.Player().TrackInfo(); info != nil {
			if slices.Contains(act.TrackPaths, info.Path) {
				// Stop playback to release file lock
				_ = m.PlaybackService.Stop()
			}
		}
		// Send approval to start retagging
		return m, func() tea.Msg { return retag.StartApprovedMsg{} }
	case retag.Complete:
		m.Popups.Hide(popupctl.Retag)

		// Refresh library views to show updated tags
		m.refreshLibraryNavigator(true)
		if m.Navigation.IsAlbumViewActive() {
			_ = m.Navigation.AlbumView().Refresh()
		}

		// Select the retagged album
		if act.AlbumArtist != "" && act.AlbumName != "" {
			m.selectAlbumInCurrentMode(act.AlbumArtist, act.AlbumName)
		}

		return m, nil
	}
	return m, nil
}

// handleExportPopupAction handles actions from the export popup.
func (m Model) handleExportPopupAction(a action.Action) (tea.Model, tea.Cmd) {
	switch act := a.(type) {
	case exportui.Close:
		m.Popups.Hide(popupctl.Export)
		return m, nil

	case exportui.DeviceNotConnected:
		name := act.TargetName
		if name == "" {
			name = "target"
		}
		m.Popups.ShowError("Device for " + name + " is not connected")
		return m, nil

	case exportui.DeleteTarget:
		// Delete the target and refresh
		if err := m.ExportRepo.Delete(act.ID); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpTargetDelete, err))
			return m, nil
		}
		// Refresh targets list
		return m, exportui.LoadTargetsCmd(m.ExportRepo)

	case exportui.RenameTarget:
		// Get the target, update its name, and refresh
		target, err := m.ExportRepo.Get(act.ID)
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpTargetRename, err))
			return m, nil
		}
		target.Name = act.NewName
		if err := m.ExportRepo.Update(target); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpTargetRename, err))
			return m, nil
		}
		// Refresh targets list
		return m, exportui.LoadTargetsCmd(m.ExportRepo)

	case exportui.StartExport:
		// Create and start the export job
		job := export.NewJob(act.Target, act.Tracks)
		jobID := job.JobBar().ID
		m.ExportJobs[jobID] = job
		m.Popups.Hide(popupctl.Export)

		params := export.Params{
			Job:         job,
			Exporter:    export.NewExporter(),
			ConvertFLAC: act.ConvertFLAC,
			BasePath:    act.MountPath,
		}
		m.ExportParams[jobID] = params
		m.ResizeComponents() // Show job bar
		return m, export.BatchCmd(params)
	}

	return m, nil
}

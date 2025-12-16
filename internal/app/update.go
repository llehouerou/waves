// internal/app/update.go
package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/download"
	importpopup "github.com/llehouerou/waves/internal/importer/popup"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui/confirm"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/librarysources"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
	"github.com/llehouerou/waves/internal/ui/textinput"
)

// Update handles messages and returns updated model and commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Standard tea messages first
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	// Category-based routing for local messages
	case LoadingMessage:
		return m.handleLoadingMsg(msg)
	case PlaybackMessage:
		return m.handlePlaybackMsg(msg)
	case NavigationMessage:
		return m.handleNavigationMsg(msg)
	case InputMessage:
		return m.handleInputMsg(msg)
	case LibraryScanMessage:
		return m.handleLibraryScanMsg(msg)
	case DownloadMessage:
		return m.handleDownloadMsgCategory(msg)

	// External messages from ui packages (cannot implement our interfaces)
	case queuepanel.JumpToTrackMsg:
		cmd := m.PlayTrackAtIndex(msg.Index)
		return m, cmd

	case queuepanel.QueueChangedMsg:
		m.SaveQueueState()
		return m, nil

	case queuepanel.ToggleFavoriteMsg:
		m.handleToggleFavorite(msg.TrackIDs)
		return m, nil

	case navigator.NavigationChangedMsg:
		m.SaveNavigationState()
		return m, nil

	case search.ResultMsg:
		return m.handleSearchResult(msg)

	case textinput.ResultMsg:
		return m.handleTextInputResult(msg)

	case confirm.ResultMsg:
		return m.handleConfirmResult(msg)

	case librarysources.SourceAddedMsg:
		return m.handleLibrarySourceAdded(msg)

	case librarysources.SourceRemovedMsg:
		return m.handleLibrarySourceRemoved(msg)

	case librarysources.CloseMsg:
		m.Popups.Hide(PopupLibrarySources)
		// Continue listening for scan progress if a scan is running
		return m, m.waitForLibraryScan()

	case librarysources.RequestTrackCountMsg:
		count, err := m.Library.TrackCountBySource(msg.Path)
		if err != nil {
			m.Popups.ShowError(err.Error())
			return m, nil
		}
		m.Popups.LibrarySources().EnterConfirmMode(count)
		return m, nil

	case helpbindings.CloseMsg:
		m.Popups.Hide(PopupHelp)
		return m, nil

	// Downloads view messages
	case dlview.DeleteDownloadMsg:
		var client *slskd.Client
		if m.HasSlskdConfig {
			client = slskd.NewClient(m.Slskd.URL, m.Slskd.APIKey)
		}
		return m, DeleteDownloadCmd(DeleteDownloadParams{
			Manager:       m.Downloads,
			ID:            msg.ID,
			SlskdClient:   client,
			CompletedPath: m.Slskd.CompletedPath,
		})

	case dlview.ClearCompletedMsg:
		return m, ClearCompletedDownloadsCmd(m.Downloads)

	case dlview.RefreshRequestMsg:
		if m.HasSlskdConfig {
			client := slskd.NewClient(m.Slskd.URL, m.Slskd.APIKey)
			return m, RefreshDownloadsCmd(m.Downloads, client, m.Slskd.CompletedPath)
		}
		return m, nil

	case dlview.OpenImportMsg:
		// Open import popup for completed download
		if msg.Download != nil && m.HasSlskdConfig {
			sources, err := m.Library.Sources()
			if err != nil {
				m.Popups.ShowError("Cannot load library sources: " + err.Error())
				return m, nil
			}
			mbClient := musicbrainz.NewClient()
			cmd := m.Popups.ShowImport(msg.Download, m.Slskd.CompletedPath, sources, mbClient)
			return m, cmd
		}
		return m, nil

	// Download popup messages
	case download.CloseMsg:
		m.Popups.Hide(PopupDownload)
		return m, nil

	case download.QueuedDataMsg:
		// Close the download popup
		m.Popups.Hide(PopupDownload)

		// Switch to downloads view and focus it
		m.Navigation.SetViewMode(ViewDownloads)
		m.SetFocus(FocusNavigator)
		m.SaveNavigationState()

		// Persist the download to database and refresh downloads view
		createCmd := CreateDownloadCmd(m.Downloads, DownloadCreatedMsg{
			MBReleaseGroupID: msg.MBReleaseGroupID,
			MBReleaseID:      msg.MBReleaseID,
			MBArtistName:     msg.MBArtistName,
			MBAlbumTitle:     msg.MBAlbumTitle,
			MBReleaseYear:    msg.MBReleaseYear,
			SlskdUsername:    msg.SlskdUsername,
			SlskdDirectory:   msg.SlskdDirectory,
			Files:            convertDownloadFiles(msg.Files),
			MBReleaseGroup:   msg.MBReleaseGroup,
			MBReleaseDetails: msg.MBReleaseDetails,
		})
		refreshCmd := m.loadAndRefreshDownloads()
		return m, tea.Batch(createCmd, refreshCmd)

	case download.ArtistSearchResultMsg,
		download.ReleaseGroupResultMsg,
		download.ReleaseResultMsg,
		download.ReleaseDetailsResultMsg,
		download.SlskdSearchStartedMsg,
		download.SlskdSearchPollMsg,
		download.SlskdPollContinueMsg,
		download.SlskdSearchResultMsg,
		download.SlskdDownloadQueuedMsg:
		return m.handleDownloadMsg(msg)

	// Import popup messages
	case importpopup.CloseMsg:
		m.Popups.Hide(PopupImport)
		return m, nil

	case importpopup.ImportCompleteMsg:
		// Import completed - add imported tracks to library
		var cmds []tea.Cmd

		// Add only the imported tracks to the library (no full refresh)
		if m.HasLibrarySources && len(msg.ImportedPaths) > 0 {
			cmds = append(cmds, AddTracksToLibraryCmd(AddTracksToLibraryParams{
				Library:      m.Library,
				Paths:        msg.ImportedPaths,
				DownloadID:   msg.DownloadID,
				ArtistName:   msg.ArtistName,
				AlbumName:    msg.AlbumName,
				AllSucceeded: msg.AllSucceeded,
			}))
		} else if msg.AllSucceeded {
			// No tracks to add but import succeeded - send completion directly
			cmds = append(cmds, func() tea.Msg {
				return importpopup.LibraryRefreshedMsg{
					DownloadID:   msg.DownloadID,
					ArtistName:   msg.ArtistName,
					AlbumName:    msg.AlbumName,
					AllSucceeded: msg.AllSucceeded,
				}
			})
		}

		return m, tea.Batch(cmds...)

	case importpopup.LibraryRefreshedMsg:
		// Library refreshed after import
		// First, let popup handle state change
		_, cmd := m.handleImportMsg(msg)
		var cmds []tea.Cmd
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		// If import fully succeeded, clean up and navigate
		if msg.AllSucceeded && msg.Err == nil {
			// Delete the download from database
			if msg.DownloadID > 0 {
				_ = m.Downloads.Delete(msg.DownloadID)
				// Refresh downloads view
				downloads, _ := m.Downloads.List()
				m.DownloadsView.SetDownloads(downloads)
			}

			// Close the import popup
			m.Popups.Hide(PopupImport)

			// Refresh library navigator to include new tracks
			m.refreshLibraryNavigator(false)

			// Switch to library view and navigate to the album
			m.Navigation.SetViewMode(ViewLibrary)
			m.SetFocus(FocusNavigator)
			if msg.ArtistName != "" && msg.AlbumName != "" {
				albumID := "library:album:" + msg.ArtistName + ":" + msg.AlbumName
				m.Navigation.LibraryNav().NavigateTo(albumID)
			}
			m.SaveNavigationState()
		}

		return m, tea.Batch(cmds...)

	case importpopup.TagsReadMsg,
		importpopup.FileImportedMsg,
		importpopup.MBReleaseRefreshedMsg:
		return m.handleImportMsg(msg)

	case StderrMsg:
		// Display stderr output from C libraries as errors
		// Check for audio server disconnection (ALSA errors indicate this)
		if isAudioDisconnectError(msg.Line) {
			m.Popups.ShowError("Audio server disconnected. Restart app to restore playback.")
			m.Playback.Stop()
			m.ResizeComponents()
		} else {
			m.Popups.ShowError("Audio: " + msg.Line)
		}
		return m, WatchStderr()
	}

	return m, nil
}

func (m Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Route mouse events to focused component
	if m.Navigation.IsQueueFocused() && m.Layout.IsQueueVisible() {
		panel, cmd := m.Layout.QueuePanel().Update(msg)
		m.Layout.SetQueuePanel(panel)
		return m, cmd
	}

	if m.Navigation.IsNavigatorFocused() {
		return m.handleNavigatorMouse(msg)
	}

	return m, nil
}

func (m Model) handleNavigatorMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle middle click: navigate into container OR play track
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonMiddle {
		return m.handleNavigatorMiddleClick(msg)
	}

	// Route other mouse events to navigator
	return m.routeMouseToNavigator(msg)
}

func (m Model) handleNavigatorMiddleClick(_ tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Middle click: replace queue and play
	// Container selected: replace queue with contents, play first track
	// Track selected: replace queue with parent container, play selected track
	if m.isSelectedItemContainer() {
		if cmd := m.HandleQueueAction(QueueReplace); cmd != nil {
			return m, cmd
		}
	} else {
		if cmd := m.HandleContainerAndPlay(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) isSelectedItemContainer() bool {
	if node := m.selectedNode(); node != nil {
		return node.IsContainer()
	}
	return false
}

func (m Model) routeMouseToNavigator(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	cmd := m.Navigation.UpdateActiveNavigator(msg)
	return m, cmd
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.Layout.SetSize(msg.Width, msg.Height)
	m.Input.SetSize(msg)
	m.ResizeComponents()
	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle popups first - they intercept all keys when active
	if handled, cmd := m.Popups.HandleKey(msg); handled {
		return m, cmd
	}

	// Handle search mode (regular search or add-to-playlist)
	if m.Input.IsSearchActive() {
		cmd := m.Input.UpdateSearch(msg)
		return m, cmd
	}

	key := msg.String()

	// Handle key sequences starting with 'g'
	if m.Input.IsKeySequence("g") {
		return m.handleGSequence(key)
	}

	// Handle queue panel input when focused
	if m.Navigation.IsQueueFocused() && m.Layout.IsQueueVisible() {
		panel, cmd := m.Layout.QueuePanel().Update(msg)
		m.Layout.SetQueuePanel(panel)
		if cmd != nil {
			return m, cmd
		}

		if key == "escape" {
			m.SetFocus(FocusNavigator)
			return m, nil
		}
	}

	return m.handleGlobalKeys(key, msg)
}

func (m Model) handleGlobalKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	handlers := []func(key string) (bool, tea.Cmd){
		m.handleQuitKeys,
		m.handleViewKeys,
		m.handleFocusKeys,
		m.handleHelpKey,
		m.handleGPrefixKey,
		m.handleQueueHistoryKeys,
		m.handlePlaybackKeys,
		m.handleNavigatorActionKeys,
		m.handlePlaylistKeys,
		m.handleLibraryKeys,
		m.handleFileBrowserKeys,
	}

	for _, h := range handlers {
		if handled, cmd := h(key); handled {
			return m, cmd
		}
	}

	// Delegate unhandled keys to downloads view when it's active
	if m.Navigation.ViewMode() == ViewDownloads && m.Navigation.IsNavigatorFocused() {
		var cmd tea.Cmd
		m.DownloadsView, cmd = m.DownloadsView.Update(msg)
		return m, cmd
	}

	// Delegate unhandled keys to the active navigator
	if m.Navigation.IsNavigatorFocused() {
		cmd := m.Navigation.UpdateActiveNavigator(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) waitForScan() tea.Cmd {
	return waitForChannel(m.Input.ScanChan(), func(result navigator.ScanResult, ok bool) tea.Msg {
		if !ok {
			return ScanResultMsg{Done: true}
		}
		return ScanResultMsg(result)
	})
}

// handleDownloadMsg routes messages to the download popup model.
func (m Model) handleDownloadMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	dl := m.Popups.Download()
	if dl == nil {
		return m, nil
	}
	_, cmd := dl.Update(msg)
	return m, cmd
}

// handleImportMsg routes messages to the import popup model.
func (m Model) handleImportMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	imp := m.Popups.Import()
	if imp == nil {
		return m, nil
	}
	_, cmd := imp.Update(msg)
	return m, cmd
}

// isAudioDisconnectError checks if a stderr message indicates the audio server disconnected.
func isAudioDisconnectError(line string) bool {
	// Common ALSA/PipeWire error patterns when the audio server restarts
	disconnectPatterns := []string{
		"ALSA lib",
		"snd_pcm",
		"pulseaudio",
		"pipewire",
		"Broken pipe",
		"Connection refused",
		"Device or resource busy",
	}
	for _, pattern := range disconnectPatterns {
		if strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// convertDownloadFiles converts download file info from download package to app package.
func convertDownloadFiles(files []download.FileInfo) []DownloadFile {
	result := make([]DownloadFile, len(files))
	for i, f := range files {
		result[i] = DownloadFile{
			Filename: f.Filename,
			Size:     f.Size,
		}
	}
	return result
}

// handleDownloadMsgCategory handles download-related messages.
func (m Model) handleDownloadMsgCategory(msg DownloadMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case DownloadCreatedMsg:
		// Persist the new download and refresh
		return m, CreateDownloadCmd(m.Downloads, msg)

	case DownloadsRefreshMsg:
		// Periodic refresh trigger
		if !m.HasSlskdConfig {
			return m, nil
		}
		client := slskd.NewClient(m.Slskd.URL, m.Slskd.APIKey)
		return m, tea.Batch(
			RefreshDownloadsCmd(m.Downloads, client, m.Slskd.CompletedPath),
			DownloadsRefreshTickCmd(),
		)

	case DownloadsRefreshResultMsg:
		// Refresh completed - update the view
		if msg.Err != nil {
			// Log error but don't show popup (too noisy for periodic refresh)
			return m, nil
		}
		// Reload downloads into view
		downloads, err := m.Downloads.List()
		if err != nil {
			return m, nil
		}
		m.DownloadsView.SetDownloads(downloads)
		return m, nil

	case DownloadDeletedMsg:
		if msg.Err != nil {
			m.Popups.ShowError("Failed to delete download: " + msg.Err.Error())
			return m, nil
		}
		// Refresh downloads list
		downloads, err := m.Downloads.List()
		if err != nil {
			return m, nil
		}
		m.DownloadsView.SetDownloads(downloads)
		return m, nil

	case CompletedDownloadsClearedMsg:
		if msg.Err != nil {
			m.Popups.ShowError("Failed to clear downloads: " + msg.Err.Error())
			return m, nil
		}
		// Refresh downloads list
		downloads, err := m.Downloads.List()
		if err != nil {
			return m, nil
		}
		m.DownloadsView.SetDownloads(downloads)
		return m, nil
	}

	return m, nil
}

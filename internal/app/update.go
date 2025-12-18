// internal/app/update.go
package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/errmsg"
	importpopup "github.com/llehouerou/waves/internal/importer/popup"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/retag"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui/action"
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

	// UI component actions (wrapped in action.Msg)
	case action.Msg:
		return m.handleUIAction(msg)

	// Pass-through messages for download popup internal workflows
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

	// Pass-through messages for import popup internal workflows
	case importpopup.TagsReadMsg,
		importpopup.FileImportedMsg,
		importpopup.MBReleaseRefreshedMsg:
		return m.handleImportMsg(msg)

	// Pass-through messages for retag popup internal workflows
	case retag.TagsReadMsg,
		retag.ReleaseGroupSearchResultMsg,
		retag.ReleasesFetchedMsg,
		retag.ReleaseDetailsFetchedMsg,
		retag.FileRetaggedMsg,
		retag.LibraryUpdatedMsg:
		return m.handleRetagMsg(msg)

	// Library refresh message - handled by both app and popup
	case importpopup.LibraryRefreshedMsg:
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

			// Switch to library album view and select the imported album
			m.Navigation.SetViewMode(ViewLibrary)
			m.Navigation.SetLibrarySubMode(LibraryModeAlbum)
			m.SetFocus(FocusNavigator)
			if msg.ArtistName != "" && msg.AlbumName != "" {
				// Refresh album view and select the imported album
				if err := m.Navigation.AlbumView().Refresh(); err == nil {
					albumID := msg.ArtistName + ":" + msg.AlbumName
					m.Navigation.AlbumView().SelectByID(albumID)
				}
			}
			m.SaveNavigationState()
		}

		return m, tea.Batch(cmds...)

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
	// Album view handles its own mouse events (including middle click)
	if m.Navigation.IsAlbumViewActive() {
		return m.routeMouseToNavigator(msg)
	}

	// Handle middle click for Miller columns: navigate into container OR play track
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

	// Handle key sequences starting with 'f'
	if m.Input.IsKeySequence("f") {
		return m.handleFSequence(key)
	}

	// Handle key sequences starting with 'o'
	if m.Input.IsKeySequence("o") {
		return m.handleOSequence(key)
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
		m.handleFPrefixKey,
		m.handleOPrefixKey,
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

// handleRetagMsg routes messages to the retag popup model.
func (m Model) handleRetagMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	rt := m.Popups.Retag()
	if rt == nil {
		return m, nil
	}
	_, cmd := rt.Update(msg)
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
			m.Popups.ShowError(errmsg.Format(errmsg.OpDownloadDelete, msg.Err))
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
			m.Popups.ShowError(errmsg.Format(errmsg.OpDownloadClear, msg.Err))
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

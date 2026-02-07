// internal/app/update.go
package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/export"
	importpopup "github.com/llehouerou/waves/internal/importer/popup"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/musicbrainz/workflow"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/retag"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui/action"
	exportui "github.com/llehouerou/waves/internal/ui/export"
	"github.com/llehouerou/waves/internal/ui/lastfmauth"
	lyricsui "github.com/llehouerou/waves/internal/ui/lyrics"
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
	case RadioMessage:
		return m.handleRadioMsgCategory(msg)

	// UI component actions (wrapped in action.Msg)
	case action.Msg:
		return m.handleUIAction(msg)

	// Pass-through messages for download popup internal workflows
	case download.SlskdSearchStartedMsg,
		download.SlskdSearchPollMsg,
		download.SlskdPollContinueMsg,
		download.SlskdSearchResultMsg,
		download.SlskdDownloadQueuedMsg:
		return m.handleDownloadMsg(msg)

	// Pass-through messages for import popup internal workflows
	case importpopup.TagsReadMsg,
		importpopup.FileImportedMsg,
		importpopup.MBReleaseRefreshedMsg,
		importpopup.CoverArtFetchedMsg:
		return m.handleImportMsg(msg)

	// Pass-through messages for retag popup internal workflows
	case retag.TagsReadMsg,
		retag.FileRetaggedMsg,
		retag.LibraryUpdatedMsg,
		retag.StartApprovedMsg:
		return m.handleRetagMsg(msg)

	// Workflow messages - route to active popup (download or retag)
	case workflow.ArtistSearchResultMsg,
		workflow.SearchResultMsg,
		workflow.ReleasesResultMsg,
		workflow.ReleaseDetailsResultMsg,
		workflow.CoverArtResultMsg:
		return m.handleWorkflowMsg(msg)

	// Pass-through messages for export popup internal workflows
	case exportui.VolumesLoadedMsg,
		exportui.TargetsLoadedMsg,
		exportui.TargetCreatedMsg,
		exportui.DirectoriesLoadedMsg:
		return m.handleExportPopupMsg(msg)

	// Pass-through messages for lyrics popup internal workflows
	case lyricsui.FetchedMsg:
		return m.handleLyricsMsg(msg)

	// Export job messages
	case export.ProgressMsg:
		// Continue to next file
		params, ok := m.ExportParams[msg.JobID]
		if !ok {
			return m, nil
		}
		return m, export.ContinueExportCmd(params, msg.Current)

	case export.CompleteMsg:
		// Clean up the job and params
		delete(m.ExportJobs, msg.JobID)
		delete(m.ExportParams, msg.JobID)
		m.ResizeComponents()

		// Show result to user
		if msg.Failed > 0 {
			// Show first error as feedback
			errMsg := "Export failed"
			if len(msg.Errors) > 0 {
				errMsg = fmt.Sprintf("Export failed: %v", msg.Errors[0].Err)
			}
			if msg.Failed < msg.Total {
				errMsg = fmt.Sprintf("Export: %d/%d failed - %v",
					msg.Failed, msg.Total, msg.Errors[0].Err)
			}
			m.Popups.ShowError(errMsg)
		} else {
			// Success - show temporary notification with artist/album info
			var notifMsg string
			switch {
			case msg.Artist != "" && msg.Album != "":
				notifMsg = fmt.Sprintf("%s - %s → %s (%d tracks)",
					msg.Artist, msg.Album, msg.TargetName, msg.Total)
			case msg.Artist != "":
				notifMsg = fmt.Sprintf("%s → %s (%d tracks)",
					msg.Artist, msg.TargetName, msg.Total)
			default:
				notifMsg = fmt.Sprintf("Exported %d tracks → %s", msg.Total, msg.TargetName)
			}
			// Add notification with unique ID
			m.nextNotificationID++
			notifID := m.nextNotificationID
			m.Notifications = append(m.Notifications, Notification{
				ID:      notifID,
				Message: notifMsg,
			})
			m.ResizeComponents()
			return m, NotificationClearCmd(notifID)
		}
		return m, nil

	// Notification messages
	case NotificationClearMsg:
		// Remove the specific notification by ID
		for i, n := range m.Notifications {
			if n.ID == msg.ID {
				m.Notifications = append(m.Notifications[:i], m.Notifications[i+1:]...)
				break
			}
		}
		m.ResizeComponents()
		return m, nil

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
			m.Popups.Hide(popupctl.Import)

			// Refresh library navigator to include new tracks
			m.refreshLibraryNavigator(false)

			// Switch to library album view and select the imported album
			m.Navigation.SetViewMode(navctl.ViewLibrary)
			m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
			m.SetFocus(navctl.FocusNavigator)
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
		// Handle stderr output from C libraries
		if isAudioDisconnectError(msg.Line) {
			m.Popups.ShowError("Audio server disconnected. Restart app to restore playback.")
			_ = m.PlaybackService.Stop()
			m.ResizeComponents()
		} else if !isIgnorableStderr(msg.Line) {
			m.Popups.ShowError("Audio: " + msg.Line)
		}
		return m, WatchStderr()

	// Last.fm messages
	case lastfm.TokenResultMsg,
		lastfm.SessionResultMsg,
		lastfm.NowPlayingResultMsg,
		lastfm.ScrobbleResultMsg,
		lastfm.RetryPendingMsg,
		lastfm.RetryResultMsg:
		return m.handleLastfmMsg(msg)

	case lastfmauth.ActionMsg:
		return m.handleLastfmMsg(msg)
	}

	return m, nil
}

func (m Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle scroll on player bar for volume control (with debounce)
	if m.isMouseOnPlayerBar(msg) {
		switch msg.Button { //nolint:exhaustive // only handling scroll
		case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
			// Debounce: ignore scroll events within 100ms of the last one
			now := time.Now()
			if now.Sub(m.LastVolumeScrollTime) < 100*time.Millisecond {
				return m, nil
			}
			m.LastVolumeScrollTime = now

			delta := 0.10
			if msg.Button == tea.MouseButtonWheelDown {
				delta = -0.10
			}
			cmd := m.handleVolumeChange(delta)
			return m, cmd
		}
	}

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

// isMouseOnPlayerBar returns true if the mouse event is within the player bar area.
func (m Model) isMouseOnPlayerBar(msg tea.MouseMsg) bool {
	playerBarRow := m.PlayerBarRow()
	if playerBarRow == 0 {
		return false // Player is stopped, no player bar
	}
	// PlayerBarRow is 1-based, msg.Y is 0-based
	playerBarStartY := playerBarRow - 1
	playerBarHeight := m.playerBarHeight()
	return msg.Y >= playerBarStartY && msg.Y < playerBarStartY+playerBarHeight
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
	m.Layout.QueuePanel().SyncCursor()
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

	// Secret demo mode: ctrl+alt+d forces library view with album mode
	if key == "ctrl+alt+d" {
		m.Navigation.SetViewMode(navctl.ViewLibrary)
		m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
		m.SetFocus(navctl.FocusNavigator)
		return m, nil
	}

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
			m.SetFocus(navctl.FocusNavigator)
			return m, nil
		}
	}

	return m.handleGlobalKeys(key, msg)
}

func (m Model) handleGlobalKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	handled, cmd := handler.Chain(
		func() handler.Result { return m.handleQuitKeys(key) },
		func() handler.Result { return m.handleViewKeys(key) },
		func() handler.Result { return m.handleFocusKeys(key) },
		func() handler.Result { return m.handleHelpKey(key) },
		func() handler.Result { return m.handleFPrefixKey(key) },
		func() handler.Result { return m.handleOPrefixKey(key) },
		func() handler.Result { return m.handleQueueHistoryKeys(key) },
		func() handler.Result { return m.handlePlaybackKeys(key) },
		func() handler.Result { return m.handleNavigatorActionKeys(key) },
		func() handler.Result { return m.handlePlaylistKeys(key) },
		func() handler.Result { return m.handleLibraryKeys(key) },
		func() handler.Result { return m.handleFileBrowserKeys(key) },
		func() handler.Result { return m.handleExportKey(key) },
	)
	if handled {
		return m, cmd
	}

	// Delegate unhandled keys to downloads view when it's active
	if m.Navigation.ViewMode() == navctl.ViewDownloads && m.Navigation.IsNavigatorFocused() {
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

// handleWorkflowMsg routes workflow messages to the active popup (download or retag).
func (m Model) handleWorkflowMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Try download popup first
	if dl := m.Popups.Download(); dl != nil {
		_, cmd := dl.Update(msg)
		return m, cmd
	}
	// Try retag popup
	if rt := m.Popups.Retag(); rt != nil {
		_, cmd := rt.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleExportPopupMsg routes messages to the export popup model.
func (m Model) handleExportPopupMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	exp := m.Popups.Export()
	if exp == nil {
		return m, nil
	}
	_, cmd := exp.Update(msg)
	return m, cmd
}

// handleLyricsMsg routes messages to the lyrics popup model.
func (m Model) handleLyricsMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	lyr := m.Popups.Lyrics()
	if lyr == nil {
		return m, nil
	}
	_, cmd := lyr.Update(msg)
	return m, cmd
}

// isAudioDisconnectError checks if a stderr message indicates the audio server disconnected.
func isAudioDisconnectError(line string) bool {
	lower := strings.ToLower(line)
	// Specific error patterns that indicate actual audio failure
	// (not just ALSA informational messages printed on init)
	disconnectPatterns := []string{
		"broken pipe",
		"connection refused",
		"device or resource busy",
		"underrun occurred",
		"i/o error",
		"cannot recover",
	}
	for _, pattern := range disconnectPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// isIgnorableStderr checks if a stderr message is a harmless init warning that should be ignored.
func isIgnorableStderr(line string) bool {
	lower := strings.ToLower(line)
	// ALSA init warnings about missing plugins or config - not actual errors
	ignorePatterns := []string{
		"cannot be opened",           // plugin loading warnings
		"was not defined inside",     // plugin symbol warnings
		"unknown pcm",                // config warnings
		"cannot find card",           // missing card warnings
		"unable to open slave",       // slave device warnings
		"snd_pcm_open_conf",          // config open warnings
		"cannot open shared library", // library warnings
	}
	for _, pattern := range ignorePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// handleRadioMsgCategory handles radio-related messages.
func (m Model) handleRadioMsgCategory(msg RadioMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RadioFillResultMsg:
		m.handleRadioFillResult(msg)
		// If tracks were added and queue was empty, start playback
		if len(msg.Tracks) > 0 && m.PlaybackService.IsStopped() {
			cmd := m.StartQueuePlayback()
			return m, cmd
		}
		return m, nil
	case RadioToggledMsg:
		// Radio was toggled - nothing else to do, UI will update
		return m, nil
	}
	return m, nil
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
			m.Popups.ShowOpError(errmsg.OpDownloadDelete, msg.Err)
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
			m.Popups.ShowOpError(errmsg.OpDownloadClear, msg.Err)
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

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
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/download"
	exportui "github.com/llehouerou/waves/internal/ui/export"
	"github.com/llehouerou/waves/internal/ui/importpopup"
	"github.com/llehouerou/waves/internal/ui/lastfmauth"
	lyricsui "github.com/llehouerou/waves/internal/ui/lyrics"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/retag"
	"github.com/llehouerou/waves/internal/ui/similarartists"
)

// Update handles messages and returns updated model and commands.
//
// It also re-renders the panels block (see renderPanels) once per message,
// except for the playback tick which only moves the player bar (issue #54).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	if _, tick := msg.(TickMsg); tick {
		return next, cmd
	}
	nm, ok := next.(Model)
	if ok && nm.loadingState == loadingDone {
		nm.panels = nm.renderPanels()
		return nm, cmd
	}
	return next, cmd
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case download.SlskdDownloadQueuedMsg,
		download.ReleasesLoadedMsg:
		return m.handleDownloadMsg(msg)

	// Pass-through messages for import popup internal workflows
	case importpopup.PreparedMsg:
		return m.handleImportMsg(msg)

	// Pass-through messages for retag popup internal workflows
	case retag.TagsReadMsg:
		return m.handleRetagMsg(msg)

	// The retag the core runs
	case ServiceRetagMsg:
		cmd := m.handleRetagEvent(msg.Event)
		return m, tea.Batch(cmd, m.WatchServiceEvents())

	// Workflow messages - route to active popup (download or retag)
	case mbworkflow.ArtistSearchResultMsg,
		mbworkflow.SearchResultMsg,
		mbworkflow.ReleasesResultMsg,
		mbworkflow.ReleaseDetailsResultMsg,
		mbworkflow.CoverArtResultMsg:
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

	// Pass-through messages for similar artists popup internal workflows
	case similarartists.FetchResultMsg:
		return m.handleSimilarArtistsMsg(msg)

	// The exports the core runs
	case ServiceExportMsg:
		cmd := m.handleExportEvent(msg.Event)
		return m, tea.Batch(cmd, m.WatchServiceEvents())

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

	case StderrMsg:
		// Handle stderr output from C libraries
		if isAudioDisconnectError(msg.Line) {
			m.Popups.ShowError("Audio server disconnected. Restart app to restore playback.")
			_ = m.Core.Playback().Stop()
			m.ResizeComponents()
		} else if !isIgnorableStderr(msg.Line) {
			m.Popups.ShowError("Audio: " + msg.Line)
		}
		return m, WatchStderr()

	// Last.fm messages
	case LastfmLinkStartedMsg,
		LastfmLinkedMsg:
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
			m.Core.Playback().AdjustVolume(delta)
			return m, nil
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

// handleImportEvent takes an event of the import the core runs. The popup,
// when it is still open on that import, shows it; the end of the import is
// also the model's: the import changed the library and the downloads, whether
// the user is still looking at it or closed the popup and went on.
func (m *Model) handleImportEvent(e core.Event) tea.Cmd {
	imp := m.Popups.Import()
	if imp != nil {
		imp.Update(e) // it knows its own import from another's
	}
	finished, ok := e.(core.ImportFinished)
	if !ok {
		return nil
	}
	watched := imp != nil && imp.DownloadID() == finished.DownloadID
	imported := finished.Err == nil && core.AllDone(finished.Files)

	if imported {
		m.sendDownloadCompleteNotification(finished.Artist, finished.Album)
	}
	switch {
	case imported && watched:
		// The user was waiting for it: show them the album.
		m.Popups.Hide(popupctl.Import)
		m.refreshLibraryNavigator(false)
		m.Navigation.SetViewMode(navctl.ViewLibrary)
		m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
		m.SetFocus(navctl.FocusNavigator)
		if err := m.Navigation.AlbumView().Refresh(); err == nil && finished.Artist != "" && finished.Album != "" {
			m.Navigation.AlbumView().SelectByID(finished.Artist + ":" + finished.Album)
		}
		m.SaveNavigationState()
	case watched:
		// The popup tells what failed; the tracks that made it are in the library.
		m.refreshLibraryViews()
	default:
		// The user went on: stay where they are, and tell a failure there.
		m.refreshLibraryViews()
		if !imported {
			m.Popups.ShowError(workFailure("Import", finished.Artist+" - "+finished.Album, finished.Files, finished.Err))
		}
	}
	// Finish import dropped the download.
	return loadDownloadsCmd(m.Core.Downloads())
}

// refreshLibraryViews reads the library again into the views that show it,
// keeping what is selected.
func (m *Model) refreshLibraryViews() {
	m.refreshLibraryNavigator(true)
	if m.Navigation.IsAlbumViewActive() {
		_ = m.Navigation.AlbumView().Refresh() //nolint:errcheck // the view keeps what it showed
	}
}

// workFailure says what failed in an import or a retag of album whose popup
// is closed: work is "Import" or "Retag".
func workFailure(work, album string, files []core.FileProgress, err error) string {
	if err != nil {
		return fmt.Sprintf("%s of %s: library not updated: %v", work, album, err)
	}
	failed := 0
	for i := range files {
		if s := files[i].State; s != core.FileDone && s != core.FileAlreadyImported {
			failed++
		}
	}
	return fmt.Sprintf("%s of %s: %d of %d files failed", work, album, failed, len(files))
}

// handleRetagEvent takes an event of the retag the core runs. The popup,
// when it is still open on that album, shows it; the end of the retag is also
// the model's: the files say something else, whether the user is still
// looking at the retag or closed the popup and went on.
func (m *Model) handleRetagEvent(e core.Event) tea.Cmd {
	rt := m.Popups.Retag()
	var cmd tea.Cmd
	if rt != nil {
		_, cmd = rt.Update(e) // it knows its own retag from another's
	}
	finished, ok := e.(core.RetagFinished)
	if !ok {
		return cmd
	}

	m.refreshLibraryViews()
	// The popup tells what failed; with none left, tell it here.
	watched := rt != nil && rt.Retags(finished.AlbumArtist, finished.Album)
	if !watched && (finished.Err != nil || !core.AllDone(finished.Files)) {
		m.Popups.ShowError(workFailure("Retag", finished.AlbumArtist+" - "+finished.Album, finished.Files, finished.Err))
	}
	return cmd
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

// handleSimilarArtistsMsg routes messages to the similar artists popup.
func (m Model) handleSimilarArtistsMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	sa := m.Popups.SimilarArtists()
	if sa == nil {
		return m, nil
	}
	_, cmd := sa.Update(msg)
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

// toDownloadPopup hands an event of the core to the download popup, when it
// is open: it knows which ones are its own.
func (m Model) toDownloadPopup(e core.Event) tea.Cmd {
	dl := m.Popups.Download()
	if dl == nil {
		return nil
	}
	_, cmd := dl.Update(e)
	return cmd
}

// handleDownloadMsgCategory handles download-related messages.
func (m Model) handleDownloadMsgCategory(msg DownloadMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceDownloadsChangedMsg:
		m.DownloadsView.SetDownloads(msg.Downloads)
		return m, m.WatchServiceEvents()

	case ServiceSearchMsg:
		return m, tea.Batch(m.toDownloadPopup(msg.Event), m.WatchServiceEvents())

	case ServiceReleasesMsg:
		// With the list closed there is nothing to do: the core remembers
		// how the refresh ended, and the list asks it when it opens.
		return m, tea.Batch(m.toDownloadPopup(msg.Event), m.WatchServiceEvents())

	case ServiceImportMsg:
		cmd := m.handleImportEvent(msg.Event)
		return m, tea.Batch(cmd, m.WatchServiceEvents())

	case DownloadsChangedMsg:
		if msg.Downloads != nil {
			m.DownloadsView.SetDownloads(msg.Downloads)
		}
		m.Popups.ShowOpError(msg.Op, msg.Err)
		return m, nil
	}

	return m, nil
}

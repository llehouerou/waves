// internal/app/update_loading.go
package app

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/albumpreset"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/ui/albumview"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/librarynav"
	"github.com/llehouerou/waves/internal/ui/playlistnav"
)

// handleLoadingMsg routes loading-related messages.
func (m Model) handleLoadingMsg(msg LoadingMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case InitResult:
		return m.handleInitResult(msg)
	case ShowLoadingMsg:
		return m.handleShowLoading()
	case HideLoadingMsg:
		return m.handleHideLoading()
	case LoadingTickMsg:
		if m.loadingState == loadingShowing {
			m.LoadingFrame++
			return m, LoadingTickCmd()
		}
	}
	return m, nil
}

// handleInitResult applies the async initialization result.
func (m Model) handleInitResult(msg InitResult) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.loadingState = loadingDone
		m.Popups.ShowOpError(errmsg.OpInitialize, msg.Err)
		return m, nil
	}

	// Apply the loaded state
	if fileNav, ok := msg.FileNav.(navigator.Model[navigator.FileNode]); ok {
		m.Navigation.SetFileNav(fileNav)
	}
	if libNav, ok := msg.LibNav.(navigator.Model[librarynav.Node]); ok {
		m.Navigation.SetLibraryNav(libNav)
	}
	// Initialize album view with library
	av := albumview.New(m.Core.Library())
	// Restore album view settings if available
	if msg.SavedAlbumGroupFields != "" || msg.SavedAlbumSortCriteria != "" {
		coreSettings, err := albumpreset.FromJSON(msg.SavedAlbumGroupFields, msg.SavedAlbumSortCriteria)
		if err == nil {
			av.SetSettings(albumview.Settings{Settings: coreSettings})
		}
	}
	// Apply library browser
	if browser, ok := msg.LibraryBrowser.(librarybrowser.Model); ok {
		restoreBrowserSelection(&browser, msg.SavedBrowserState)
		m.Navigation.SetLibraryBrowser(browser)
	}
	// Restore library sub-mode and album selection
	switch msg.SavedLibrarySubMode {
	case "album":
		m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
		// Load albums and restore selection
		if err := av.Refresh(); err == nil && msg.SavedAlbumSelectedID != "" {
			av.SelectByID(msg.SavedAlbumSelectedID)
		}
	case "miller":
		// Migrate from old Miller view to new Browser view
		m.Navigation.SetLibrarySubMode(navctl.LibraryModeBrowser)
	default:
		// Default to browser view (new installs and "browser" value)
		m.Navigation.SetLibrarySubMode(navctl.LibraryModeBrowser)
	}
	m.Navigation.SetAlbumView(av)
	if plsNav, ok := msg.PlsNav.(navigator.Model[playlistnav.Node]); ok {
		m.Navigation.SetPlaylistNav(plsNav)
	}

	m.Navigation.SetViewMode(msg.SavedView)
	// Set focus to update album view focus state based on sub-mode
	m.Navigation.SetFocus(navctl.FocusNavigator)
	m.loadingInitDone = true
	m.initConfig = nil
	m.updateHasLibrarySources()
	m.ResizeComponents()

	// Center browser cursors now that dimensions are known
	m.Navigation.LibraryBrowser().CenterCursors()

	// Sync queue cursor to current playing track (must be after resize for correct height)
	m.Layout.QueuePanel().SyncCursor()

	// Load favorites and update navigators
	m.RefreshFavorites()

	// Load downloads if starting on downloads view
	var downloadsRefreshCmd tea.Cmd
	if msg.SavedView == navctl.ViewDownloads && m.Core.Downloads().Configured() {
		m.DownloadsView.SetFocused(true)
		downloadsRefreshCmd = m.loadAndRefreshDownloads()
	}

	// Helper to batch downloads refresh and service events with other commands
	withCommonCmds := func(cmds ...tea.Cmd) tea.Cmd {
		allCmds := append([]tea.Cmd{m.WatchServiceEvents()}, cmds...)
		if downloadsRefreshCmd != nil {
			allCmds = append(allCmds, downloadsRefreshCmd)
		}
		return tea.Batch(allCmds...)
	}

	// Decide whether to transition to done based on current phase
	switch m.loadingState {
	case loadingWaiting:
		if msg.IsFirstLaunch {
			// First launch: show loading screen for 3 seconds
			m.loadingState = loadingShowing
			m.loadingShowTime = time.Now()
			return m, withCommonCmds(LoadingTickCmd(), HideLoadingAfterCmd(loadingMinTimeFirstLaunch))
		}
		// Init finished before show delay - never show loading screen
		m.loadingState = loadingDone
		return m, withCommonCmds()
	case loadingShowing:
		minTime := loadingMinTime
		if msg.IsFirstLaunch {
			minTime = loadingMinTimeFirstLaunch
		}
		remaining := minTime - time.Since(m.loadingShowTime)
		if remaining <= 0 {
			m.loadingState = loadingDone
			return m, withCommonCmds()
		}
		// The HideLoadingMsg armed when the screen showed may have come and
		// gone: arm one for the time that remains.
		return m, withCommonCmds(HideLoadingAfterCmd(remaining))
	case loadingDone:
		// Already done (shouldn't happen)
		return m, withCommonCmds()
	}

	return m, withCommonCmds()
}

// handleShowLoading shows the loading screen if initialization still runs.
// Applying a successful InitResult leaves the waiting state, so a late
// ShowLoadingMsg finds nothing to show.
func (m Model) handleShowLoading() (tea.Model, tea.Cmd) {
	if m.loadingState != loadingWaiting {
		return m, nil
	}
	m.loadingState = loadingShowing
	m.loadingShowTime = time.Now()
	// On a first launch, InitResult arms a HideLoadingMsg for the rest of the 3 s.
	return m, tea.Batch(LoadingTickCmd(), HideLoadingAfterCmd(loadingMinTime))
}

// handleHideLoading hides the loading screen once initialization is done. It
// starts nothing that reads the core's events: the InitResult did. A launch
// may get two HideLoadingMsg: the one after the screen hid does nothing.
func (m Model) handleHideLoading() (tea.Model, tea.Cmd) {
	if m.loadingState == loadingShowing && m.loadingInitDone {
		m.loadingState = loadingDone
	}
	return m, nil
}

// updateHasLibrarySources updates the cached HasLibrarySources flag.
func (m *Model) updateHasLibrarySources() {
	sources, err := m.Core.Library().Sources()
	m.HasLibrarySources = err == nil && len(sources) > 0
}

// restoreBrowserSelection restores a browser's artist/album/track/column selection from saved state.
// The state format is "column\x00artist\x00album\x00trackID".
func restoreBrowserSelection(browser *librarybrowser.Model, savedState string) {
	if savedState == "" {
		return
	}
	parts := strings.SplitN(savedState, "\x00", 4)
	if len(parts) < 2 {
		return
	}
	// Restore active column
	if col, err := strconv.Atoi(parts[0]); err == nil {
		browser.SetActiveColumn(librarybrowser.Column(col))
	}
	// Restore artist
	if parts[1] != "" {
		browser.SelectArtist(parts[1])
	}
	// Restore album
	if len(parts) >= 3 && parts[2] != "" {
		browser.SelectAlbum(parts[2])
	}
	// Restore track
	if len(parts) >= 4 && parts[3] != "" {
		if trackID, err := strconv.ParseInt(parts[3], 10, 64); err == nil {
			browser.SelectTrackByID(trackID)
		}
	}
}

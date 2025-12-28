// internal/app/update_loading.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/ui/albumview"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
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
		m.Popups.ShowError(errmsg.Format(errmsg.OpInitialize, msg.Err))
		return m, nil
	}

	// Apply the loaded state
	if fileNav, ok := msg.FileNav.(navigator.Model[navigator.FileNode]); ok {
		m.Navigation.SetFileNav(fileNav)
	}
	if libNav, ok := msg.LibNav.(navigator.Model[library.Node]); ok {
		m.Navigation.SetLibraryNav(libNav)
	}
	// Initialize album view with library
	av := albumview.New(m.Library)
	// Restore album view settings if available
	if msg.SavedAlbumGroupFields != "" || msg.SavedAlbumSortCriteria != "" {
		settings, err := albumview.SettingsFromJSON(msg.SavedAlbumGroupFields, msg.SavedAlbumSortCriteria)
		if err == nil {
			av.SetSettings(settings)
		}
	}
	// Restore library sub-mode and album selection
	if msg.SavedLibrarySubMode == "album" {
		m.Navigation.SetLibrarySubMode(LibraryModeAlbum)
		// Load albums and restore selection
		if err := av.Refresh(); err == nil && msg.SavedAlbumSelectedID != "" {
			av.SelectByID(msg.SavedAlbumSelectedID)
		}
	}
	m.Navigation.SetAlbumView(av)
	if plsNav, ok := msg.PlsNav.(navigator.Model[playlists.Node]); ok {
		m.Navigation.SetPlaylistNav(plsNav)
	}
	if queue, ok := msg.Queue.(*playlist.PlayingQueue); ok {
		// Close the old service to stop its goroutines and clean up subscriptions
		_ = m.PlaybackService.Close()
		// Recreate PlaybackService with the restored queue
		// (the old service had an empty queue created during New())
		m.PlaybackService = playback.New(m.PlaybackService.Player(), queue)
		m.playbackSub = m.PlaybackService.Subscribe()
	}
	if queuePanel, ok := msg.QueuePanel.(queuepanel.Model); ok {
		m.Layout.SetQueuePanel(queuePanel)
	}

	m.Navigation.SetViewMode(msg.SavedView)
	// Set focus to update album view focus state based on sub-mode
	m.Navigation.SetFocus(FocusNavigator)
	m.loadingInitDone = true
	m.loadingFirstLaunch = msg.IsFirstLaunch
	m.initConfig = nil
	m.updateHasLibrarySources()
	m.ResizeComponents()

	// Sync queue cursor to current playing track (must be after resize for correct height)
	m.Layout.QueuePanel().SyncCursor()

	// Load favorites and update navigators
	m.RefreshFavorites()

	// Ensure FTS search index exists (only builds if empty)
	_ = m.Library.EnsureFTSIndex()

	// Load downloads if starting on downloads view
	var downloadsRefreshCmd tea.Cmd
	if msg.SavedView == ViewDownloads && m.HasSlskdConfig {
		downloads, err := m.Downloads.List()
		if err == nil {
			m.DownloadsView.SetDownloads(downloads)
		}
		m.DownloadsView.SetFocused(true)
		// Start periodic refresh
		downloadsRefreshCmd = func() tea.Msg {
			return DownloadsRefreshMsg{}
		}
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
			return m, withCommonCmds(LoadingTickCmd(), HideLoadingFirstLaunchCmd())
		}
		// Init finished before show delay - never show loading screen
		m.loadingState = loadingDone
		return m, withCommonCmds()
	case loadingShowing:
		// Check if minimum display time has elapsed
		minTime := 800 * time.Millisecond
		if m.loadingFirstLaunch {
			minTime = 3 * time.Second
		}
		if time.Since(m.loadingShowTime) >= minTime {
			m.loadingState = loadingDone
			return m, withCommonCmds()
		}
		// Otherwise wait for HideLoadingMsg - still need to start service events
		cmds := []tea.Cmd{m.WatchServiceEvents()}
		if downloadsRefreshCmd != nil {
			cmds = append(cmds, downloadsRefreshCmd)
		}
		return m, tea.Batch(cmds...)
	case loadingDone:
		// Already done (shouldn't happen)
		return m, withCommonCmds()
	}

	return m, withCommonCmds()
}

// handleShowLoading transitions to showing state if still waiting.
func (m Model) handleShowLoading() (tea.Model, tea.Cmd) {
	// Only show if we're still waiting (init not done)
	if m.loadingState != loadingWaiting {
		return m, nil
	}

	if m.loadingInitDone {
		if m.loadingFirstLaunch {
			// First launch: show loading screen for 3 seconds even though init is done
			m.loadingState = loadingShowing
			m.loadingShowTime = time.Now()
			return m, tea.Batch(LoadingTickCmd(), HideLoadingFirstLaunchCmd())
		}
		// Init finished during the delay - go straight to done
		m.loadingState = loadingDone
		return m, m.WatchServiceEvents()
	}

	// Show the loading screen (init still running)
	m.loadingState = loadingShowing
	m.loadingShowTime = time.Now()
	// Use first launch timer if applicable (we'll know from InitResult later)
	return m, tea.Batch(LoadingTickCmd(), HideLoadingAfterMinTimeCmd())
}

// handleHideLoading transitions to done state if init is complete.
func (m Model) handleHideLoading() (tea.Model, tea.Cmd) {
	// Only hide if we're showing and init is done
	if m.loadingState != loadingShowing {
		return m, nil
	}

	if m.loadingInitDone {
		m.loadingState = loadingDone
		return m, m.WatchServiceEvents()
	}

	// Init not done yet - keep showing, wait for InitResult
	return m, nil
}

// updateHasLibrarySources updates the cached HasLibrarySources flag.
func (m *Model) updateHasLibrarySources() {
	sources, err := m.Library.Sources()
	m.HasLibrarySources = err == nil && len(sources) > 0
}

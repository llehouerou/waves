// internal/app/update_loading.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
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
	if msg.Error != nil {
		m.loadingState = loadingDone
		m.Popups.ShowError("Failed to initialize: " + msg.Error.Error())
		return m, nil
	}

	// Apply the loaded state
	if fileNav, ok := msg.FileNav.(navigator.Model[navigator.FileNode]); ok {
		m.Navigation.SetFileNav(fileNav)
	}
	if libNav, ok := msg.LibNav.(navigator.Model[library.Node]); ok {
		m.Navigation.SetLibraryNav(libNav)
	}
	if plsNav, ok := msg.PlsNav.(navigator.Model[playlists.Node]); ok {
		m.Navigation.SetPlaylistNav(plsNav)
	}
	if queue, ok := msg.Queue.(*playlist.PlayingQueue); ok {
		m.Playback.SetQueue(queue)
	}
	if queuePanel, ok := msg.QueuePanel.(queuepanel.Model); ok {
		m.Layout.SetQueuePanel(queuePanel)
	}

	m.Navigation.SetViewMode(msg.SavedView)
	m.loadingInitDone = true
	m.loadingFirstLaunch = msg.IsFirstLaunch
	m.initConfig = nil
	m.updateHasLibrarySources()
	m.ResizeComponents()

	// Pre-load search cache for fast search popup
	_ = m.Library.RefreshSearchCache()

	// Decide whether to transition to done based on current phase
	switch m.loadingState {
	case loadingWaiting:
		if msg.IsFirstLaunch {
			// First launch: show loading screen for 3 seconds
			m.loadingState = loadingShowing
			m.loadingShowTime = time.Now()
			return m, tea.Batch(LoadingTickCmd(), HideLoadingFirstLaunchCmd())
		}
		// Init finished before show delay - never show loading screen
		m.loadingState = loadingDone
		return m, m.WatchTrackFinished()
	case loadingShowing:
		// Check if minimum display time has elapsed
		minTime := 800 * time.Millisecond
		if m.loadingFirstLaunch {
			minTime = 3 * time.Second
		}
		if time.Since(m.loadingShowTime) >= minTime {
			m.loadingState = loadingDone
			return m, m.WatchTrackFinished()
		}
		// Otherwise wait for HideLoadingMsg
		return m, nil
	case loadingDone:
		// Already done (shouldn't happen)
		return m, m.WatchTrackFinished()
	}

	return m, m.WatchTrackFinished()
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
		return m, m.WatchTrackFinished()
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
		return m, m.WatchTrackFinished()
	}

	// Init not done yet - keep showing, wait for InitResult
	return m, nil
}

// updateHasLibrarySources updates the cached HasLibrarySources flag.
func (m *Model) updateHasLibrarySources() {
	sources, err := m.Library.Sources()
	m.HasLibrarySources = err == nil && len(sources) > 0
}

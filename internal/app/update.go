// internal/app/update.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/confirm"
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

	// External messages from ui packages (cannot implement our interfaces)
	case queuepanel.JumpToTrackMsg:
		cmd := m.PlayTrackAtIndex(msg.Index)
		return m, cmd

	case queuepanel.QueueChangedMsg:
		m.SaveQueueState()
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
		m.Popups.HideLibrarySources()
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
		m.Popups.HideHelp()
		return m, nil
	}

	return m, nil
}

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

// handlePlaybackMsg routes playback-related messages.
func (m Model) handlePlaybackMsg(msg PlaybackMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TrackFinishedMsg:
		return m.handleTrackFinished()
	case TrackSkipTimeoutMsg:
		return m.handleTrackSkipTimeout(msg)
	case TickMsg:
		if m.Playback.IsPlaying() {
			return m, TickCmd()
		}
	}
	return m, nil
}

// handleNavigationMsg routes navigation-related messages.
func (m Model) handleNavigationMsg(msg NavigationMessage) (tea.Model, tea.Cmd) {
	if scanMsg, ok := msg.(ScanResultMsg); ok {
		return m.handleScanResult(scanMsg)
	}
	return m, nil
}

// handleInputMsg routes input-related messages.
func (m Model) handleInputMsg(msg InputMessage) (tea.Model, tea.Cmd) {
	if _, ok := msg.(KeySequenceTimeoutMsg); ok {
		return m.handleKeySequenceTimeout()
	}
	return m, nil
}

// handleLibraryScanMsg routes library scan messages.
func (m Model) handleLibraryScanMsg(msg LibraryScanMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LibraryScanProgressMsg:
		return m.handleLibraryScanProgress(msg)
	case LibraryScanCompleteMsg:
		m.LibraryScanJob = nil
		m.LibraryScanCh = nil
		m.ResizeComponents()
		// Refresh search cache after scan
		_ = m.Library.RefreshSearchCache()
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

func (m Model) handleNavigatorMiddleClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Alt {
		// Alt+middle click: play container (like alt+enter)
		if m.Navigation.ViewMode().SupportsContainerPlay() {
			if cmd := m.HandleContainerAndPlay(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil
	}

	// Middle click: navigate if container, play if track
	if m.isSelectedItemContainer() {
		// Navigate into container - let navigator handle it
		return m.routeMouseToNavigator(msg)
	}

	// Play track (like enter on a track)
	if cmd := m.HandleQueueAction(QueueAddAndPlay); cmd != nil {
		return m, cmd
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

func (m Model) handleTrackFinished() (tea.Model, tea.Cmd) {
	if m.Playback.Queue().HasNext() {
		next := m.Playback.Queue().Next()
		m.SaveQueueState()
		m.Layout.QueuePanel().SyncCursor()
		cmd := m.PlayTrack(next.Path)
		if cmd != nil {
			return m, tea.Batch(cmd, m.WatchTrackFinished())
		}
		return m, m.WatchTrackFinished()
	}
	m.Playback.Stop()
	m.ResizeComponents()
	return m, m.WatchTrackFinished()
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
		m.handlePlaybackKeys,
		m.handleNavigatorActionKeys,
		m.handlePlaylistKeys,
		m.handleLibraryKeys,
	}

	for _, h := range handlers {
		if handled, cmd := h(key); handled {
			return m, cmd
		}
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

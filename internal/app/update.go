// internal/app/update.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
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
	case InitResult:
		return m.handleInitResult(msg)

	case LoadingTickMsg:
		if m.Loading {
			m.LoadingFrame++
			return m, LoadingTickCmd()
		}
		return m, nil

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case TrackFinishedMsg:
		return m.handleTrackFinished()

	case queuepanel.JumpToTrackMsg:
		cmd := m.PlayTrackAtIndex(msg.Index)
		return m, cmd

	case queuepanel.QueueChangedMsg:
		m.SaveQueueState()
		return m, nil

	case navigator.NavigationChangedMsg:
		m.SaveNavigationState()
		return m, nil

	case ScanResultMsg:
		return m.handleScanResult(msg)

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
		m.ShowLibrarySourcesPopup = false
		m.LibrarySourcesPopup.Reset()
		// Continue listening for scan progress if a scan is running
		return m, m.waitForLibraryScan()

	case librarysources.RequestTrackCountMsg:
		count, err := m.Library.TrackCountBySource(msg.Path)
		if err != nil {
			m.ErrorMsg = err.Error()
			return m, nil
		}
		m.LibrarySourcesPopup.EnterConfirmMode(count)
		return m, nil

	case LibraryScanProgressMsg:
		return m.handleLibraryScanProgress(msg)

	case LibraryScanCompleteMsg:
		m.LibraryScanJob = nil
		m.LibraryScanCh = nil
		m.ResizeComponents()
		return m, nil

	case helpbindings.CloseMsg:
		m.ShowHelpPopup = false
		return m, nil

	case KeySequenceTimeoutMsg:
		return m.handleKeySequenceTimeout()

	case TrackSkipTimeoutMsg:
		return m.handleTrackSkipTimeout(msg)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)

	case TickMsg:
		if m.Player.State() == player.Playing {
			return m, TickCmd()
		}
	}

	return m, nil
}

func (m Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Route mouse events to focused component
	if m.Focus == FocusQueue && m.QueueVisible {
		var cmd tea.Cmd
		m.QueuePanel, cmd = m.QueuePanel.Update(msg)
		return m, cmd
	}

	if m.Focus == FocusNavigator {
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
		if m.ViewMode.SupportsContainerPlay() {
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
	switch m.ViewMode {
	case ViewFileBrowser:
		if sel := m.FileNavigator.Selected(); sel != nil {
			return sel.IsContainer()
		}
	case ViewPlaylists:
		if sel := m.PlaylistNavigator.Selected(); sel != nil {
			return sel.IsContainer()
		}
	case ViewLibrary:
		if sel := m.LibraryNavigator.Selected(); sel != nil {
			return sel.IsContainer()
		}
	}
	return false
}

func (m Model) routeMouseToNavigator(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.ViewMode {
	case ViewFileBrowser:
		m.FileNavigator, cmd = m.FileNavigator.Update(msg)
	case ViewPlaylists:
		m.PlaylistNavigator, cmd = m.PlaylistNavigator.Update(msg)
	case ViewLibrary:
		m.LibraryNavigator, cmd = m.LibraryNavigator.Update(msg)
	}
	return m, cmd
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.Width = msg.Width
	m.Height = msg.Height
	m.Search, _ = m.Search.Update(msg)
	m.ResizeComponents()
	return m, nil
}

func (m Model) handleInitResult(msg InitResult) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.Loading = false
		m.ErrorMsg = "Failed to initialize: " + msg.Error.Error()
		return m, nil
	}

	// Apply the loaded state
	if fileNav, ok := msg.FileNav.(navigator.Model[navigator.FileNode]); ok {
		m.FileNavigator = fileNav
	}
	if libNav, ok := msg.LibNav.(navigator.Model[library.Node]); ok {
		m.LibraryNavigator = libNav
	}
	if plsNav, ok := msg.PlsNav.(navigator.Model[playlists.Node]); ok {
		m.PlaylistNavigator = plsNav
	}
	if queue, ok := msg.Queue.(*playlist.PlayingQueue); ok {
		m.Queue = queue
	}
	if queuePanel, ok := msg.QueuePanel.(queuepanel.Model); ok {
		m.QueuePanel = queuePanel
	}

	m.ViewMode = msg.SavedView
	m.Loading = false
	m.initConfig = nil
	m.updateHasLibrarySources()
	m.ResizeComponents()

	return m, m.WatchTrackFinished()
}

// updateHasLibrarySources updates the cached HasLibrarySources flag.
func (m *Model) updateHasLibrarySources() {
	sources, err := m.Library.Sources()
	m.HasLibrarySources = err == nil && len(sources) > 0
}

func (m Model) handleTrackFinished() (tea.Model, tea.Cmd) {
	if m.Queue.HasNext() {
		next := m.Queue.Next()
		m.SaveQueueState()
		m.QueuePanel.SyncCursor()
		cmd := m.PlayTrack(next.Path)
		if cmd != nil {
			return m, tea.Batch(cmd, m.WatchTrackFinished())
		}
		return m, m.WatchTrackFinished()
	}
	m.Player.Stop()
	m.ResizeComponents()
	return m, m.WatchTrackFinished()
}

func (m Model) handleKeySequenceTimeout() (tea.Model, tea.Cmd) {
	if m.PendingKeys == " " {
		m.PendingKeys = ""
		if cmd := m.HandleSpaceAction(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) handleTrackSkipTimeout(msg TrackSkipTimeoutMsg) (tea.Model, tea.Cmd) {
	if msg.Version == m.TrackSkipVersion {
		cmd := m.PlayTrackAtIndex(m.PendingTrackIdx)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle error overlay - any key dismisses it
	if m.ErrorMsg != "" {
		m.ErrorMsg = ""
		return m, nil
	}

	// Handle scan report popup - Enter/Escape dismisses it
	if m.ScanReportPopup != nil {
		key := msg.String()
		if key == "enter" || key == "escape" {
			m.ScanReportPopup = nil
		}
		return m, nil
	}

	// Handle help popup
	if m.ShowHelpPopup {
		var cmd tea.Cmd
		m.HelpPopup, cmd = m.HelpPopup.Update(msg)
		return m, cmd
	}

	// Handle confirmation dialog
	if m.Confirm.Active() {
		var cmd tea.Cmd
		m.Confirm, cmd = m.Confirm.Update(msg)
		return m, cmd
	}

	// Handle text input mode
	if m.InputMode != InputNone {
		var cmd tea.Cmd
		m.TextInput, cmd = m.TextInput.Update(msg)
		return m, cmd
	}

	// Handle library sources popup
	if m.ShowLibrarySourcesPopup {
		var cmd tea.Cmd
		m.LibrarySourcesPopup, cmd = m.LibrarySourcesPopup.Update(msg)
		return m, cmd
	}

	// Handle search mode (regular search or add-to-playlist)
	if m.SearchMode || m.AddToPlaylistMode {
		var cmd tea.Cmd
		m.Search, cmd = m.Search.Update(msg)
		return m, cmd
	}

	key := msg.String()

	// Handle key sequences starting with 'g'
	if m.PendingKeys == "g" {
		return m.handleGSequence(key)
	}

	// Handle queue panel input when focused
	if m.Focus == FocusQueue && m.QueueVisible {
		var cmd tea.Cmd
		m.QueuePanel, cmd = m.QueuePanel.Update(msg)
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

func (m Model) handleGSequence(key string) (tea.Model, tea.Cmd) {
	m.PendingKeys = ""

	switch key {
	case "f":
		// Deep search in file browser or library
		switch m.ViewMode {
		case ViewFileBrowser:
			m.SearchMode = true
			m.Search.SetLoading(true)
			ctx, cancel := context.WithCancel(context.Background())
			m.CancelScan = cancel
			m.ScanChan = navigator.ScanDir(ctx, m.FileNavigator.CurrentPath())
			return m, m.waitForScan()
		case ViewLibrary:
			m.SearchMode = true
			m.Search.SetItems(m.AllLibrarySearchItems())
			m.Search.SetLoading(false)
			return m, nil
		case ViewPlaylists:
			// Not supported in playlists view
		}
	case "p":
		// Open library sources popup
		if m.ViewMode == ViewLibrary {
			sources, err := m.Library.Sources()
			if err != nil {
				m.ErrorMsg = err.Error()
				return m, nil
			}
			m.LibrarySourcesPopup.SetSources(sources)
			m.LibrarySourcesPopup.SetSize(m.Width, m.Height)
			m.ShowLibrarySourcesPopup = true
			return m, nil
		}
	case "r":
		// Incremental library refresh
		if m.ViewMode == ViewLibrary && m.LibraryScanCh == nil {
			sources, err := m.Library.Sources()
			if err != nil || len(sources) == 0 {
				return m, nil
			}
			ch := make(chan library.ScanProgress)
			m.LibraryScanCh = ch
			go func() {
				_ = m.Library.Refresh(sources, ch)
			}()
			return m, m.waitForLibraryScan()
		}
	case "R":
		// Full library rescan
		if m.ViewMode == ViewLibrary && m.LibraryScanCh == nil {
			sources, err := m.Library.Sources()
			if err != nil || len(sources) == 0 {
				return m, nil
			}
			ch := make(chan library.ScanProgress)
			m.LibraryScanCh = ch
			go func() {
				_ = m.Library.FullRefresh(sources, ch)
			}()
			return m, m.waitForLibraryScan()
		}
	}

	return m, nil
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
	if m.Focus == FocusNavigator {
		var cmd tea.Cmd
		switch m.ViewMode {
		case ViewFileBrowser:
			m.FileNavigator, cmd = m.FileNavigator.Update(msg)
		case ViewPlaylists:
			m.PlaylistNavigator, cmd = m.PlaylistNavigator.Update(msg)
		case ViewLibrary:
			m.LibraryNavigator, cmd = m.LibraryNavigator.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

func (m *Model) handleSeek(seconds int) {
	if time.Since(m.LastSeekTime) < 150*time.Millisecond {
		return
	}
	m.LastSeekTime = time.Now()
	m.Player.Seek(time.Duration(seconds) * time.Second)
}

func (m Model) waitForScan() tea.Cmd {
	ch := m.ScanChan
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		result, ok := <-ch
		if !ok {
			return ScanResultMsg{Done: true}
		}
		return ScanResultMsg(result)
	}
}

func (m Model) waitForLibraryScan() tea.Cmd {
	ch := m.LibraryScanCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return LibraryScanCompleteMsg{}
		}
		return LibraryScanProgressMsg(progress)
	}
}

// internal/app/update.go
package app

import (
	"context"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/confirm"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
	"github.com/llehouerou/waves/internal/ui/textinput"
)

// Update handles messages and returns updated model and commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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

	case LibraryScanProgressMsg:
		return m.handleLibraryScanProgress(msg)

	case LibraryScanCompleteMsg:
		m.LibraryScanJob = nil
		m.LibraryScanCh = nil
		m.ResizeComponents()
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
		if m.ViewMode == ViewLibrary || m.ViewMode == ViewPlaylists {
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

func (m Model) handleTrackFinished() (tea.Model, tea.Cmd) {
	if m.Queue.HasNext() {
		next := m.Queue.Next()
		if err := m.Player.Play(next.Path); err != nil {
			m.ErrorMsg = err.Error()
			return m, m.WatchTrackFinished()
		}
		m.SaveQueueState()
		m.QueuePanel.SyncCursor()
		return m, tea.Batch(TickCmd(), m.WatchTrackFinished())
	}
	m.Player.Stop()
	m.ResizeComponents()
	return m, m.WatchTrackFinished()
}

func (m Model) handleScanResult(msg ScanResultMsg) (tea.Model, tea.Cmd) {
	m.Search.SetItems(msg.Items)
	m.Search.SetLoading(!msg.Done)
	if !msg.Done {
		return m, m.waitForScan()
	}
	return m, nil
}

func (m Model) handleSearchResult(msg search.ResultMsg) (tea.Model, tea.Cmd) {
	// Handle add-to-playlist mode
	if m.AddToPlaylistMode {
		return m.handleAddToPlaylistResult(msg)
	}

	m.SearchMode = false
	m.ScanChan = nil
	if m.CancelScan != nil {
		m.CancelScan()
		m.CancelScan = nil
	}
	if !msg.Canceled && msg.Item != nil {
		switch item := msg.Item.(type) {
		case navigator.FileItem:
			m.FileNavigator.NavigateTo(item.Path)
		case library.SearchItem:
			m.HandleLibrarySearchResult(item.Result)
		}
	}
	m.Search.Reset()
	return m, nil
}

func (m Model) handleAddToPlaylistResult(msg search.ResultMsg) (tea.Model, tea.Cmd) {
	m.AddToPlaylistMode = false
	trackIDs := m.AddToPlaylistTracks
	m.AddToPlaylistTracks = nil
	m.Search.Reset()

	if msg.Canceled || msg.Item == nil {
		return m, nil
	}

	item, ok := msg.Item.(playlists.SearchItem)
	if !ok {
		return m, nil
	}

	if err := m.Playlists.AddTracks(item.ID, trackIDs); err != nil {
		m.ErrorMsg = err.Error()
		return m, nil
	}

	// Update last used timestamp
	_ = m.Playlists.UpdateLastUsed(item.ID)

	// Refresh playlist navigator so new tracks are visible
	selectedID := m.PlaylistNavigator.SelectedID()
	plsSource := playlists.NewSource(m.Playlists)
	if newNav, err := navigator.New(plsSource); err == nil {
		m.PlaylistNavigator = newNav
		m.PlaylistNavigator, _ = m.PlaylistNavigator.Update(tea.WindowSizeMsg{
			Width:  m.NavigatorWidth(),
			Height: m.NavigatorHeight(),
		})
		if selectedID != "" {
			m.PlaylistNavigator.FocusByID(selectedID)
		}
		m.PlaylistNavigator.SetFocused(m.Focus == FocusNavigator && m.ViewMode == ViewPlaylists)
	}

	return m, nil
}

func (m Model) handleLibraryScanProgress(msg LibraryScanProgressMsg) (tea.Model, tea.Cmd) {
	switch msg.Phase {
	case "scanning":
		m.LibraryScanJob = &jobbar.Job{
			ID:      "library-refresh",
			Label:   "Scanning library",
			Current: msg.Current,
			Total:   0, // Unknown during scanning
		}
		m.ResizeComponents()
	case "processing":
		m.LibraryScanJob = &jobbar.Job{
			ID:      "library-refresh",
			Label:   "Processing files",
			Current: msg.Current,
			Total:   msg.Total,
		}
	case "cleaning":
		m.LibraryScanJob = &jobbar.Job{
			ID:      "library-refresh",
			Label:   "Cleaning up removed files",
			Current: 0,
			Total:   0,
		}
	case "done":
		m.LibraryScanJob = nil
		m.LibraryScanCh = nil

		// Preserve current selection before refreshing
		selectedID := m.LibraryNavigator.SelectedID()

		// Recreate navigator with fresh data
		libSource := library.NewSource(m.Library)
		if newNav, err := navigator.New(libSource); err == nil {
			m.LibraryNavigator = newNav
			m.LibraryNavigator, _ = m.LibraryNavigator.Update(tea.WindowSizeMsg{
				Width:  m.NavigatorWidth(),
				Height: m.NavigatorHeight(),
			})

			// Restore selection if still available
			if selectedID != "" {
				m.LibraryNavigator.FocusByID(selectedID)
			}

			// Restore focus state
			m.LibraryNavigator.SetFocused(m.Focus == FocusNavigator && m.ViewMode == ViewLibrary)
		}
		m.ResizeComponents()
		return m, nil
	}
	return m, m.waitForLibraryScan()
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

	// Handle search mode (regular search or add-to-playlist)
	if m.SearchMode || m.AddToPlaylistMode {
		var cmd tea.Cmd
		m.Search, cmd = m.Search.Update(msg)
		return m, cmd
	}

	key := msg.String()

	// Handle key sequences starting with space
	if m.PendingKeys != "" {
		return m.handlePendingKeys(key)
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

func (m Model) handlePendingKeys(key string) (tea.Model, tea.Cmd) {
	m.PendingKeys += key
	switch {
	case m.PendingKeys == " ff" && m.ViewMode == ViewFileBrowser:
		m.PendingKeys = ""
		m.SearchMode = true
		m.Search.SetLoading(true)
		ctx, cancel := context.WithCancel(context.Background())
		m.CancelScan = cancel
		m.ScanChan = navigator.ScanDir(ctx, m.FileNavigator.CurrentPath())
		return m, m.waitForScan()
	case m.PendingKeys == " lr" && m.ViewMode == ViewLibrary:
		m.PendingKeys = ""
		if len(m.LibrarySources) > 0 && m.LibraryScanCh == nil {
			ch := make(chan library.ScanProgress)
			m.LibraryScanCh = ch
			go func() {
				_ = m.Library.Refresh(m.LibrarySources, ch)
			}()
			return m, m.waitForLibraryScan()
		}
		return m, nil
	case len(m.PendingKeys) >= 3 || !IsValidSequencePrefix(m.PendingKeys):
		m.PendingKeys = ""
		if cmd := m.HandleSpaceAction(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) handleGlobalKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	handlers := []func(key string) (bool, tea.Cmd){
		m.handleQuitKeys,
		m.handleViewKeys,
		m.handleFocusKeys,
		m.handlePlaybackKeys,
		m.handleNavigatorActionKeys,
		m.handlePlaylistKeys,
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

func (m Model) handleTextInputResult(msg textinput.ResultMsg) (tea.Model, tea.Cmd) {
	m.InputMode = InputNone
	m.TextInput.Reset()

	if msg.Canceled || msg.Text == "" {
		return m, nil
	}

	ctx, ok := msg.Context.(PlaylistInputContext)
	if !ok {
		return m, nil
	}

	var navigateToID string

	switch ctx.Mode {
	case InputNone:
		// No action
	case InputNewPlaylist:
		id, err := m.Playlists.Create(ctx.FolderID, msg.Text)
		if err != nil {
			m.ErrorMsg = err.Error()
			return m, nil
		}
		navigateToID = "playlists:playlist:" + strconv.FormatInt(id, 10)
	case InputNewFolder:
		id, err := m.Playlists.CreateFolder(ctx.FolderID, msg.Text)
		if err != nil {
			m.ErrorMsg = err.Error()
			return m, nil
		}
		navigateToID = "playlists:folder:" + strconv.FormatInt(id, 10)
	case InputRename:
		var err error
		if ctx.IsFolder {
			err = m.Playlists.RenameFolder(ctx.ItemID, msg.Text)
		} else {
			err = m.Playlists.Rename(ctx.ItemID, msg.Text)
		}
		if err != nil {
			m.ErrorMsg = err.Error()
			return m, nil
		}
	}

	// Refresh and navigate to newly created item
	m.PlaylistNavigator.Refresh()
	if navigateToID != "" {
		m.PlaylistNavigator.NavigateTo(navigateToID)
	}
	return m, nil
}

func (m Model) handleConfirmResult(msg confirm.ResultMsg) (tea.Model, tea.Cmd) {
	m.Confirm.Reset()

	if !msg.Confirmed {
		return m, nil
	}

	ctx, ok := msg.Context.(DeleteConfirmContext)
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
		m.ErrorMsg = err.Error()
		return m, nil
	}

	// Refresh playlist navigator
	return m.refreshPlaylistNavigator()
}

func (m Model) refreshPlaylistNavigator() (tea.Model, tea.Cmd) {
	m.PlaylistNavigator.Refresh()
	return m, nil
}

// internal/app/update.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
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

	case TickMsg:
		if m.Player.State() == player.Playing {
			return m, TickCmd()
		}
	}

	// Route message to active navigator when focused
	if m.Focus == FocusNavigator {
		var cmd tea.Cmd
		if m.ViewMode == ViewFileBrowser {
			m.FileNavigator, cmd = m.FileNavigator.Update(msg)
		} else {
			m.LibraryNavigator, cmd = m.LibraryNavigator.Update(msg)
		}
		return m, cmd
	}

	return m, nil
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

	// Handle search mode
	if m.SearchMode {
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
	}

	for _, h := range handlers {
		if handled, cmd := h(key); handled {
			return m, cmd
		}
	}

	// Delegate unhandled keys to the active navigator
	if m.Focus == FocusNavigator {
		var cmd tea.Cmd
		if m.ViewMode == ViewFileBrowser {
			m.FileNavigator, cmd = m.FileNavigator.Update(msg)
		} else {
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

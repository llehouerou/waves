// internal/app/handlers.go
package app

import tea "github.com/charmbracelet/bubbletea"

// handleQuitKeys handles q and ctrl+c.
func (m *Model) handleQuitKeys(key string) (bool, tea.Cmd) {
	if key != "q" && key != "ctrl+c" {
		return false, nil
	}
	m.Player.Stop()
	m.SaveQueueState()
	m.StateMgr.Close()
	return true, tea.Quit
}

// handleViewKeys handles F1, F2 view switching.
func (m *Model) handleViewKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "f1":
		m.ViewMode = ViewLibrary
		m.SaveNavigationState()
		return true, nil
	case "f2":
		m.ViewMode = ViewFileBrowser
		m.SaveNavigationState()
		return true, nil
	}
	return false, nil
}

// handleFocusKeys handles tab and p (queue toggle).
func (m *Model) handleFocusKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "p":
		m.QueueVisible = !m.QueueVisible
		if !m.QueueVisible && m.Focus == FocusQueue {
			m.SetFocus(FocusNavigator)
		}
		m.ResizeComponents()
		return true, nil
	case "tab":
		if m.QueueVisible {
			if m.Focus == FocusQueue {
				m.SetFocus(FocusNavigator)
			} else {
				m.SetFocus(FocusQueue)
			}
		}
		return true, nil
	}
	return false, nil
}

// handlePlaybackKeys handles space, s, pgup/pgdown, seek, R, S.
func (m *Model) handlePlaybackKeys(key string) (bool, tea.Cmd) {
	switch key {
	case " ":
		m.PendingKeys = " "
		return true, KeySequenceTimeoutCmd()
	case "s":
		m.Player.Stop()
		m.ResizeComponents()
		return true, nil
	case "pgdown":
		return true, m.AdvanceToNextTrack()
	case "pgup":
		return true, m.GoToPreviousTrack()
	case "home":
		if !m.Queue.IsEmpty() {
			return true, m.JumpToQueueIndex(0)
		}
		return true, nil
	case "end":
		if !m.Queue.IsEmpty() {
			return true, m.JumpToQueueIndex(m.Queue.Len() - 1)
		}
		return true, nil
	case "v":
		m.TogglePlayerDisplayMode()
		return true, nil
	case "shift+left":
		m.handleSeek(-5)
		return true, nil
	case "shift+right":
		m.handleSeek(5)
		return true, nil
	case "R":
		m.Queue.CycleRepeatMode()
		m.SaveQueueState()
		return true, nil
	case "S":
		m.Queue.ToggleShuffle()
		m.SaveQueueState()
		return true, nil
	}
	return false, nil
}

// handleNavigatorActionKeys handles enter, a, r, alt+enter, /.
func (m *Model) handleNavigatorActionKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "/":
		m.SearchMode = true
		if m.ViewMode == ViewFileBrowser {
			m.Search.SetItems(m.CurrentDirSearchItems())
		} else {
			m.Search.SetItems(m.CurrentLibrarySearchItems())
		}
		m.Search.SetLoading(false)
		return true, nil
	case "enter":
		if m.Focus == FocusNavigator {
			if cmd := m.HandleQueueAction(QueueAddAndPlay); cmd != nil {
				return true, cmd
			}
		}
	case "alt+enter":
		if m.Focus == FocusNavigator && m.ViewMode == ViewLibrary {
			if cmd := m.HandleAddAlbumAndPlay(); cmd != nil {
				return true, cmd
			}
		}
	case "a":
		if m.Focus == FocusNavigator {
			if cmd := m.HandleQueueAction(QueueAdd); cmd != nil {
				return true, cmd
			}
		}
	case "r":
		if m.Focus == FocusNavigator {
			if cmd := m.HandleQueueAction(QueueReplace); cmd != nil {
				return true, cmd
			}
		}
	}
	return false, nil
}

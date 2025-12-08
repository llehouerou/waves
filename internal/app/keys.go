// internal/app/keys.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/navigator"
)

// handleGPrefixKey handles 'g' key to start a key sequence.
func (m *Model) handleGPrefixKey(key string) (bool, tea.Cmd) {
	if key == "g" && m.Focus == FocusNavigator {
		m.PendingKeys = "g"
		return true, nil
	}
	return false, nil
}

// handleKeySequenceTimeout handles timeout for key sequences like space.
func (m Model) handleKeySequenceTimeout() (tea.Model, tea.Cmd) {
	if m.PendingKeys == " " {
		m.PendingKeys = ""
		if cmd := m.HandleSpaceAction(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

// handleTrackSkipTimeout handles the debounced track skip after rapid key presses.
func (m Model) handleTrackSkipTimeout(msg TrackSkipTimeoutMsg) (tea.Model, tea.Cmd) {
	if msg.Version == m.TrackSkipVersion {
		cmd := m.PlayTrackAtIndex(m.PendingTrackIdx)
		return m, cmd
	}
	return m, nil
}

// handleGSequence handles key sequences starting with 'g'.
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
		if m.ViewMode == ViewLibrary {
			cmd := m.startLibraryScan(m.Library.Refresh)
			return m, cmd
		}
	case "R":
		// Full library rescan
		if m.ViewMode == ViewLibrary {
			cmd := m.startLibraryScan(m.Library.FullRefresh)
			return m, cmd
		}
	}

	return m, nil
}

// handleSeek handles seek operations with debouncing.
func (m *Model) handleSeek(seconds int) {
	if time.Since(m.LastSeekTime) < 150*time.Millisecond {
		return
	}
	m.LastSeekTime = time.Now()
	m.Player.Seek(time.Duration(seconds) * time.Second)
}

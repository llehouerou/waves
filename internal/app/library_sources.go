// internal/app/library_sources.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui/jobbar"
)

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

		// Rebuild FTS search index after scan
		_ = m.Library.RebuildFTSIndex()

		// Refresh navigator and album view with fresh data
		m.refreshLibraryNavigator(true)
		_ = m.Navigation.AlbumView().Refresh()

		// Show scan report popup with stats
		if msg.Stats != nil {
			m.Popups.ShowScanReport(msg.Stats)
		}

		m.ResizeComponents()
		return m, nil
	}
	return m, m.waitForLibraryScan()
}

func (m Model) waitForLibraryScan() tea.Cmd {
	return waitForChannel(m.LibraryScanCh, func(progress library.ScanProgress, ok bool) tea.Msg {
		if !ok {
			return LibraryScanCompleteMsg{}
		}
		return LibraryScanProgressMsg(progress)
	})
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
		// Rebuild FTS search index after scan
		_ = m.Library.RebuildFTSIndex()
		// Refresh views to show new/updated albums
		m.Navigation.RefreshLibrary(true)
		_ = m.Navigation.AlbumView().Refresh()
	}
	return m, nil
}

// startLibraryScan initiates a library scan with the given refresh function.
// It returns nil if a scan is already running or no sources exist.
func (m *Model) startLibraryScan(refreshFn func([]string, chan<- library.ScanProgress) error) tea.Cmd {
	if m.LibraryScanCh != nil {
		return nil
	}

	sources, err := m.Library.Sources()
	if err != nil || len(sources) == 0 {
		return nil
	}

	ch := make(chan library.ScanProgress)
	m.LibraryScanCh = ch
	go func() {
		_ = refreshFn(sources, ch)
	}()

	return m.waitForLibraryScan()
}

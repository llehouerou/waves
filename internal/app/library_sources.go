// internal/app/library_sources.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/jobbar"
)

// libraryRefreshJobID identifies the library scan job in the job bar.
const libraryRefreshJobID = "library-refresh"

// handleLibraryScanMsg shows what the core reports of a library scan: the
// core runs the scan and keeps the search index, the TUI only follows.
func (m Model) handleLibraryScanMsg(msg LibraryScanMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LibraryScanProgressMsg:
		m.showLibraryScanProgress(msg)
	case LibraryScanCompleteMsg:
		m.LibraryScanJob = nil

		// Refresh navigator and album view with fresh data
		m.refreshLibraryNavigator(true)
		_ = m.Navigation.AlbumView().Refresh()

		// Show scan report popup with stats: a failed scan has none
		if msg.Stats != nil {
			m.Popups.ShowScanReport(msg.Stats)
		}

		m.ResizeComponents()
	}
	return m, m.WatchServiceEvents()
}

func (m *Model) showLibraryScanProgress(msg LibraryScanProgressMsg) {
	switch msg.Phase {
	case "scanning":
		m.LibraryScanJob = &jobbar.Job{
			ID:      libraryRefreshJobID,
			Label:   "Scanning library",
			Current: msg.Current,
			Total:   0, // Unknown during scanning
		}
		m.ResizeComponents()
	case "processing":
		m.LibraryScanJob = &jobbar.Job{
			ID:      libraryRefreshJobID,
			Label:   "Processing files",
			Current: msg.Current,
			Total:   msg.Total,
		}
	case "cleaning":
		m.LibraryScanJob = &jobbar.Job{
			ID:      libraryRefreshJobID,
			Label:   "Cleaning up removed files",
			Current: 0,
			Total:   0,
		}
	}
}

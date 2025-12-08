// internal/app/library_sources.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/librarysources"
	"github.com/llehouerou/waves/internal/ui/scanreport"
)

func (m Model) handleLibrarySourceAdded(msg librarysources.SourceAddedMsg) (tea.Model, tea.Cmd) {
	// Check if source already exists
	exists, err := m.Library.SourceExists(msg.Path)
	if err != nil {
		m.Popups.ShowError(err.Error())
		return m, nil
	}
	if exists {
		m.Popups.ShowError("Source already exists")
		return m, nil
	}

	// Add the source to the database
	if err := m.Library.AddSource(msg.Path); err != nil {
		m.Popups.ShowError(err.Error())
		return m, nil
	}

	// Update popup with new sources list
	sources, _ := m.Library.Sources()
	m.Popups.LibrarySources().SetSources(sources)
	m.HasLibrarySources = len(sources) > 0

	// Start scanning this source
	ch := make(chan library.ScanProgress)
	m.LibraryScanCh = ch
	go func() {
		_ = m.Library.RefreshSource(msg.Path, ch)
	}()

	return m, m.waitForLibraryScan()
}

func (m Model) handleLibrarySourceRemoved(msg librarysources.SourceRemovedMsg) (tea.Model, tea.Cmd) {
	// Remove the source and its tracks
	if err := m.Library.RemoveSource(msg.Path); err != nil {
		m.Popups.ShowError(err.Error())
		return m, nil
	}

	// Update popup with new sources list
	sources, _ := m.Library.Sources()
	m.Popups.LibrarySources().SetSources(sources)
	m.HasLibrarySources = len(sources) > 0

	// Refresh the library navigator
	m.refreshLibraryNavigator(true)

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

		// Refresh navigator with fresh data
		m.refreshLibraryNavigator(true)

		// Show scan report popup with stats
		if msg.Stats != nil {
			popup := scanreport.New(msg.Stats)
			popup.SetSize(m.Layout.Width(), m.Layout.Height())
			m.Popups.ShowScanReport(popup)
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

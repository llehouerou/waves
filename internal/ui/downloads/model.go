// Package downloads provides the Downloads view for monitoring slskd downloads.
package downloads

import (
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

// Model represents the downloads view state.
type Model struct {
	ui.Base
	downloads []downloads.Download
	cursor    cursor.Cursor
	expanded  map[int64]bool // Track which downloads show file details
}

// New creates a new downloads view model.
func New() Model {
	return Model{
		cursor:   cursor.New(2), // Small scroll margin
		expanded: make(map[int64]bool),
	}
}

// SetDownloads updates the list of downloads.
func (m *Model) SetDownloads(dl []downloads.Download) {
	m.downloads = dl
	// Ensure cursor stays in bounds
	m.cursor.ClampToBounds(len(dl))
	// Clean up expanded state for deleted downloads
	m.cleanupExpandedState()
}

// Downloads returns the current list of downloads.
func (m Model) Downloads() []downloads.Download {
	return m.downloads
}

// SelectedDownload returns the currently selected download, or nil if none.
func (m Model) SelectedDownload() *downloads.Download {
	if len(m.downloads) == 0 || m.cursor.Pos() >= len(m.downloads) {
		return nil
	}
	return &m.downloads[m.cursor.Pos()]
}

// IsEmpty returns true if there are no downloads.
func (m Model) IsEmpty() bool {
	return len(m.downloads) == 0
}

// toggleExpanded toggles the expanded state of the current download.
func (m *Model) toggleExpanded() {
	if d := m.SelectedDownload(); d != nil {
		if m.expanded[d.ID] {
			delete(m.expanded, d.ID)
		} else {
			m.expanded[d.ID] = true
		}
	}
}

// isExpanded returns true if the download with given ID is expanded.
func (m Model) isExpanded(id int64) bool {
	return m.expanded[id]
}

// cleanupExpandedState removes expanded state for downloads that no longer exist.
func (m *Model) cleanupExpandedState() {
	// Build set of current download IDs
	currentIDs := make(map[int64]bool, len(m.downloads))
	for i := range m.downloads {
		currentIDs[m.downloads[i].ID] = true
	}
	// Remove expanded state for IDs not in current downloads
	for id := range m.expanded {
		if !currentIDs[id] {
			delete(m.expanded, id)
		}
	}
}

// moveCursor moves the cursor by delta and ensures it stays in bounds.
func (m *Model) moveCursor(delta int) {
	m.cursor.Move(delta, len(m.downloads), m.listHeight())
}

// listHeight returns the available height for the download list.
func (m Model) listHeight() int {
	return m.ListHeight(ui.PanelOverhead)
}

// isReadyForImport checks if a download is ready for import.
// A download is ready when all files are completed and verified on disk.
func (m Model) isReadyForImport(d *downloads.Download) bool {
	if d == nil || len(d.Files) == 0 {
		return false
	}

	for _, f := range d.Files {
		// Check if file is completed (status is lowercase "completed")
		if f.Status != downloads.StatusCompleted {
			return false
		}
		// Check if file is verified on disk
		if !f.VerifiedOnDisk {
			return false
		}
	}

	return true
}

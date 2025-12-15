// Package downloads provides the Downloads view for monitoring slskd downloads.
package downloads

import (
	"github.com/llehouerou/waves/internal/downloads"
)

// DeleteDownloadMsg requests deletion of a specific download.
type DeleteDownloadMsg struct {
	ID int64
}

// ClearCompletedMsg requests clearing all completed downloads.
type ClearCompletedMsg struct{}

// RefreshRequestMsg requests an immediate refresh from slskd.
type RefreshRequestMsg struct{}

// OpenImportMsg requests opening the import popup for a completed download.
type OpenImportMsg struct {
	Download *downloads.Download
}

// Model represents the downloads view state.
type Model struct {
	downloads []downloads.Download
	cursor    int
	offset    int
	width     int
	height    int
	focused   bool
	expanded  map[int64]bool // Track which downloads show file details
}

// New creates a new downloads view model.
func New() Model {
	return Model{
		cursor:   0,
		offset:   0,
		expanded: make(map[int64]bool),
	}
}

// SetDownloads updates the list of downloads.
func (m *Model) SetDownloads(dl []downloads.Download) {
	m.downloads = dl
	// Ensure cursor stays in bounds
	if len(dl) == 0 {
		m.cursor = 0
		m.offset = 0
	} else if m.cursor >= len(dl) {
		m.cursor = len(dl) - 1
	}
}

// Downloads returns the current list of downloads.
func (m Model) Downloads() []downloads.Download {
	return m.downloads
}

// SelectedDownload returns the currently selected download, or nil if none.
func (m Model) SelectedDownload() *downloads.Download {
	if len(m.downloads) == 0 || m.cursor >= len(m.downloads) {
		return nil
	}
	return &m.downloads[m.cursor]
}

// SetFocused sets whether the view is focused.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the view is focused.
func (m Model) IsFocused() bool {
	return m.focused
}

// SetSize sets the view dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
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

// moveCursor moves the cursor by delta and ensures it stays in bounds.
func (m *Model) moveCursor(delta int) {
	if len(m.downloads) == 0 {
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.downloads) {
		m.cursor = len(m.downloads) - 1
	}

	m.ensureCursorVisible()
}

// ensureCursorVisible adjusts offset to keep cursor visible.
func (m *Model) ensureCursorVisible() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}

	// Keep some margin around cursor
	const scrollMargin = 2

	if m.cursor < m.offset+scrollMargin {
		m.offset = max(0, m.cursor-scrollMargin)
	}
	if m.cursor >= m.offset+listHeight-scrollMargin {
		m.offset = m.cursor - listHeight + scrollMargin + 1
	}
}

// listHeight returns the available height for the download list.
func (m Model) listHeight() int {
	// Account for border (2) + header (1) + separator (1)
	return m.height - 4
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

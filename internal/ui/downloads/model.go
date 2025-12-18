// Package downloads provides the Downloads view for monitoring slskd downloads.
package downloads

import (
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/list"
)

// Model represents the downloads view state.
type Model struct {
	list     list.Model[downloads.Download]
	expanded map[int64]bool // Track which downloads show file details
}

// New creates a new downloads view model.
func New() Model {
	return Model{
		list:     list.New[downloads.Download](2), // Small scroll margin
		expanded: make(map[int64]bool),
	}
}

// SetFocused sets whether the component is focused.
func (m *Model) SetFocused(focused bool) {
	m.list.SetFocused(focused)
}

// IsFocused returns whether the component is focused.
func (m Model) IsFocused() bool {
	return m.list.IsFocused()
}

// SetSize sets the component dimensions.
func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// Width returns the component width.
func (m Model) Width() int {
	return m.list.Width()
}

// Height returns the component height.
func (m Model) Height() int {
	return m.list.Height()
}

// SetDownloads updates the list of downloads.
func (m *Model) SetDownloads(dl []downloads.Download) {
	m.list.SetItems(dl)
	// Clean up expanded state for deleted downloads
	m.cleanupExpandedState()
}

// Downloads returns the current list of downloads.
func (m Model) Downloads() []downloads.Download {
	return m.list.Items()
}

// SelectedDownload returns the currently selected download, or nil if none.
func (m Model) SelectedDownload() *downloads.Download {
	items := m.list.Items()
	pos := m.list.Cursor().Pos()
	if len(items) == 0 || pos >= len(items) {
		return nil
	}
	return &items[pos]
}

// IsEmpty returns true if there are no downloads.
func (m Model) IsEmpty() bool {
	return m.list.Len() == 0
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
	items := m.list.Items()
	currentIDs := make(map[int64]bool, len(items))
	for i := range items {
		currentIDs[items[i].ID] = true
	}
	// Remove expanded state for IDs not in current downloads
	for id := range m.expanded {
		if !currentIDs[id] {
			delete(m.expanded, id)
		}
	}
}

// listHeight returns the available height for the download list.
func (m Model) listHeight() int {
	return m.list.ListHeight(ui.PanelOverhead)
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

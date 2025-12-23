// Package downloads provides the Downloads view for monitoring slskd downloads.
package downloads

import (
	"fmt"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/list"
)

// Model represents the downloads view state.
type Model struct {
	list       list.Model[downloads.Download]
	expanded   map[int64]bool // Track which downloads show file details
	configured bool           // Whether slskd is configured
}

// New creates a new downloads view model.
func New() Model {
	return Model{
		list:     list.New[downloads.Download](2), // Small scroll margin
		expanded: make(map[int64]bool),
	}
}

// SetConfigured sets whether slskd is configured.
func (m *Model) SetConfigured(configured bool) {
	m.configured = configured
}

// IsConfigured returns whether slskd is configured.
func (m Model) IsConfigured() bool {
	return m.configured
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
	return m.importBlockedReason(d) == ""
}

// importBlockedReason returns the reason why import is blocked, or empty string if ready.
func (m Model) importBlockedReason(d *downloads.Download) string {
	if d == nil {
		return "No download selected"
	}
	if len(d.Files) == 0 {
		return "Download has no files"
	}

	var pending, downloading, failed, completed, verified int
	for _, f := range d.Files {
		switch f.Status {
		case downloads.StatusPending:
			pending++
		case downloads.StatusDownloading:
			downloading++
		case downloads.StatusFailed:
			failed++
		case downloads.StatusCompleted:
			completed++
			if f.VerifiedOnDisk {
				verified++
			}
		}
	}

	total := len(d.Files)
	switch {
	case downloading > 0:
		return fmt.Sprintf("Still downloading (%d/%d completed)", completed, total)
	case pending > 0:
		return fmt.Sprintf("Waiting to start (%d/%d completed)", completed, total)
	case failed > 0:
		return fmt.Sprintf("%d/%d files failed to download", failed, total)
	case verified < completed:
		return fmt.Sprintf("Verifying files (%d/%d verified)", verified, total)
	}

	return ""
}

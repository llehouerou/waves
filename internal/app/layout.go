// internal/app/layout.go
package app

import (
	"github.com/llehouerou/waves/internal/ui/headerbar"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// ContentHeight returns the total available height for the main content area
// (navigator + queue). This is the terminal height minus header, player bar, and job bar.
func (m *Model) ContentHeight() int {
	height := m.Layout.Height()
	height -= headerbar.Height
	if !m.Playback.IsStopped() {
		height -= playerbar.Height(m.Playback.DisplayMode())
	}
	if m.HasActiveJobs() {
		height -= jobbar.Height
	}
	return height
}

// NavigatorHeight returns the available height for navigators.
// In narrow mode with queue visible, returns 2/3 of content height.
// Otherwise returns full content height.
func (m *Model) NavigatorHeight() int {
	contentHeight := m.ContentHeight()
	if m.Layout.IsNarrowMode() && m.Layout.IsQueueVisible() {
		return contentHeight * 2 / 3
	}
	return contentHeight
}

// QueueHeight returns the available height for the queue panel.
// In narrow mode, returns 1/3 of content height.
// Otherwise returns same as navigator height (side by side layout).
func (m *Model) QueueHeight() int {
	contentHeight := m.ContentHeight()
	if m.Layout.IsNarrowMode() {
		return contentHeight - m.NavigatorHeight()
	}
	return m.NavigatorHeight()
}

// HasActiveJobs returns true if there are active background jobs.
func (m *Model) HasActiveJobs() bool {
	return m.LibraryScanJob != nil && !m.LibraryScanJob.Done
}

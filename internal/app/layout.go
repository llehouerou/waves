// internal/app/layout.go
package app

import (
	"github.com/llehouerou/waves/internal/ui/headerbar"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// NotificationBorderHeight is the height of borders around notifications.
const NotificationBorderHeight = 2

// NotificationHeight returns the height for the given number of notifications.
func NotificationHeight(count int) int {
	if count == 0 {
		return 0
	}
	return count + NotificationBorderHeight
}

// ContentHeight returns the total available height for the main content area
// (navigator + queue). This is the terminal height minus header, player bar, job bar, and notification.
func (m *Model) ContentHeight() int {
	height := m.Layout.Height()
	height -= headerbar.Height
	if !m.PlaybackService.IsStopped() {
		height -= playerbar.Height(m.Layout.PlayerDisplayMode())
	}
	if activeCount := m.ActiveJobCount(); activeCount > 0 {
		height -= jobbar.Height(activeCount)
	}
	if len(m.Notifications) > 0 {
		height -= NotificationHeight(len(m.Notifications))
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
	return m.ActiveJobCount() > 0
}

// ActiveJobCount returns the number of active background jobs.
func (m *Model) ActiveJobCount() int {
	count := 0
	if m.LibraryScanJob != nil && !m.LibraryScanJob.Done {
		count++
	}
	for _, job := range m.ExportJobs {
		if !job.JobBar().Done {
			count++
		}
	}
	return count
}

// PlayerBarRow returns the 1-based row number where the player bar starts.
// Returns 0 if player is stopped.
func (m *Model) PlayerBarRow() int {
	if m.PlaybackService.IsStopped() {
		return 0
	}

	// Player bar is at the bottom, before job bar and notifications
	row := m.Layout.Height()

	// Subtract notifications
	if len(m.Notifications) > 0 {
		row -= NotificationHeight(len(m.Notifications))
	}

	// Subtract job bar
	if activeCount := m.ActiveJobCount(); activeCount > 0 {
		row -= jobbar.Height(activeCount)
	}

	// Subtract player bar height to get the starting row
	row -= playerbar.Height(m.Layout.PlayerDisplayMode())

	// Convert to 1-based row number
	return row + 1
}

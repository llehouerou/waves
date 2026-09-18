// internal/app/layout.go
package app

import (
	"github.com/llehouerou/waves/internal/ui/headerbar"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/layout"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// ContentHeight returns the total available height for the main content area
// (navigator + queue). This is the terminal height minus header, player bar, job bar, and notification.
func (m *Model) ContentHeight() int {
	opts := layout.ContentOpts{
		HeaderHeight:      headerbar.Height,
		NotificationCount: len(m.Notifications),
	}
	if !m.PlaybackService.IsStopped() {
		opts.PlayerBarHeight = playerbar.Height(m.Layout.PlayerDisplayMode())
	}
	if activeCount := m.ActiveJobCount(); activeCount > 0 {
		opts.JobBarHeight = jobbar.Height(activeCount)
	}
	return layout.ContentHeight(m.Layout.Height(), opts)
}

// NavigatorHeight returns the available height for navigators.
// In narrow mode with queue visible, returns 2/3 of content height.
// Otherwise returns full content height.
func (m *Model) NavigatorHeight() int {
	return layout.NavigatorHeight(
		m.ContentHeight(),
		m.Layout.IsNarrowMode(),
		m.Layout.IsQueueVisible(),
	)
}

// QueueHeight returns the available height for the queue panel.
// In narrow mode, returns 1/3 of content height.
// Otherwise returns same as navigator height (side by side layout).
func (m *Model) QueueHeight() int {
	return layout.QueueHeight(
		m.ContentHeight(),
		m.Layout.IsNarrowMode(),
		m.Layout.IsQueueVisible(),
	)
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

// playerBarHeight returns the current player bar height (0 if stopped).
func (m *Model) playerBarHeight() int {
	if m.PlaybackService.IsStopped() {
		return 0
	}
	return playerbar.Height(m.Layout.PlayerDisplayMode())
}

// PlayerBarRow returns the 1-based row number where the player bar starts.
// Returns 0 if player is stopped.
func (m *Model) PlayerBarRow() int {
	var playerBarHeight int
	if !m.PlaybackService.IsStopped() {
		playerBarHeight = playerbar.Height(m.Layout.PlayerDisplayMode())
	}

	var jobBarHeight int
	if activeCount := m.ActiveJobCount(); activeCount > 0 {
		jobBarHeight = jobbar.Height(activeCount)
	}

	return layout.PlayerBarRow(
		m.Layout.Height(),
		playerBarHeight,
		jobBarHeight,
		len(m.Notifications),
	)
}

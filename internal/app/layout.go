// internal/app/layout.go
package app

import (
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// NavigatorHeight returns the available height for navigators.
func (m *Model) NavigatorHeight() int {
	height := m.Height
	if m.Player.State() != player.Stopped {
		height -= playerbar.Height(m.PlayerDisplayMode)
	}
	if m.HasActiveJobs() {
		height -= jobbar.Height
	}
	return height
}

// HasActiveJobs returns true if there are active background jobs.
func (m *Model) HasActiveJobs() bool {
	return m.LibraryScanJob != nil && !m.LibraryScanJob.Done
}

// NavigatorWidth returns the available width for navigators.
func (m *Model) NavigatorWidth() int {
	if m.QueueVisible {
		return m.Width * 2 / 3
	}
	return m.Width
}

// QueueWidth returns the width for the queue panel.
func (m *Model) QueueWidth() int {
	return m.Width - m.NavigatorWidth()
}

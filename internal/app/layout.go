// internal/app/layout.go
package app

import (
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// NavigatorHeight returns the available height for navigators.
// This depends on player state and active jobs, so it remains on Model.
func (m *Model) NavigatorHeight() int {
	height := m.Layout.Height()
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

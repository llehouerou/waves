// internal/app/layout.go
package app

import (
	"github.com/llehouerou/waves/internal/ui/headerbar"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// NavigatorHeight returns the available height for navigators.
// This depends on player state and active jobs, so it remains on Model.
func (m *Model) NavigatorHeight() int {
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

// HasActiveJobs returns true if there are active background jobs.
func (m *Model) HasActiveJobs() bool {
	return m.LibraryScanJob != nil && !m.LibraryScanJob.Done
}

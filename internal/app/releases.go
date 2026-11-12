// internal/app/releases.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/download"
)

// showDownloadPopup opens the download popup with the configured filters.
func (m Model) showDownloadPopup() tea.Cmd {
	filters := download.FilterConfig{
		Format:     m.Slskd.Filters.Format,
		NoSlot:     m.Slskd.Filters.NoSlot,
		TrackCount: m.Slskd.Filters.TrackCount,
		AlbumsOnly: m.MusicBrainz.AlbumsOnly,
	}
	return m.Popups.ShowDownload(m.Core.Downloads(), m.Core.MusicBrainz(), filters, m.Core.Library())
}

// openNewReleases opens the download popup on the new releases list. Opening
// it is what asks the core to keep it fresh: that work stays out of the job
// bar, it is the list's own business, said with one discreet line inside the
// popup. No slskd gate either: it is Enter that refuses when slskd is
// unconfigured.
func (m Model) openNewReleases() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.showDownloadPopup()}
	if dl := m.Popups.Download(); dl != nil {
		cmds = append(cmds, dl.StartReleases(m.Core.Releases()))
	}
	return m, tea.Batch(cmds...)
}

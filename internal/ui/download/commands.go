package download

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/request"
)

// queueDownload queues a download on slskd and records it.
func queueDownload(dls Downloads, d core.Download) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			_, err := dls.Queue(d)
			return SlskdDownloadQueuedMsg{Request: id, Artist: d.MBArtistName, Album: d.MBAlbumTitle, Err: err}
		}
	}
}

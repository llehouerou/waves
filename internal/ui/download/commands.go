package download

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
)

// queueDownloadCmd queues a download on slskd and records it.
func queueDownloadCmd(dls Downloads, d core.Download) tea.Cmd {
	return func() tea.Msg {
		_, err := dls.Queue(d)
		return SlskdDownloadQueuedMsg{Err: err}
	}
}

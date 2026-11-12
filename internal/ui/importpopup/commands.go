package importpopup

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
)

// PreparedMsg is the plan of an import, or why the core could make none.
type PreparedMsg struct {
	DownloadID int64 // the popup that asked; others ignore it
	Plan       core.ImportPlan
	Err        error
}

// prepareCmd asks the core for the plan of an import, off the UI goroutine:
// it reads the files and waits for MusicBrainz.
func prepareCmd(imp Importer, downloadID int64) tea.Cmd {
	return func() tea.Msg {
		plan, err := imp.PrepareImport(downloadID)
		return PreparedMsg{DownloadID: downloadID, Plan: plan, Err: err}
	}
}

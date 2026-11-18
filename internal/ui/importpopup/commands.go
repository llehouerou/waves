package importpopup

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/request"
)

// PreparedMsg is the plan of an import, or why the core could make none.
type PreparedMsg struct {
	Request request.ID
	Plan    core.ImportPlan
	Err     error
}

// prepare asks the core for the plan of an import, off the UI goroutine: it
// reads the files and waits for MusicBrainz.
func prepare(imp Importer, downloadID int64) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			plan, err := imp.PrepareImport(downloadID)
			return PreparedMsg{Request: id, Plan: plan, Err: err}
		}
	}
}

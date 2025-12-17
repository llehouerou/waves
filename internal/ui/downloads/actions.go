package downloads

import (
	dl "github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/ui/action"
)

// DeleteDownload requests deletion of a download.
type DeleteDownload struct {
	ID int64
}

// ActionType implements action.Action.
func (a DeleteDownload) ActionType() string { return "downloads.delete" }

// ClearCompleted requests clearing all completed downloads.
type ClearCompleted struct{}

// ActionType implements action.Action.
func (a ClearCompleted) ActionType() string { return "downloads.clear_completed" }

// RefreshRequest requests refreshing download status from slskd.
type RefreshRequest struct{}

// ActionType implements action.Action.
func (a RefreshRequest) ActionType() string { return "downloads.refresh" }

// OpenImport requests opening the import popup for a download.
type OpenImport struct {
	Download *dl.Download
}

// ActionType implements action.Action.
func (a OpenImport) ActionType() string { return "downloads.open_import" }

// ActionMsg creates an action.Msg for a downloads view action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "downloads", Action: a}
}

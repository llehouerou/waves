package librarysources

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// SourceAdded signals a new library source was added.
type SourceAdded struct {
	Path string
}

// ActionType implements action.Action.
func (a SourceAdded) ActionType() string { return "librarysources.source_added" }

// SourceRemoved signals a library source was removed.
type SourceRemoved struct {
	Path string
}

// ActionType implements action.Action.
func (a SourceRemoved) ActionType() string { return "librarysources.source_removed" }

// RequestTrackCount requests the track count for a source path.
type RequestTrackCount struct {
	Path string
}

// ActionType implements action.Action.
func (a RequestTrackCount) ActionType() string { return "librarysources.request_track_count" }

// Close signals the popup should close.
type Close struct{}

// ActionType implements action.Action.
func (a Close) ActionType() string { return "librarysources.close" }

// ActionMsg creates an action.Msg for a librarysources action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "librarysources", Action: a}
}

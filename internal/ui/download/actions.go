package download

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Close signals the download popup should close.
type Close struct{}

// ActionType implements action.Action.
func (a Close) ActionType() string { return "download.close" }

// Queued signals a download was queued on slskd and recorded.
type Queued struct{}

// ActionType implements action.Action.
func (a Queued) ActionType() string { return "download.queued" }

// ActionMsg creates an action.Msg for a download popup action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "download", Action: a}
}

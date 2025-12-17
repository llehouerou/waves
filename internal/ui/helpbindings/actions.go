package helpbindings

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Close signals the help popup should close.
type Close struct{}

// ActionType implements action.Action.
func (a Close) ActionType() string { return "helpbindings.close" }

// ActionMsg creates an action.Msg for a helpbindings action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "helpbindings", Action: a}
}

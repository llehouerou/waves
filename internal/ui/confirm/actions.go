package confirm

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Result contains the confirmation dialog result.
type Result struct {
	Confirmed      bool
	Context        any // User-provided context passed through
	SelectedOption int // Index of selected option (for multi-option mode)
}

// ActionType implements action.Action.
func (a Result) ActionType() string { return "confirm.result" }

// ActionMsg creates an action.Msg for a confirm action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "confirm", Action: a}
}

package textinput

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Result contains the text input result.
type Result struct {
	Text     string
	Context  any  // User-provided context passed through
	Canceled bool // True if user pressed Escape
}

// ActionType implements action.Action.
func (a Result) ActionType() string { return "textinput.result" }

// ActionMsg creates an action.Msg for a textinput action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "textinput", Action: a}
}

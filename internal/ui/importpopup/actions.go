package importpopup

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Close signals the import popup should close. An import the core has started
// goes on without it.
type Close struct{}

// ActionType implements action.Action.
func (a Close) ActionType() string { return "import.close" }

// ActionMsg creates an action.Msg for an import popup action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "import", Action: a}
}

package search

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Result contains the search result (selected item or cancellation).
type Result struct {
	Item     Item // The selected item (nil if canceled)
	Canceled bool // True if user pressed Escape
}

// ActionType implements action.Action.
func (a Result) ActionType() string { return "search.result" }

// ActionMsg creates an action.Msg for a search action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "search", Action: a}
}

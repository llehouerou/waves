package navigator

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// NavigationChanged signals that the navigation path or selection has changed.
type NavigationChanged struct {
	CurrentPath  string // The current directory path
	SelectedName string // The name of the selected item
}

// ActionType implements action.Action.
func (a NavigationChanged) ActionType() string { return "navigator.navigation_changed" }

// NavigationFailed signals that a move was refused: its destination could not
// be listed, Err says why, and the navigator stayed where it was.
type NavigationFailed struct {
	Err error
}

// ActionType implements action.Action.
func (a NavigationFailed) ActionType() string { return "navigator.navigation_failed" }

// ActionMsg creates an action.Msg for a navigator action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "navigator", Action: a}
}

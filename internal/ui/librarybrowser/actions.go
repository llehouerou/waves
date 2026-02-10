package librarybrowser

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// NavigationChanged is emitted when the selection changes.
type NavigationChanged struct{}

// ActionType implements action.Action.
func (NavigationChanged) ActionType() string { return "librarybrowser.navigation_changed" }

// ActionMsg creates an action.Msg for a librarybrowser action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "librarybrowser", Action: a}
}

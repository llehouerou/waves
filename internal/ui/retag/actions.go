package retag

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/action"
)

// ActionMsg wraps an action with the source identifier.
func ActionMsg(a action.Action) tea.Msg {
	return action.Msg{
		Source: "retag",
		Action: a,
	}
}

// Close signals that the popup should be closed. A retag the core has started
// goes on without it.
type Close struct{}

func (Close) ActionType() string { return "retag.Close" }

// Complete signals that the retag the popup followed is over and that the
// popup is done with it: close it, and show the album.
type Complete struct {
	AlbumArtist string
	AlbumName   string
}

func (Complete) ActionType() string { return "retag.Complete" }

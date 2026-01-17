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

// Close signals that the popup should be closed.
type Close struct{}

func (Close) ActionType() string { return "retag.Close" }

// Complete signals that retagging finished (may have errors).
type Complete struct {
	AlbumArtist  string
	AlbumName    string
	SuccessCount int
	FailedCount  int
}

func (Complete) ActionType() string { return "retag.Complete" }

// RequestStart signals that user wants to start retagging.
// The app should stop playback if any track is currently playing.
type RequestStart struct {
	TrackPaths []string
}

func (RequestStart) ActionType() string { return "retag.RequestStart" }

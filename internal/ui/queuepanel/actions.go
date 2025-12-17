package queuepanel

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// JumpToTrack requests playback to jump to a specific queue index.
type JumpToTrack struct {
	Index int
}

// ActionType implements action.Action.
func (a JumpToTrack) ActionType() string { return "queuepanel.jump_to_track" }

// QueueChanged signals that the queue contents have changed and need persisting.
type QueueChanged struct{}

// ActionType implements action.Action.
func (a QueueChanged) ActionType() string { return "queuepanel.queue_changed" }

// ToggleFavorite requests toggling favorite status for tracks.
type ToggleFavorite struct {
	TrackIDs []int64
}

// ActionType implements action.Action.
func (a ToggleFavorite) ActionType() string { return "queuepanel.toggle_favorite" }

// ActionMsg creates an action.Msg for a queuepanel action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "queuepanel", Action: a}
}

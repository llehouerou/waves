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

// AddToPlaylist requests adding tracks to a playlist.
type AddToPlaylist struct {
	TrackIDs []int64
}

// ActionType implements action.Action.
func (a AddToPlaylist) ActionType() string { return "queuepanel.add_to_playlist" }

// GoToSource requests navigation to the track's source in the current view.
type GoToSource struct {
	TrackID int64  // Library track ID (0 if from filesystem)
	Path    string // File path
	Album   string // Album name
	Artist  string // Artist name
}

// ActionType implements action.Action.
func (a GoToSource) ActionType() string { return "queuepanel.go_to_source" }

// ActionMsg creates an action.Msg for a queuepanel action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "queuepanel", Action: a}
}

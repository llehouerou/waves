package albumview

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// QueueAlbum requests queueing an album (replace or add mode).
type QueueAlbum struct {
	AlbumArtist string
	Album       string
	Replace     bool // true = replace queue, false = add
}

// ActionType implements action.Action.
func (a QueueAlbum) ActionType() string { return "albumview.queue_album" }

// ActionMsg creates an action.Msg for an albumview action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "albumview", Action: a}
}

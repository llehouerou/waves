package download

import (
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/ui/action"
)

// Close signals the download popup should close.
type Close struct{}

// ActionType implements action.Action.
func (a Close) ActionType() string { return "download.close" }

// QueuedData contains the data for a successfully queued download.
type QueuedData struct {
	MBReleaseGroupID string
	MBReleaseID      string // Specific release selected for import
	MBArtistName     string
	MBAlbumTitle     string
	MBReleaseYear    string
	SlskdUsername    string
	SlskdDirectory   string
	Files            []FileInfo
	// Full MusicBrainz data for importing
	MBReleaseGroup   *musicbrainz.ReleaseGroup   // Release group metadata
	MBReleaseDetails *musicbrainz.ReleaseDetails // Full release with tracks
}

// ActionType implements action.Action.
func (a QueuedData) ActionType() string { return "download.queued_data" }

// ActionMsg creates an action.Msg for a download popup action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "download", Action: a}
}

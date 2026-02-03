package retag

import (
	"github.com/llehouerou/waves/internal/tags"
)

// TagsReadMsg is sent when tags have been read from album files.
type TagsReadMsg struct {
	Tags             []tags.FileInfo
	MBReleaseID      string // Extracted from first file with MB release ID
	MBReleaseGroupID string // Extracted from first file with MB release group ID
	MBArtistID       string // Extracted from first file with MB artist ID
	Err              error
}

// FileRetaggedMsg is sent when a single file has been retagged.
type FileRetaggedMsg struct {
	Index int
	Err   error
}

// LibraryUpdatedMsg is sent when library tracks have been updated after retagging.
type LibraryUpdatedMsg struct {
	Err error
}

// StartApprovedMsg is sent by the app when playback has been stopped (if needed)
// and the retag can proceed.
type StartApprovedMsg struct{}

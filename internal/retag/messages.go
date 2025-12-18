package retag

import (
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
)

// TagsReadMsg is sent when tags have been read from album files.
type TagsReadMsg struct {
	Tags             []player.TrackInfo
	MBReleaseID      string // Extracted from first file with MB release ID
	MBReleaseGroupID string // Extracted from first file with MB release group ID
	MBArtistID       string // Extracted from first file with MB artist ID
	Err              error
}

// ReleaseGroupSearchResultMsg is sent when release group search completes.
type ReleaseGroupSearchResultMsg struct {
	ReleaseGroups []musicbrainz.ReleaseGroup
	Err           error
}

// ReleasesFetchedMsg is sent when releases for a release group are fetched.
type ReleasesFetchedMsg struct {
	Releases []musicbrainz.Release
	Err      error
}

// ReleaseDetailsFetchedMsg is sent when full release details are fetched.
type ReleaseDetailsFetchedMsg struct {
	Release *musicbrainz.ReleaseDetails
	Err     error
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

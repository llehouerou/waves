package download

import (
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/slskd"
)

// ArtistSearchResultMsg is sent when artist search completes.
type ArtistSearchResultMsg struct {
	Artists []musicbrainz.Artist
	Err     error
}

// ReleaseGroupResultMsg is sent when release groups are loaded.
type ReleaseGroupResultMsg struct {
	ReleaseGroups []musicbrainz.ReleaseGroup
	Err           error
}

// ReleaseResultMsg is sent when releases for a release group are loaded.
type ReleaseResultMsg struct {
	Releases []musicbrainz.Release
	Err      error
}

// ReleaseDetailsResultMsg is sent when full release details (with tracks) are loaded.
type ReleaseDetailsResultMsg struct {
	Details *musicbrainz.ReleaseDetails
	Err     error
}

// SlskdSearchStartedMsg is sent when slskd search is initiated.
type SlskdSearchStartedMsg struct {
	SearchID string
	Err      error
}

// SlskdSearchResultMsg is sent when slskd search results are ready.
type SlskdSearchResultMsg struct {
	RawResponses []slskd.SearchResponse // Raw responses for re-filtering
	Err          error
}

// SlskdSearchPollMsg triggers polling for slskd search status.
type SlskdSearchPollMsg struct {
	SearchID      string
	State         string // Search state: InProgress, Completed, etc.
	ResponseCount int
	StablePolls   int // Number of polls where response count hasn't changed
	FetchRetries  int // Number of times we've tried fetching responses after completion
	TotalPolls    int // Total number of polls since search started (for timeout)
}

// SlskdDownloadQueuedMsg is sent when download is queued.
type SlskdDownloadQueuedMsg struct {
	Err error
}

// QueuedDataMsg is sent after successful download queue to persist data.
type QueuedDataMsg struct {
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

// FileInfo contains info about a file in a queued download.
type FileInfo struct {
	Filename string
	Size     int64
}

// CloseMsg is sent when the download popup should be closed.
type CloseMsg struct{}

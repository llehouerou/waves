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
}

// SlskdDownloadQueuedMsg is sent when download is queued.
type SlskdDownloadQueuedMsg struct {
	Err error
}

// CloseMsg is sent when the download popup should be closed.
type CloseMsg struct{}

// Package mbworkflow provides the MusicBrainz requests of the popups as
// Bubble Tea commands.
package mbworkflow

import (
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/request"
)

// Client defines the MusicBrainz client interface used by the workflow.
// This interface allows for easy mocking in tests.
type Client interface {
	SearchArtists(query string) ([]core.MBArtist, error)
	SearchReleaseGroups(query string) ([]core.MBReleaseGroup, error)
	GetArtistReleaseGroups(artistID string) (core.MBBrowsed[core.MBReleaseGroup], error)
	GetReleaseGroupReleases(releaseGroupID string) (core.MBBrowsed[core.MBRelease], error)
	GetRelease(mbid string) (*core.MBReleaseDetails, error)
	GetCoverArt(releaseMBID string) ([]byte, error)
}

// Message types returned by the requests of request.go. Each carries the ID
// of the request it answers.

// ArtistSearchResultMsg is returned when an artist search completes.
type ArtistSearchResultMsg struct {
	Request request.ID
	Artists []core.MBArtist
	Err     error
}

// SearchResultMsg is returned when release groups are loaded: found by a
// search, or those of an artist. Cut says the release groups of an artist
// MusicBrainz has more of than were read; a search is never cut.
type SearchResultMsg struct {
	Request       request.ID
	ReleaseGroups []core.MBReleaseGroup
	Cut           Cut
	Err           error
}

// ReleasesResultMsg is returned when releases are loaded. Cut says a release
// group MusicBrainz has more releases of than were read.
type ReleasesResultMsg struct {
	Request  request.ID
	Releases []core.MBRelease
	Cut      Cut
	Err      error
}

// ReleaseDetailsResultMsg is returned when release details are loaded.
type ReleaseDetailsResultMsg struct {
	Request request.ID
	Details *core.MBReleaseDetails
	Err     error
}

// CoverArtResultMsg is returned when cover art is fetched: the front cover of
// the release asked, none if it has none or if the fetch failed.
type CoverArtResultMsg struct {
	Request request.ID
	Data    []byte
	Err     error
}

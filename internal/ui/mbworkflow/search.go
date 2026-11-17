// Package mbworkflow provides the MusicBrainz requests of the popups as
// Bubble Tea commands.
package mbworkflow

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
)

// Client defines the MusicBrainz client interface used by the workflow.
// This interface allows for easy mocking in tests.
type Client interface {
	SearchArtists(query string) ([]core.MBArtist, error)
	SearchReleaseGroups(query string) ([]core.MBReleaseGroup, error)
	GetArtistReleaseGroups(artistID string) ([]core.MBReleaseGroup, error)
	GetReleaseGroupReleases(releaseGroupID string) ([]core.MBRelease, error)
	GetRelease(mbid string) (*core.MBReleaseDetails, error)
	GetCoverArt(releaseMBID string) ([]byte, error)
}

// Message types returned by the commands of Pending.Ask. Each names the
// request it answers.

// ArtistSearchResultMsg is returned when an artist search completes.
type ArtistSearchResultMsg struct {
	Request Request
	Artists []core.MBArtist
	Err     error
}

// SearchResultMsg is returned when release groups are loaded: found by a
// search, or those of an artist.
type SearchResultMsg struct {
	Request       Request
	ReleaseGroups []core.MBReleaseGroup
	Err           error
}

// ReleasesResultMsg is returned when releases are loaded.
type ReleasesResultMsg struct {
	Request  Request
	Releases []core.MBRelease
	Err      error
}

// ReleaseDetailsResultMsg is returned when release details are loaded.
type ReleaseDetailsResultMsg struct {
	Request Request
	Details *core.MBReleaseDetails
	Err     error
}

// CoverArtResultMsg is returned when cover art is fetched: the front cover of
// the release ReleaseID, none if it has none or if the fetch failed.
type CoverArtResultMsg struct {
	ReleaseID string
	Data      []byte
	Err       error
}

// FetchCoverArtCmd fetches cover art for a release.
func FetchCoverArtCmd(client Client, releaseMBID string) tea.Cmd {
	return func() tea.Msg {
		data, err := client.GetCoverArt(releaseMBID)
		return CoverArtResultMsg{ReleaseID: releaseMBID, Data: data, Err: err}
	}
}

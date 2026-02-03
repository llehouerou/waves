// Package workflow provides reusable MusicBrainz search workflows.
package workflow

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

// Client defines the MusicBrainz client interface used by the workflow.
// This interface allows for easy mocking in tests.
type Client interface {
	SearchArtists(query string) ([]musicbrainz.Artist, error)
	SearchReleaseGroups(query string) ([]musicbrainz.ReleaseGroup, error)
	SearchReleaseGroupsByArtistAlbum(artist, album string) ([]musicbrainz.ReleaseGroup, error)
	GetArtistReleaseGroups(artistID string) ([]musicbrainz.ReleaseGroup, error)
	GetReleaseGroupReleases(releaseGroupID string) ([]musicbrainz.Release, error)
	GetRelease(mbid string) (*musicbrainz.ReleaseDetails, error)
	GetCoverArt(releaseMBID string) ([]byte, error)
}

// Message types returned by workflow commands.

// ArtistSearchResultMsg is returned when an artist search completes.
type ArtistSearchResultMsg struct {
	Artists []musicbrainz.Artist
	Err     error
}

// SearchResultMsg is returned when a release group search completes.
type SearchResultMsg struct {
	ReleaseGroups []musicbrainz.ReleaseGroup
	Err           error
}

// ReleasesResultMsg is returned when releases are loaded.
type ReleasesResultMsg struct {
	Releases []musicbrainz.Release
	Err      error
}

// ReleaseDetailsResultMsg is returned when release details are loaded.
type ReleaseDetailsResultMsg struct {
	Details *musicbrainz.ReleaseDetails
	Err     error
}

// CoverArtResultMsg is returned when cover art is fetched.
type CoverArtResultMsg struct {
	Data []byte
	Err  error
}

// Command functions for MusicBrainz API operations.

// SearchArtistsCmd searches for artists using a free-form query.
func SearchArtistsCmd(client Client, query string) tea.Cmd {
	return func() tea.Msg {
		artists, err := client.SearchArtists(query)
		return ArtistSearchResultMsg{Artists: artists, Err: err}
	}
}

// SearchCmd searches for release groups using a free-form query.
func SearchCmd(client Client, query string) tea.Cmd {
	return func() tea.Msg {
		groups, err := client.SearchReleaseGroups(query)
		return SearchResultMsg{ReleaseGroups: groups, Err: err}
	}
}

// SearchByArtistAlbumCmd searches using field-specific Lucene syntax.
func SearchByArtistAlbumCmd(client Client, artist, album string) tea.Cmd {
	return func() tea.Msg {
		groups, err := client.SearchReleaseGroupsByArtistAlbum(artist, album)
		return SearchResultMsg{ReleaseGroups: groups, Err: err}
	}
}

// FetchArtistReleaseGroupsCmd fetches all release groups for an artist ID.
func FetchArtistReleaseGroupsCmd(client Client, artistID string) tea.Cmd {
	return func() tea.Msg {
		groups, err := client.GetArtistReleaseGroups(artistID)
		return SearchResultMsg{ReleaseGroups: groups, Err: err}
	}
}

// FetchReleasesCmd fetches all releases for a release group.
func FetchReleasesCmd(client Client, releaseGroupID string) tea.Cmd {
	return func() tea.Msg {
		releases, err := client.GetReleaseGroupReleases(releaseGroupID)
		return ReleasesResultMsg{Releases: releases, Err: err}
	}
}

// FetchReleaseDetailsCmd fetches full details for a release.
func FetchReleaseDetailsCmd(client Client, releaseID string) tea.Cmd {
	return func() tea.Msg {
		details, err := client.GetRelease(releaseID)
		return ReleaseDetailsResultMsg{Details: details, Err: err}
	}
}

// FetchCoverArtCmd fetches cover art for a release.
func FetchCoverArtCmd(client Client, releaseMBID string) tea.Cmd {
	return func() tea.Msg {
		data, err := client.GetCoverArt(releaseMBID)
		return CoverArtResultMsg{Data: data, Err: err}
	}
}

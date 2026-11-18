package mbworkflow

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/request"
)

// The requests a popup sends through a request.Pending: each returns the
// command that asks client, whose answer carries the ID of the request.

// SearchArtists asks for the artists a query finds: ArtistSearchResultMsg.
func SearchArtists(client Client, query string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			artists, err := client.SearchArtists(query)
			return ArtistSearchResultMsg{Request: id, Artists: artists, Err: err}
		}
	}
}

// SearchReleaseGroups asks for the release groups a query finds:
// SearchResultMsg.
func SearchReleaseGroups(client Client, query string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			groups, err := client.SearchReleaseGroups(query)
			return SearchResultMsg{Request: id, ReleaseGroups: groups, Err: err}
		}
	}
}

// ArtistReleaseGroups asks for the release groups of an artist:
// SearchResultMsg.
func ArtistReleaseGroups(client Client, artistID string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			groups, err := client.GetArtistReleaseGroups(artistID)
			return SearchResultMsg{Request: id, ReleaseGroups: groups, Err: err}
		}
	}
}

// ReleaseGroupReleases asks for the releases of a release group:
// ReleasesResultMsg.
func ReleaseGroupReleases(client Client, releaseGroupID string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			releases, err := client.GetReleaseGroupReleases(releaseGroupID)
			return ReleasesResultMsg{Request: id, Releases: releases, Err: err}
		}
	}
}

// ReleaseDetails asks for the details of a release: ReleaseDetailsResultMsg.
func ReleaseDetails(client Client, releaseID string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			details, err := client.GetRelease(releaseID)
			return ReleaseDetailsResultMsg{Request: id, Details: details, Err: err}
		}
	}
}

// CoverArt asks for the front cover of a release: CoverArtResultMsg.
func CoverArt(client Client, releaseID string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			data, err := client.GetCoverArt(releaseID)
			return CoverArtResultMsg{Request: id, Data: data, Err: err}
		}
	}
}

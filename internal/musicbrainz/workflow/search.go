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

// SearchFlow manages the MusicBrainz search workflow.
// It encapsulates the common pattern of searching for release groups,
// selecting one, fetching its releases, and selecting a final release.
type SearchFlow struct {
	client        Client
	query         string
	releaseGroups []musicbrainz.ReleaseGroup
	releases      []musicbrainz.Release
	selected      struct {
		releaseGroup *musicbrainz.ReleaseGroup
		release      *musicbrainz.Release
	}
}

// NewSearchFlow creates a new search workflow.
func NewSearchFlow(client Client) *SearchFlow {
	return &SearchFlow{
		client: client,
	}
}

// Query returns the current search query.
func (f *SearchFlow) Query() string {
	return f.query
}

// ReleaseGroups returns the current release group results.
func (f *SearchFlow) ReleaseGroups() []musicbrainz.ReleaseGroup {
	return f.releaseGroups
}

// Releases returns the current release results.
func (f *SearchFlow) Releases() []musicbrainz.Release {
	return f.releases
}

// SetReleaseGroups sets the release groups (useful when results come from external source).
func (f *SearchFlow) SetReleaseGroups(groups []musicbrainz.ReleaseGroup) {
	f.releaseGroups = groups
}

// SetReleases sets the releases (useful when results come from external source).
func (f *SearchFlow) SetReleases(releases []musicbrainz.Release) {
	f.releases = releases
}

// Search initiates a free-form release group search.
func (f *SearchFlow) Search(query string) tea.Cmd {
	f.query = query
	return SearchCmd(f.client, query)
}

// SearchByArtistAlbum initiates a field-specific search for better accuracy.
func (f *SearchFlow) SearchByArtistAlbum(artist, album string) tea.Cmd {
	f.query = artist + " - " + album
	return SearchByArtistAlbumCmd(f.client, artist, album)
}

// FetchArtistReleaseGroups fetches all release groups for an artist.
func (f *SearchFlow) FetchArtistReleaseGroups(artistID string) tea.Cmd {
	return FetchArtistReleaseGroupsCmd(f.client, artistID)
}

// SelectReleaseGroup sets the selected release group by index.
func (f *SearchFlow) SelectReleaseGroup(idx int) {
	if idx >= 0 && idx < len(f.releaseGroups) {
		f.selected.releaseGroup = &f.releaseGroups[idx]
	}
}

// SelectedReleaseGroup returns the currently selected release group.
func (f *SearchFlow) SelectedReleaseGroup() *musicbrainz.ReleaseGroup {
	return f.selected.releaseGroup
}

// FetchReleases loads releases for the selected release group.
func (f *SearchFlow) FetchReleases() tea.Cmd {
	if f.selected.releaseGroup == nil {
		return nil
	}
	return FetchReleasesCmd(f.client, f.selected.releaseGroup.ID)
}

// SelectRelease sets the final release selection by index.
func (f *SearchFlow) SelectRelease(idx int) {
	if idx >= 0 && idx < len(f.releases) {
		f.selected.release = &f.releases[idx]
	}
}

// SelectedRelease returns the currently selected release.
func (f *SearchFlow) SelectedRelease() *musicbrainz.Release {
	return f.selected.release
}

// FetchReleaseDetails fetches full details for the selected release.
func (f *SearchFlow) FetchReleaseDetails() tea.Cmd {
	if f.selected.release == nil {
		return nil
	}
	return FetchReleaseDetailsCmd(f.client, f.selected.release.ID)
}

// FetchReleaseDetailsByID fetches full details for a specific release ID.
func (f *SearchFlow) FetchReleaseDetailsByID(releaseID string) tea.Cmd {
	return FetchReleaseDetailsCmd(f.client, releaseID)
}

// FetchCoverArt fetches cover art for the selected release.
func (f *SearchFlow) FetchCoverArt() tea.Cmd {
	if f.selected.release == nil {
		return nil
	}
	return FetchCoverArtCmd(f.client, f.selected.release.ID)
}

// FetchCoverArtByID fetches cover art for a specific release ID.
func (f *SearchFlow) FetchCoverArtByID(releaseID string) tea.Cmd {
	return FetchCoverArtCmd(f.client, releaseID)
}

// Selected returns the chosen release group and release.
func (f *SearchFlow) Selected() (*musicbrainz.ReleaseGroup, *musicbrainz.Release) {
	return f.selected.releaseGroup, f.selected.release
}

// Reset clears all search state.
func (f *SearchFlow) Reset() {
	f.query = ""
	f.releaseGroups = nil
	f.releases = nil
	f.selected.releaseGroup = nil
	f.selected.release = nil
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

// Command functions that can be used independently of SearchFlow.

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

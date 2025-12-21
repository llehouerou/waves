package retag

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/importer"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
)

// ReadAlbumTagsCmd reads tags from all album track files.
func ReadAlbumTagsCmd(paths []string) tea.Cmd {
	return func() tea.Msg {
		tags := make([]player.TrackInfo, len(paths))
		var mbReleaseID, mbReleaseGroupID, mbArtistID string

		for i, path := range paths {
			info, err := player.ReadTrackInfo(path)
			if err != nil {
				// Use empty info for files that can't be read
				tags[i] = player.TrackInfo{Path: path}
				continue
			}
			tags[i] = *info

			// Capture first non-empty MusicBrainz IDs
			if mbReleaseID == "" && info.MBReleaseID != "" {
				mbReleaseID = info.MBReleaseID
			}
			if mbReleaseGroupID == "" && info.MBReleaseGroupID != "" {
				mbReleaseGroupID = info.MBReleaseGroupID
			}
			if mbArtistID == "" && info.MBArtistID != "" {
				mbArtistID = info.MBArtistID
			}
		}

		return TagsReadMsg{
			Tags:             tags,
			MBReleaseID:      mbReleaseID,
			MBReleaseGroupID: mbReleaseGroupID,
			MBArtistID:       mbArtistID,
		}
	}
}

// SearchReleaseGroupsCmd searches MusicBrainz for release groups by free-form query.
// Used for manual search refinement.
func SearchReleaseGroupsCmd(client *musicbrainz.Client, query string) tea.Cmd {
	return func() tea.Msg {
		releaseGroups, err := client.SearchReleaseGroups(query)
		return ReleaseGroupSearchResultMsg{ReleaseGroups: releaseGroups, Err: err}
	}
}

// SearchReleaseGroupsByArtistAlbumCmd searches MusicBrainz for release groups
// using field-specific search for better accuracy.
func SearchReleaseGroupsByArtistAlbumCmd(client *musicbrainz.Client, artist, album string) tea.Cmd {
	return func() tea.Msg {
		releaseGroups, err := client.SearchReleaseGroupsByArtistAlbum(artist, album)
		return ReleaseGroupSearchResultMsg{ReleaseGroups: releaseGroups, Err: err}
	}
}

// FetchReleaseGroupsByArtistIDCmd fetches all release groups for an artist by ID.
// This is more accurate than search when we have the artist's MusicBrainz ID.
func FetchReleaseGroupsByArtistIDCmd(client *musicbrainz.Client, artistID string) tea.Cmd {
	return func() tea.Msg {
		releaseGroups, err := client.GetArtistReleaseGroups(artistID)
		return ReleaseGroupSearchResultMsg{ReleaseGroups: releaseGroups, Err: err}
	}
}

// FetchReleasesForReleaseGroupCmd fetches releases for a release group by ID.
func FetchReleasesForReleaseGroupCmd(client *musicbrainz.Client, releaseGroupID string) tea.Cmd {
	return func() tea.Msg {
		releases, err := client.GetReleaseGroupReleases(releaseGroupID)
		return ReleasesFetchedMsg{Releases: releases, Err: err}
	}
}

// FetchReleaseByIDCmd fetches full release details directly by MusicBrainz ID.
func FetchReleaseByIDCmd(client *musicbrainz.Client, releaseID string) tea.Cmd {
	return func() tea.Msg {
		release, err := client.GetRelease(releaseID)
		return ReleaseDetailsFetchedMsg{Release: release, Err: err}
	}
}

// FetchReleasesCmd fetches releases for a release group.
func FetchReleasesCmd(client *musicbrainz.Client, releaseGroupID string) tea.Cmd {
	return func() tea.Msg {
		releases, err := client.GetReleaseGroupReleases(releaseGroupID)
		return ReleasesFetchedMsg{Releases: releases, Err: err}
	}
}

// FetchReleaseDetailsCmd fetches full release details with tracks.
func FetchReleaseDetailsCmd(client *musicbrainz.Client, releaseID string) tea.Cmd {
	return func() tea.Msg {
		release, err := client.GetRelease(releaseID)
		return ReleaseDetailsFetchedMsg{Release: release, Err: err}
	}
}

// FileParams contains parameters for retagging a single file.
type FileParams struct {
	Index         int    // Index in file list (for tracking)
	Path          string // Full path to file
	ReleaseGroup  *musicbrainz.ReleaseGroup
	Release       *musicbrainz.ReleaseDetails
	TrackIndex    int // Index into Release.Tracks
	DiscNumber    int
	TotalDiscs    int
	ExistingGenre string // Preserve if new genre is empty
}

// FileCmd retags a single file with MusicBrainz metadata.
func FileCmd(params FileParams) tea.Cmd {
	return func() tea.Msg {
		track := &params.Release.Tracks[params.TrackIndex]

		tagData := importer.BuildTagData(importer.BuildTagDataParams{
			ReleaseGroup:  params.ReleaseGroup,
			Release:       params.Release,
			Track:         track,
			DiscNumber:    params.DiscNumber,
			TotalDiscs:    params.TotalDiscs,
			ExistingGenre: params.ExistingGenre,
		})

		err := importer.RetagFile(params.Path, tagData)
		return FileRetaggedMsg{Index: params.Index, Err: err}
	}
}

// UpdateTracksCmd updates only the specified tracks in the library.
// AddTracks handles FTS index updates incrementally.
func UpdateTracksCmd(lib *library.Library, paths []string) tea.Cmd {
	return func() tea.Msg {
		// Update only the retagged files in the database (FTS is updated incrementally)
		if err := lib.AddTracks(paths); err != nil {
			return LibraryUpdatedMsg{Err: err}
		}

		return LibraryUpdatedMsg{}
	}
}

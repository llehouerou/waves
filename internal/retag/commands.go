package retag

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/importer"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/musicbrainz/workflow"
	"github.com/llehouerou/waves/internal/tags"
)

// ReadAlbumTagsCmd reads tags from all album track files.
func ReadAlbumTagsCmd(paths []string) tea.Cmd {
	return func() tea.Msg {
		fileTags := make([]tags.FileInfo, len(paths))
		var mbReleaseID, mbReleaseGroupID, mbArtistID string

		for i, path := range paths {
			info, err := tags.Read(path)
			if err != nil {
				// Use empty info for files that can't be read
				fileTags[i] = tags.FileInfo{}
				fileTags[i].Path = path
				continue
			}
			fileTags[i] = tags.FileInfo{Tag: *info}

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
			Tags:             fileTags,
			MBReleaseID:      mbReleaseID,
			MBReleaseGroupID: mbReleaseGroupID,
			MBArtistID:       mbArtistID,
		}
	}
}

// FetchReleaseByIDCmd fetches full release details directly by MusicBrainz ID.
// This is retag-specific as it returns ReleaseDetailsFetchedMsg for direct ID lookup.
func FetchReleaseByIDCmd(client workflow.Client, releaseID string) tea.Cmd {
	return func() tea.Msg {
		release, err := client.GetRelease(releaseID)
		return workflow.ReleaseDetailsResultMsg{Details: release, Err: err}
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
	CoverArt      []byte // Cover art to embed (optional)
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
			CoverArt:      params.CoverArt,
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

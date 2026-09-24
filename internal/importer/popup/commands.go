package popup

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/tags"
)

// ReadTagsCmd reads tags from all source files.
func ReadTagsCmd(completedPath string, download *downloads.Download) tea.Cmd {
	return func() tea.Msg {
		fileTags := make([]tags.FileInfo, len(download.Files))

		// Sort files by track number
		sortedFiles := downloads.SortFilesByTrackNumber(download.Files)

		for i, f := range sortedFiles {
			filePath := downloads.ExpectedDiskPath(completedPath, f.Filename)

			info, err := tags.Read(filePath)
			if err != nil {
				// Use empty info for files that can't be read
				fileTags[i] = tags.FileInfo{}
				fileTags[i].Path = filePath
				continue
			}
			fileTags[i] = tags.FileInfo{Tag: *info}
		}

		return TagsReadMsg{DownloadID: download.ID, Tags: fileTags}
	}
}

// releaseClient is the part of the MusicBrainz client the popup uses.
type releaseClient interface {
	GetRelease(mbid string) (*musicbrainz.ReleaseDetails, error)
	GetCoverArt(releaseMBID string) ([]byte, error)
}

// RefreshReleaseCmd fetches fresh MusicBrainz release data.
// If releaseID differs from originalID, it means we're switching to a different release.
func RefreshReleaseCmd(client releaseClient, downloadID int64, releaseID, originalID string) tea.Cmd {
	return func() tea.Msg {
		release, err := client.GetRelease(releaseID)
		if err != nil {
			return MBReleaseRefreshedMsg{
				DownloadID: downloadID,
				Err:        err,
				SwitchedID: releaseID != originalID,
				OriginalID: originalID,
			}
		}
		return MBReleaseRefreshedMsg{
			DownloadID: downloadID,
			Release:    release,
			SwitchedID: releaseID != originalID,
			OriginalID: originalID,
		}
	}
}

// FetchCoverArtCmd fetches cover art from Cover Art Archive.
func FetchCoverArtCmd(client releaseClient, downloadID int64, releaseMBID string) tea.Cmd {
	return func() tea.Msg {
		data, err := client.GetCoverArt(releaseMBID)
		// GetCoverArt returns nil data (not error) for 404, which is fine
		return CoverArtFetchedMsg{DownloadID: downloadID, ReleaseID: releaseMBID, Data: data, Err: err}
	}
}

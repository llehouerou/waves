package popup

import (
	"path/filepath"
	"strings"

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
			// Build full path to file
			folderPath := downloads.BuildDiskPath(completedPath, download.SlskdDirectory)
			normalizedFilename := strings.ReplaceAll(f.Filename, "\\", "/")
			filePath := filepath.Join(folderPath, filepath.Base(normalizedFilename))

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

// RefreshReleaseCmd fetches fresh MusicBrainz release data.
// If releaseID differs from originalID, it means we're switching to a different release.
func RefreshReleaseCmd(client *musicbrainz.Client, downloadID int64, releaseID, originalID string) tea.Cmd {
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

// BuildSourcePath constructs the full path to a source file.
func BuildSourcePath(completedPath string, download *downloads.Download, file *downloads.DownloadFile) string {
	folderPath := downloads.BuildDiskPath(completedPath, download.SlskdDirectory)
	normalizedFilename := strings.ReplaceAll(file.Filename, "\\", "/")
	return filepath.Join(folderPath, filepath.Base(normalizedFilename))
}

// FetchCoverArtCmd fetches cover art from Cover Art Archive.
func FetchCoverArtCmd(client *musicbrainz.Client, downloadID int64, releaseMBID string) tea.Cmd {
	return func() tea.Msg {
		data, err := client.GetCoverArt(releaseMBID)
		// GetCoverArt returns nil data (not error) for 404, which is fine
		return CoverArtFetchedMsg{DownloadID: downloadID, Data: data, Err: err}
	}
}

package popup

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/importer"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
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

		return TagsReadMsg{Tags: fileTags}
	}
}

// RefreshReleaseCmd fetches fresh MusicBrainz release data.
// If releaseID differs from originalID, it means we're switching to a different release.
func RefreshReleaseCmd(client *musicbrainz.Client, releaseID, originalID string) tea.Cmd {
	return func() tea.Msg {
		release, err := client.GetRelease(releaseID)
		if err != nil {
			return MBReleaseRefreshedMsg{
				Err:        err,
				SwitchedID: releaseID != originalID,
				OriginalID: originalID,
			}
		}
		return MBReleaseRefreshedMsg{
			Release:    release,
			SwitchedID: releaseID != originalID,
			OriginalID: originalID,
		}
	}
}

// ImportFileCmd imports a single file.
func ImportFileCmd(params ImportFileParams) tea.Cmd {
	return func() tea.Msg {
		result, err := importer.Import(importer.ImportParams{
			SourcePath:   params.SourcePath,
			DestRoot:     params.DestRoot,
			ReleaseGroup: params.ReleaseGroup,
			Release:      params.Release,
			TrackIndex:   params.TrackIndex,
			DiscNumber:   params.DiscNumber,
			TotalDiscs:   params.TotalDiscs,
			CoverArt:     params.CoverArt,
			CopyMode:     false, // Move files, don't copy
			RenameConfig: params.RenameConfig,
		})

		if err != nil {
			return FileImportedMsg{Index: params.FileIndex, Err: err}
		}
		return FileImportedMsg{Index: params.FileIndex, DestPath: result.DestPath}
	}
}

// ImportFileParams contains the parameters needed to import a file.
type ImportFileParams struct {
	FileIndex    int    // Index in our file list (for tracking)
	SourcePath   string // Full path to source file
	DestRoot     string // Library root directory
	ReleaseGroup *musicbrainz.ReleaseGroup
	Release      *musicbrainz.ReleaseDetails
	TrackIndex   int // Index into Release.Tracks
	DiscNumber   int
	TotalDiscs   int
	CoverArt     []byte
	RenameConfig rename.Config
}

// RefreshLibraryParams contains parameters for library refresh after import.
type RefreshLibraryParams struct {
	Library      *library.Library
	Sources      []string
	DownloadID   int64
	ArtistName   string
	AlbumName    string
	AllSucceeded bool
}

// RefreshLibraryCmd refreshes the library after import.
func RefreshLibraryCmd(params RefreshLibraryParams) tea.Cmd {
	return func() tea.Msg {
		// Create a dummy progress channel that we'll drain
		progress := make(chan library.ScanProgress, 100)
		go func() {
			//nolint:revive // draining channel
			for range progress {
			}
		}()

		err := params.Library.Refresh(params.Sources, progress)
		close(progress)

		return LibraryRefreshedMsg{
			Err:          err,
			DownloadID:   params.DownloadID,
			ArtistName:   params.ArtistName,
			AlbumName:    params.AlbumName,
			AllSucceeded: params.AllSucceeded,
		}
	}
}

// RemoveDownloadCmd removes the download entry from the database.
func RemoveDownloadCmd(dlMgr *downloads.Manager, downloadID int64) tea.Cmd {
	return func() tea.Msg {
		err := dlMgr.Delete(downloadID)
		return DownloadRemovedMsg{Err: err}
	}
}

// BuildSourcePath constructs the full path to a source file.
func BuildSourcePath(completedPath string, download *downloads.Download, file *downloads.DownloadFile) string {
	folderPath := downloads.BuildDiskPath(completedPath, download.SlskdDirectory)
	normalizedFilename := strings.ReplaceAll(file.Filename, "\\", "/")
	return filepath.Join(folderPath, filepath.Base(normalizedFilename))
}

// FetchCoverArtCmd fetches cover art from Cover Art Archive.
func FetchCoverArtCmd(client *musicbrainz.Client, releaseMBID string) tea.Cmd {
	return func() tea.Msg {
		data, err := client.GetCoverArt(releaseMBID)
		// GetCoverArt returns nil data (not error) for 404, which is fine
		return CoverArtFetchedMsg{Data: data, Err: err}
	}
}

// BuildDestPath constructs the destination path for a file using the rename algorithm.
func BuildDestPath(destRoot string, download *downloads.Download, trackIndex int, cfg rename.Config) string {
	if download.MBReleaseDetails == nil || trackIndex >= len(download.MBReleaseDetails.Tracks) {
		return ""
	}

	track := download.MBReleaseDetails.Tracks[trackIndex]

	// Build metadata for renaming
	releaseType := ""
	secondaryTypes := ""
	if download.MBReleaseGroup != nil {
		releaseType = strings.ToLower(download.MBReleaseGroup.PrimaryType)
		secondaryTypes = strings.Join(download.MBReleaseGroup.SecondaryTypes, "; ")
	}

	originalDate := ""
	if download.MBReleaseGroup != nil {
		originalDate = download.MBReleaseGroup.FirstRelease
	}

	// Get disc info from release and track
	discNumber := track.DiscNumber
	if discNumber == 0 {
		discNumber = 1
	}
	totalDiscs := download.MBReleaseDetails.DiscCount
	if totalDiscs == 0 {
		totalDiscs = 1
	}

	meta := rename.TrackMetadata{
		Artist:               download.MBReleaseDetails.Artist,
		AlbumArtist:          download.MBReleaseDetails.Artist,
		Album:                download.MBReleaseDetails.Title,
		Title:                track.Title,
		TrackNumber:          track.Position,
		DiscNumber:           discNumber,
		TotalDiscs:           totalDiscs,
		Date:                 download.MBReleaseDetails.Date,
		OriginalDate:         originalDate,
		ReleaseType:          releaseType,
		SecondaryReleaseType: secondaryTypes,
	}

	relPath := rename.GeneratePathWithConfig(meta, cfg)
	return filepath.Join(destRoot, relPath)
}

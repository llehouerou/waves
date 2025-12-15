package popup

import (
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
)

// CloseMsg is sent when the import popup should be closed.
type CloseMsg struct{}

// TagsReadMsg is sent when current tags have been read from files.
type TagsReadMsg struct {
	Tags []player.TrackInfo
	Err  error
}

// MBReleaseRefreshedMsg is sent when MusicBrainz release data has been refreshed.
type MBReleaseRefreshedMsg struct {
	Release    *musicbrainz.ReleaseDetails
	SwitchedID bool   // True if we switched to a different release ID from files
	OriginalID string // The original release ID before switching
	Err        error
}

// FileImportedMsg is sent when a single file has been imported.
type FileImportedMsg struct {
	Index    int    // Index of the file in the list
	DestPath string // Path where file was imported
	Err      error
}

// ImportCompleteMsg is sent when all files have been processed.
type ImportCompleteMsg struct {
	SuccessCount  int
	FailedFiles   []FailedFile
	DownloadID    int64    // ID of download to remove on success
	ArtistName    string   // For library navigation
	AlbumName     string   // For library navigation
	AllSucceeded  bool     // True if no failures
	ImportedPaths []string // Paths of successfully imported files
}

// LibraryRefreshedMsg is sent when the library has been refreshed after import.
type LibraryRefreshedMsg struct {
	Err          error
	DownloadID   int64  // ID of download to remove (if AllSucceeded)
	ArtistName   string // For library navigation
	AlbumName    string // For library navigation
	AllSucceeded bool   // True if import fully succeeded
}

// DownloadRemovedMsg is sent when the download has been removed from the list.
type DownloadRemovedMsg struct {
	Err error
}

package popup

import (
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
)

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

// LibraryRefreshedMsg is sent when the library has been refreshed after import.
// Note: This message flows from app commands back to both app and popup,
// so it's kept as a message type rather than an action.
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

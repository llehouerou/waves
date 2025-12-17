package popup

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Close signals the import popup should close.
type Close struct{}

// ActionType implements action.Action.
func (a Close) ActionType() string { return "import.close" }

// ImportComplete signals all files have been processed.
type ImportComplete struct {
	SuccessCount  int
	FailedFiles   []FailedFile
	DownloadID    int64    // ID of download to remove on success
	ArtistName    string   // For library navigation
	AlbumName     string   // For library navigation
	AllSucceeded  bool     // True if no failures
	ImportedPaths []string // Paths of successfully imported files
}

// ActionType implements action.Action.
func (a ImportComplete) ActionType() string { return "import.complete" }

// LibraryRefreshed signals the library has been updated after import.
type LibraryRefreshed struct {
	Err          error
	DownloadID   int64  // ID of download to remove (if AllSucceeded)
	ArtistName   string // For library navigation
	AlbumName    string // For library navigation
	AllSucceeded bool   // True if import fully succeeded
}

// ActionType implements action.Action.
func (a LibraryRefreshed) ActionType() string { return "import.library_refreshed" }

// ActionMsg creates an action.Msg for an import popup action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "import", Action: a}
}

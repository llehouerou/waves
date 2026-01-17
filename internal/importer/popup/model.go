// Package popup provides an import popup for importing downloaded albums to the library.
package popup

import (
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui"
)

// State represents the current state of the import popup.
type State int

const (
	StateTagPreview  State = iota // Two-column tag comparison
	StatePathPreview              // File path table + library selector
	StateImporting                // Progress view
	StateComplete                 // Results summary
)

// Model is the Bubble Tea model for the import popup.
type Model struct {
	state    State
	download *downloads.Download

	// MusicBrainz client for refreshing release data
	mbClient *musicbrainz.Client

	// Source file info
	completedPath string // Path to soulseek completed downloads

	// Tag preview data
	currentTags []player.TrackInfo // Read from files
	tagDiffs    []TagDiff          // Computed differences
	loadingMB   bool               // True when refreshing MusicBrainz data

	// Path preview data
	librarySources []string      // Available destinations
	selectedSource int           // Cursor for source selection
	filePaths      []PathMapping // Current -> New path mappings
	pathOffset     int           // Scroll offset for paths list

	// Import progress
	importStatus []FileImportStatus // Status per file
	currentFile  int                // Currently importing index

	// Results
	successCount  int
	failedFiles   []FailedFile
	importedPaths []string // Paths of successfully imported files

	// Cover art
	coverArt        []byte // Cached cover art for embedding
	coverArtFetched bool   // True once cover art fetch attempted

	ui.Base
}

// TagDiff represents a difference between current and new tag values.
type TagDiff struct {
	Field    string
	OldValue string // Or "(N different)" for multi-value
	NewValue string
	Changed  bool
}

// PathMapping represents the mapping from old to new file path.
type PathMapping struct {
	TrackNum int
	OldPath  string // Full path to source file
	NewPath  string // Full path to destination
	Filename string // Just the filename for display
}

// FileImportStatus tracks the import status of a single file.
type FileImportStatus struct {
	Filename string
	Status   ImportStatus
	Error    string
}

// ImportStatus represents the status of a file import.
type ImportStatus int

const (
	StatusPending ImportStatus = iota
	StatusTagging
	StatusMoving
	StatusComplete
	StatusFailed
)

// FailedFile represents a file that failed to import.
type FailedFile struct {
	Filename string
	Error    string
}

// New creates a new import popup model.
func New(download *downloads.Download, completedPath string, librarySources []string, mbClient *musicbrainz.Client) *Model {
	m := &Model{
		state:          StateTagPreview,
		download:       download,
		mbClient:       mbClient,
		completedPath:  completedPath,
		librarySources: librarySources,
		selectedSource: 0,
	}
	m.SetFocused(true)

	// Initialize import status for all files
	m.importStatus = make([]FileImportStatus, len(download.Files))
	for i, f := range download.Files {
		m.importStatus[i] = FileImportStatus{
			Filename: f.Filename,
			Status:   StatusPending,
		}
	}

	return m
}

// State returns the current state.
func (m *Model) State() State {
	return m.state
}

// Download returns the download being imported.
func (m *Model) Download() *downloads.Download {
	return m.download
}

// IsComplete returns true if the import is complete (success or with errors).
func (m *Model) IsComplete() bool {
	return m.state == StateComplete
}

// SuccessCount returns the number of successfully imported files.
func (m *Model) SuccessCount() int {
	return m.successCount
}

// FailedCount returns the number of failed imports.
func (m *Model) FailedCount() int {
	return len(m.failedFiles)
}

// Package retag provides a popup for retagging existing library albums with MusicBrainz metadata.
package retag

import (
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

// State represents the current state of the retag popup.
type State int

const (
	StateLoading               State = iota // Reading tags from album tracks
	StateSearching                          // Searching MusicBrainz
	StateReleaseGroupResults                // Show release groups (with search input)
	StateReleaseLoading                     // Loading releases for selected release group
	StateReleaseResults                     // Show releases to select
	StateReleaseDetailsLoading              // Loading full release details
	StateTagPreview                         // Show tag diff
	StateRetagging                          // Applying tags
	StateComplete                           // Done summary
)

// Model is the Bubble Tea model for the retag popup.
type Model struct {
	state State

	// Album identification
	albumArtist string
	albumName   string
	trackPaths  []string // Full paths to all track files

	// Current tag data
	currentTags []player.TrackInfo

	// MusicBrainz client and data
	mbClient             *musicbrainz.Client
	releaseGroups        []musicbrainz.ReleaseGroup
	releaseGroupCursor   cursor.Cursor
	selectedReleaseGroup *musicbrainz.ReleaseGroup

	releases       []musicbrainz.Release
	releaseCursor  cursor.Cursor
	releaseDetails *musicbrainz.ReleaseDetails

	// Search refinement
	searchInput           textinput.Model
	searchMode            bool // True when user is typing a new search
	initialSearch         string
	foundMBReleaseID      string // Non-empty if MB release ID was found in tags
	foundMBReleaseGroupID string // Non-empty if MB release group ID was found
	foundMBArtistID       string // Non-empty if MB artist ID was found
	searchMethod          string // Description of how we're searching

	// Tag preview
	tagDiffs []TagDiff

	// Retag progress
	retagStatus  []FileRetagStatus
	currentFile  int
	successCount int
	failedFiles  []FailedFile

	// Library reference for refresh
	lib *library.Library

	// Status and error messages
	statusMsg string
	errorMsg  string

	// Dimensions
	width, height int
	focused       bool
}

// TagDiff represents a difference between current and new tag values.
type TagDiff struct {
	Field    string
	OldValue string // Or "(N different)" for multi-value
	NewValue string
	Changed  bool
}

// FileRetagStatus tracks the retag status of a single file.
type FileRetagStatus struct {
	Filename string
	Status   Status
	Error    string
}

// Status represents the status of a file retag operation.
type Status int

const (
	StatusPending Status = iota
	StatusRetagging
	StatusComplete
	StatusFailed
)

// FailedFile represents a file that failed to retag.
type FailedFile struct {
	Filename string
	Error    string
}

// New creates a new retag popup model.
func New(albumArtist, albumName string, trackPaths []string, mbClient *musicbrainz.Client, lib *library.Library) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search artist album..."
	ti.CharLimit = 256
	ti.Width = 50

	m := &Model{
		state:              StateLoading,
		albumArtist:        albumArtist,
		albumName:          albumName,
		trackPaths:         trackPaths,
		mbClient:           mbClient,
		lib:                lib,
		searchInput:        ti,
		initialSearch:      albumArtist + " " + albumName,
		focused:            true,
		releaseGroupCursor: cursor.New(2),
		releaseCursor:      cursor.New(2),
	}

	// Initialize retag status for all files
	m.retagStatus = make([]FileRetagStatus, len(trackPaths))
	for i, path := range trackPaths {
		m.retagStatus[i] = FileRetagStatus{
			Filename: path,
			Status:   StatusPending,
		}
	}

	return m
}

// SetSize sets the dimensions of the retag popup.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.searchInput.Width = width - 4
}

// SetFocused sets whether the retag popup is focused.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the retag popup is focused.
func (m *Model) IsFocused() bool {
	return m.focused
}

// State returns the current state.
func (m *Model) State() State {
	return m.state
}

// AlbumArtist returns the album artist being retagged.
func (m *Model) AlbumArtist() string {
	return m.albumArtist
}

// AlbumName returns the album name being retagged.
func (m *Model) AlbumName() string {
	return m.albumName
}

// IsComplete returns true if the retag is complete.
func (m *Model) IsComplete() bool {
	return m.state == StateComplete
}

// SuccessCount returns the number of successfully retagged files.
func (m *Model) SuccessCount() int {
	return m.successCount
}

// FailedCount returns the number of failed retags.
func (m *Model) FailedCount() int {
	return len(m.failedFiles)
}

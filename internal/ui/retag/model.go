// Package retag is the popup that retags a library album with MusicBrainz
// metadata: it finds the release, shows what the core's retag will write, and
// follows the retag the core runs.
package retag

import (
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
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

// Library is what the popup asks of the core. The retag it starts reports
// through core.RetagProgressed and core.RetagFinished, which the app hands to
// Update.
type Library interface {
	AlbumTags(albumArtist, album string) ([]core.FileTags, error)
	Retag(req core.RetagRequest) error
}

// Model is the Bubble Tea model for the retag popup.
type Model struct {
	state State

	// Album identification
	albumArtist string
	albumName   string

	// Current tag data: one per file of the album
	currentTags []core.FileTags

	// MusicBrainz client and data
	mbClient             mbworkflow.Client
	releaseGroups        []core.MBReleaseGroup
	releaseGroupCursor   cursor.Cursor
	selectedReleaseGroup *core.MBReleaseGroup

	releases       []core.MBRelease
	releaseCursor  cursor.Cursor
	releaseDetails *core.MBReleaseDetails

	// Search refinement
	searchInput           textinput.Model
	searchMode            bool // True when user is typing a new search
	initialSearch         string
	foundMBReleaseID      string // Non-empty if MB release ID was found in tags
	foundMBReleaseGroupID string // Non-empty if MB release group ID was found
	foundMBArtistID       string // Non-empty if MB artist ID was found
	searchMethod          string // Description of how we're searching
	skipAutoSelect        bool   // True when user navigated back, skip auto-selection

	// Tag preview
	tagDiffs []TagDiff

	// Where the retag is, as the core last said
	files []core.FileProgress

	// The core's library: the album's tags, and its retag
	lib Library

	// Status and error messages
	statusMsg string
	errorMsg  string

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

// New creates the retag popup of a library album. Its Init asks lib for the
// tags of the album's files.
func New(albumArtist, albumName string, mbClient mbworkflow.Client, lib Library) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search artist album..."
	ti.CharLimit = 256
	ti.Width = 50

	m := &Model{
		state:              StateLoading,
		albumArtist:        albumArtist,
		albumName:          albumName,
		mbClient:           mbClient,
		lib:                lib,
		searchInput:        ti,
		initialSearch:      albumArtist + " " + albumName,
		releaseGroupCursor: cursor.New(2),
		releaseCursor:      cursor.New(2),
	}
	m.SetFocused(true)
	return m
}

// SetSize sets the dimensions of the retag popup.
func (m *Model) SetSize(width, height int) {
	m.Base.SetSize(width, height)
	m.searchInput.Width = width - 4
}

// Retags reports whether the popup is about the album a retag was asked for.
func (m *Model) Retags(albumArtist, album string) bool {
	return m.albumArtist == albumArtist && m.albumName == album
}

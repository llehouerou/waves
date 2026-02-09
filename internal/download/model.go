// Package download provides a download manager view integrating MusicBrainz and slskd.
package download

import (
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

// FormatFilter represents the audio format filter option.
type FormatFilter int

const (
	FormatBoth     FormatFilter = iota // Show both lossy and lossless
	FormatLossless                     // Only FLAC
	FormatLossy                        // Only MP3 320
)

// Layout constants
const (
	// CompactWidthThreshold is the width below which compact layout is used.
	// In compact mode, the Directory column is hidden to fit narrow screens.
	CompactWidthThreshold = 90

	// slskdListOverhead is the number of lines used by header, separator,
	// filter controls, filter stats, and spacing in the slskd results view.
	slskdListOverhead = 8
)

// Model is the Bubble Tea model for the download view.
type Model struct {
	state       State
	searchInput textinput.Model
	searchQuery string

	// Artist results
	artistResults  []musicbrainz.Artist
	artistCursor   cursor.Cursor
	selectedArtist *musicbrainz.Artist

	// Release group results (grouped by type)
	releaseGroupsRaw     []musicbrainz.ReleaseGroup // Unfiltered release groups
	releaseGroups        []musicbrainz.ReleaseGroup // Filtered release groups
	releaseGroupCursor   cursor.Cursor
	selectedReleaseGroup *musicbrainz.ReleaseGroup

	// Release results (for track count selection)
	releasesRaw            []musicbrainz.Release // Unfiltered releases
	releases               []musicbrainz.Release // Filtered/deduplicated releases
	releaseCursor          cursor.Cursor
	selectedRelease        *musicbrainz.Release        // The release user selected
	selectedReleaseDetails *musicbrainz.ReleaseDetails // Full release details with tracks
	expectedTracks         int                         // Expected track count from MB (0 = no filtering)
	deduplicateRelease     bool                        // Deduplicate releases by track count/year/format

	// slskd state
	slskdClient      *slskd.Client
	slskdSearchID    string
	slskdRawResponse []slskd.SearchResponse // Raw responses for re-filtering
	slskdResults     []SlskdResult
	slskdCursor      cursor.Cursor
	filterStats      FilterStats

	// Filter settings
	formatFilter     FormatFilter // Lossy/Lossless/Both
	filterNoSlot     bool         // Filter out users with no free slot
	filterTrackCount bool         // Filter out results with fewer tracks than majority
	albumsOnly       bool         // Filter release groups to albums only

	// MusicBrainz client
	mbClient *musicbrainz.Client

	// Library for checking existing albums
	lib           *library.Library
	libraryAlbums map[string]struct{} // Normalized album titles for current artist

	// Status message
	statusMsg string
	errorMsg  string

	ui.Base

	// Download complete flag - true after successful download
	downloadComplete bool
}

// SlskdResult wraps slskd search results with scoring metadata.
type SlskdResult struct {
	Username    string
	Directory   string
	Files       []slskd.File
	Format      string // "FLAC", "MP3", etc.
	BitRate     int    // Bitrate in kbps (for lossy formats)
	FileCount   int
	TotalSize   int64
	IsComplete  bool // Matches MB track count
	Score       int  // Quality score for sorting
	UploadSpeed int  // User's upload speed in bytes/sec
}

// FilterConfig holds default filter settings.
type FilterConfig struct {
	Format     string // "both", "lossless", "lossy"
	NoSlot     *bool  // nil means use default (true)
	TrackCount *bool  // nil means use default (true)
	AlbumsOnly *bool  // nil means use default (true) - filter to albums only
}

// New creates a new download view model.
func New(slskdURL, slskdAPIKey string, filters FilterConfig, lib *library.Library) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search artist..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	// Determine format filter
	formatFilter := FormatBoth
	switch filters.Format {
	case "lossless":
		formatFilter = FormatLossless
	case "lossy":
		formatFilter = FormatLossy
	}

	// Determine boolean filters (default to true if not specified)
	filterNoSlot := true
	if filters.NoSlot != nil {
		filterNoSlot = *filters.NoSlot
	}
	filterTrackCount := true
	if filters.TrackCount != nil {
		filterTrackCount = *filters.TrackCount
	}
	albumsOnly := true
	if filters.AlbumsOnly != nil {
		albumsOnly = *filters.AlbumsOnly
	}

	m := &Model{
		state:              StateSearch,
		searchInput:        ti,
		mbClient:           musicbrainz.NewClient(),
		slskdClient:        slskd.NewClient(slskdURL, slskdAPIKey),
		formatFilter:       formatFilter,
		filterNoSlot:       filterNoSlot,
		filterTrackCount:   filterTrackCount,
		albumsOnly:         albumsOnly,
		deduplicateRelease: true,
		artistCursor:       cursor.New(2),
		releaseGroupCursor: cursor.New(2),
		releaseCursor:      cursor.New(2),
		slskdCursor:        cursor.New(2),
		lib:                lib,
	}
	m.SetFocused(true)
	return m
}

// SetSize sets the dimensions of the download view.
func (m *Model) SetSize(width, height int) {
	m.Base.SetSize(width, height)
	m.searchInput.Width = width - 4 // Leave some margin
}

// SetFocused sets whether the download view is focused.
func (m *Model) SetFocused(focused bool) {
	m.Base.SetFocused(focused)
	if focused && m.state == StateSearch {
		m.searchInput.Focus()
	}
}

// SetSearchQuery sets the search input value.
func (m *Model) SetSearchQuery(query string) {
	m.searchInput.SetValue(query)
}

// State returns the current state.
func (m *Model) State() State {
	return m.state
}

// SelectedReleaseGroup returns the currently selected MusicBrainz release group.
func (m *Model) SelectedReleaseGroup() *musicbrainz.ReleaseGroup {
	return m.selectedReleaseGroup
}

// Reset clears all state and returns to search mode.
func (m *Model) Reset() {
	m.state = StateSearch
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	m.searchQuery = ""
	m.artistResults = nil
	m.artistCursor.Reset()
	m.selectedArtist = nil
	m.releaseGroupsRaw = nil
	m.releaseGroups = nil
	m.releaseGroupCursor.Reset()
	m.selectedReleaseGroup = nil
	m.libraryAlbums = nil
	m.releasesRaw = nil
	m.releases = nil
	m.releaseCursor.Reset()
	m.selectedRelease = nil
	m.selectedReleaseDetails = nil
	m.expectedTracks = 0
	m.slskdSearchID = ""
	m.slskdRawResponse = nil
	m.slskdResults = nil
	m.slskdCursor.Reset()
	m.statusMsg = ""
	m.errorMsg = ""
	m.downloadComplete = false
}

// IsDownloadComplete returns true if download succeeded and popup can be closed.
func (m *Model) IsDownloadComplete() bool {
	return m.downloadComplete
}

// isCompactMode returns true if the view should use compact layout.
func (m *Model) isCompactMode() bool {
	return m.Width() < CompactWidthThreshold
}

// slskdListHeight returns the available height for the slskd results list.
func (m *Model) slskdListHeight() int {
	h := m.Height() - slskdListOverhead
	if h < 5 {
		return 5
	}
	return h
}

// reapplyFilters re-filters the raw responses with current filter settings.
func (m *Model) reapplyFilters() {
	if len(m.slskdRawResponse) == 0 {
		return
	}
	// Extract release year for folder name matching
	var releaseYear string
	if m.selectedRelease != nil && len(m.selectedRelease.Date) >= 4 {
		releaseYear = m.selectedRelease.Date[:4]
	}
	opts := FilterOptions{
		Format:           m.formatFilter,
		FilterNoSlot:     m.filterNoSlot,
		FilterTrackCount: m.filterTrackCount,
		ExpectedTracks:   m.expectedTracks,
		ReleaseYear:      releaseYear,
	}
	m.slskdResults, m.filterStats = FilterAndScoreResults(m.slskdRawResponse, opts)
	// Clamp cursor if it's out of bounds
	m.slskdCursor.ClampToBounds(len(m.slskdResults))
}

// loadLibraryAlbums loads normalized album titles for the given artist from the library.
func (m *Model) loadLibraryAlbums(artistName string) {
	if m.lib == nil {
		m.libraryAlbums = nil
		return
	}
	m.libraryAlbums = m.lib.AlbumsForArtistNormalized(artistName)
}

// IsInLibrary checks if a release group's album is already in the library.
func (m *Model) IsInLibrary(rg musicbrainz.ReleaseGroup) bool {
	if m.libraryAlbums == nil {
		return false
	}
	normalized := library.NormalizeTitle(rg.Title)
	_, exists := m.libraryAlbums[normalized]
	return exists
}

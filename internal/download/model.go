// Package download provides a download manager view integrating MusicBrainz and slskd.
package download

import (
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/slskd"
)

// State represents the current state of the download view.
type State int

const (
	StateSearch              State = iota // Waiting for search input
	StateArtistSearching                  // Searching artists
	StateArtistResults                    // Showing artist results
	StateReleaseGroupLoading              // Loading release groups
	StateReleaseGroupResults              // Showing release groups
	StateReleaseLoading                   // Loading releases for track count
	StateReleaseResults                   // Showing releases to select track count
	StateSlskdSearching                   // Searching slskd
	StateSlskdResults                     // Showing slskd results
	StateDownloading                      // Download queued
)

// FormatFilter represents the audio format filter option.
type FormatFilter int

const (
	FormatBoth     FormatFilter = iota // Show both lossy and lossless
	FormatLossless                     // Only FLAC
	FormatLossy                        // Only MP3 320
)

// Model is the Bubble Tea model for the download view.
type Model struct {
	state       State
	searchInput textinput.Model
	searchQuery string

	// Artist results
	artistResults  []musicbrainz.Artist
	artistCursor   int
	selectedArtist *musicbrainz.Artist

	// Release group results (grouped by type)
	releaseGroupsRaw     []musicbrainz.ReleaseGroup // Unfiltered release groups
	releaseGroups        []musicbrainz.ReleaseGroup // Filtered release groups
	releaseGroupCursor   int
	selectedReleaseGroup *musicbrainz.ReleaseGroup

	// Release results (for track count selection)
	releases       []musicbrainz.Release
	releaseCursor  int
	expectedTracks int // Expected track count from MB (0 = no filtering)

	// slskd state
	slskdClient      *slskd.Client
	slskdSearchID    string
	slskdRawResponse []slskd.SearchResponse // Raw responses for re-filtering
	slskdResults     []SlskdResult
	slskdCursor      int
	filterStats      FilterStats

	// Filter settings
	formatFilter     FormatFilter // Lossy/Lossless/Both
	filterNoSlot     bool         // Filter out users with no free slot
	filterTrackCount bool         // Filter out results with fewer tracks than majority
	albumsOnly       bool         // Filter release groups to albums only

	// MusicBrainz client
	mbClient *musicbrainz.Client

	// Status message
	statusMsg string
	errorMsg  string

	// Dimensions
	width, height int
	focused       bool

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
func New(slskdURL, slskdAPIKey string, filters FilterConfig) *Model {
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

	return &Model{
		state:            StateSearch,
		searchInput:      ti,
		mbClient:         musicbrainz.NewClient(),
		slskdClient:      slskd.NewClient(slskdURL, slskdAPIKey),
		focused:          true,
		formatFilter:     formatFilter,
		filterNoSlot:     filterNoSlot,
		filterTrackCount: filterTrackCount,
		albumsOnly:       albumsOnly,
	}
}

// SetSize sets the dimensions of the download view.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.searchInput.Width = width - 4 // Leave some margin
}

// SetFocused sets whether the download view is focused.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
	if focused && m.state == StateSearch {
		m.searchInput.Focus()
	}
}

// IsFocused returns whether the download view is focused.
func (m *Model) IsFocused() bool {
	return m.focused
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
	m.artistCursor = 0
	m.selectedArtist = nil
	m.releaseGroupsRaw = nil
	m.releaseGroups = nil
	m.releaseGroupCursor = 0
	m.selectedReleaseGroup = nil
	m.releases = nil
	m.releaseCursor = 0
	m.expectedTracks = 0
	m.slskdSearchID = ""
	m.slskdRawResponse = nil
	m.slskdResults = nil
	m.slskdCursor = 0
	m.statusMsg = ""
	m.errorMsg = ""
	m.downloadComplete = false
}

// IsDownloadComplete returns true if download succeeded and popup can be closed.
func (m *Model) IsDownloadComplete() bool {
	return m.downloadComplete
}

// currentListLen returns the length of the current list based on state.
func (m *Model) currentListLen() int {
	switch m.state {
	case StateArtistResults:
		return len(m.artistResults)
	case StateReleaseGroupResults:
		return len(m.releaseGroups)
	case StateReleaseResults:
		return len(m.releases)
	case StateSlskdResults:
		return len(m.slskdResults)
	case StateSearch, StateArtistSearching, StateReleaseGroupLoading,
		StateReleaseLoading, StateSlskdSearching, StateDownloading:
		return 0
	}
	return 0
}

// currentCursor returns a pointer to the current cursor based on state.
func (m *Model) currentCursor() *int {
	switch m.state {
	case StateArtistResults:
		return &m.artistCursor
	case StateReleaseGroupResults:
		return &m.releaseGroupCursor
	case StateReleaseResults:
		return &m.releaseCursor
	case StateSlskdResults:
		return &m.slskdCursor
	case StateSearch, StateArtistSearching, StateReleaseGroupLoading,
		StateReleaseLoading, StateSlskdSearching, StateDownloading:
		return nil
	}
	return nil
}

// analyzeTrackCounts analyzes releases to find the expected track count.
// Returns (trackCount, needsSelection):
// - If all releases have same count: (count, false)
// - If majority have same count (all but 1-2): (count, false)
// - Otherwise: (0, true) - user needs to select
func analyzeTrackCounts(releases []musicbrainz.Release) (trackCount int, needsSelection bool) {
	if len(releases) == 0 {
		return 0, false
	}

	// Count occurrences of each track count
	counts := make(map[int]int)
	for i := range releases {
		counts[releases[i].TrackCount]++
	}

	// If only one unique count, use it
	if len(counts) == 1 {
		for tc := range counts {
			return tc, false
		}
	}

	// Find the most common count
	var maxCount, maxTrackCount int
	for tc, count := range counts {
		if count > maxCount {
			maxCount = count
			maxTrackCount = tc
		}
	}

	// If majority (all but 1-2) have same count, auto-select
	// Threshold: at least 75% of releases, or all but 2
	totalReleases := len(releases)
	outliers := totalReleases - maxCount

	if outliers <= 2 || float64(maxCount)/float64(totalReleases) >= 0.75 {
		return maxTrackCount, false
	}

	// Need user selection
	return 0, true
}

// reapplyFilters re-filters the raw responses with current filter settings.
func (m *Model) reapplyFilters() {
	if len(m.slskdRawResponse) == 0 {
		return
	}
	opts := FilterOptions{
		Format:           m.formatFilter,
		FilterNoSlot:     m.filterNoSlot,
		FilterTrackCount: m.filterTrackCount,
		ExpectedTracks:   m.expectedTracks,
	}
	m.slskdResults, m.filterStats = FilterAndScoreResults(m.slskdRawResponse, opts)
	// Reset cursor if it's out of bounds
	if m.slskdCursor >= len(m.slskdResults) {
		m.slskdCursor = max(0, len(m.slskdResults)-1)
	}
}

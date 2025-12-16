package albumview

import (
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui"
)

// GroupBy specifies how albums are grouped.
type GroupBy int

const (
	GroupByWeek    GroupBy = iota // Week of original_date (with year-only fallback)
	GroupByMonth                  // Month of original_date
	GroupByYear                   // Year of original_date
	GroupByArtist                 // By album_artist
	GroupByGenre                  // By genre
	GroupByAddedAt                // When added to library
	GroupByNone                   // No grouping
)

// SortBy specifies how albums are sorted within groups.
type SortBy int

const (
	SortByOriginalDate SortBy = iota
	SortByReleaseDate
	SortByAddedAt
	SortByArtist
	SortByAlbum
)

// SortOrder specifies ascending or descending.
type SortOrder int

const (
	SortDesc SortOrder = iota // Newest first (default)
	SortAsc                   // Oldest first
)

// Settings holds the current view configuration.
type Settings struct {
	GroupBy   GroupBy
	SortBy    SortBy
	SortOrder SortOrder
}

// DefaultSettings returns the default album view settings.
func DefaultSettings() Settings {
	return Settings{
		GroupBy:   GroupByMonth,
		SortBy:    SortByOriginalDate,
		SortOrder: SortDesc,
	}
}

// Group represents a group of albums with a header.
type Group struct {
	Header string
	Albums []library.AlbumEntry
}

// AlbumItem represents either a group header or an album in the flat list.
type AlbumItem struct {
	IsHeader bool
	Header   string
	Album    *library.AlbumEntry
}

// Model represents the album view state.
type Model struct {
	lib      *library.Library
	settings Settings
	groups   []Group
	flatList []AlbumItem
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
}

// New creates a new album view model.
func New(lib *library.Library) Model {
	return Model{
		lib:      lib,
		settings: DefaultSettings(),
	}
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.centerCursor()
}

// SetFocused sets the focus state.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns the focus state.
func (m Model) IsFocused() bool {
	return m.focused
}

// Settings returns the current settings.
func (m Model) Settings() Settings {
	return m.settings
}

// SetSettings updates the settings and triggers a refresh.
func (m *Model) SetSettings(settings Settings) {
	m.settings = settings
}

// SelectedAlbum returns the currently selected album, or nil if on a header.
func (m Model) SelectedAlbum() *library.AlbumEntry {
	if m.cursor < 0 || m.cursor >= len(m.flatList) {
		return nil
	}
	item := m.flatList[m.cursor]
	if item.IsHeader {
		return nil
	}
	return item.Album
}

// SelectedID returns a unique identifier for the selected album ("artist:album").
func (m Model) SelectedID() string {
	album := m.SelectedAlbum()
	if album == nil {
		return ""
	}
	return album.AlbumArtist + ":" + album.Album
}

// SelectByID selects the album matching the given ID ("artist:album").
func (m *Model) SelectByID(id string) {
	for i, item := range m.flatList {
		if item.IsHeader {
			continue
		}
		if item.Album != nil && item.Album.AlbumArtist+":"+item.Album.Album == id {
			m.cursor = i
			m.centerCursor()
			return
		}
	}
}

// listHeight returns the number of visible list rows.
func (m Model) listHeight() int {
	// Account for header line and separator
	return m.height - 4
}

// ensureCursorVisible adjusts offset to keep cursor in view with scroll margin.
func (m *Model) ensureCursorVisible() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}

	// Check if there's a group header above the cursor that should be visible
	targetOffset := m.cursor
	if m.cursor > 0 && m.flatList[m.cursor-1].IsHeader {
		targetOffset = m.cursor - 1
	}

	// Scroll up if cursor is too close to top (with margin)
	if targetOffset < m.offset+ui.ScrollMargin {
		m.offset = max(targetOffset-ui.ScrollMargin, 0)
	}

	// Scroll down if cursor is too close to bottom (with margin)
	if m.cursor >= m.offset+listHeight-ui.ScrollMargin {
		m.offset = m.cursor - listHeight + ui.ScrollMargin + 1
	}
}

// centerCursor centers the cursor in the viewport.
func (m *Model) centerCursor() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}

	// Calculate offset to center the cursor
	targetOffset := m.cursor - listHeight/2

	// Include header if present
	if m.cursor > 0 && m.flatList[m.cursor-1].IsHeader {
		targetOffset--
	}

	// Clamp to valid range
	maxOffset := max(len(m.flatList)-listHeight, 0)
	m.offset = max(min(targetOffset, maxOffset), 0)
}

// ensureCursorInBounds ensures cursor is within valid range.
func (m *Model) ensureCursorInBounds() {
	if len(m.flatList) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}

	if m.cursor >= len(m.flatList) {
		m.cursor = len(m.flatList) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	// Skip headers
	for m.cursor < len(m.flatList) && m.flatList[m.cursor].IsHeader {
		m.cursor++
	}
	if m.cursor >= len(m.flatList) {
		// Go back to find last non-header
		for m.cursor > 0 && m.flatList[m.cursor].IsHeader {
			m.cursor--
		}
	}

	m.ensureCursorVisible()
}

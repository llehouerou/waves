package albumview

import (
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
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
	cursor   cursor.Cursor
	width    int
	height   int
	focused  bool
}

// New creates a new album view model.
func New(lib *library.Library) Model {
	return Model{
		lib:      lib,
		settings: DefaultSettings(),
		cursor:   cursor.New(ui.ScrollMargin),
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
	pos := m.cursor.Pos()
	if pos < 0 || pos >= len(m.flatList) {
		return nil
	}
	item := m.flatList[pos]
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
			m.cursor.SetPos(i)
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
// This has header-aware logic that cannot be delegated to the cursor package.
func (m *Model) ensureCursorVisible() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}

	pos := m.cursor.Pos()
	offset := m.cursor.Offset()

	// Check if there's a group header above the cursor that should be visible
	targetOffset := pos
	if pos > 0 && m.flatList[pos-1].IsHeader {
		targetOffset = pos - 1
	}

	// Scroll up if cursor is too close to top (with margin)
	if targetOffset < offset+ui.ScrollMargin {
		m.cursor.SetOffset(max(targetOffset-ui.ScrollMargin, 0))
	}

	// Scroll down if cursor is too close to bottom (with margin)
	if pos >= m.cursor.Offset()+listHeight-ui.ScrollMargin {
		m.cursor.SetOffset(pos - listHeight + ui.ScrollMargin + 1)
	}
}

// centerCursor centers the cursor in the viewport.
// This has header-aware logic that cannot be delegated to the cursor package.
func (m *Model) centerCursor() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}

	pos := m.cursor.Pos()

	// Calculate offset to center the cursor
	targetOffset := pos - listHeight/2

	// Include header if present
	if pos > 0 && m.flatList[pos-1].IsHeader {
		targetOffset--
	}

	// Clamp to valid range
	maxOffset := max(len(m.flatList)-listHeight, 0)
	m.cursor.SetOffset(max(min(targetOffset, maxOffset), 0))
}

// ensureCursorInBounds ensures cursor is within valid range.
// This has header-skipping logic that cannot be delegated to the cursor package.
func (m *Model) ensureCursorInBounds() {
	if len(m.flatList) == 0 {
		m.cursor.Reset()
		return
	}

	pos := m.cursor.Pos()
	if pos >= len(m.flatList) {
		pos = len(m.flatList) - 1
	}
	if pos < 0 {
		pos = 0
	}

	// Skip headers
	for pos < len(m.flatList) && m.flatList[pos].IsHeader {
		pos++
	}
	if pos >= len(m.flatList) {
		// Go back to find last non-header
		for pos > 0 && m.flatList[pos].IsHeader {
			pos--
		}
	}

	m.cursor.SetPos(pos)
	m.ensureCursorVisible()
}

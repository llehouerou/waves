// Package librarybrowser provides a 3-column library browser (Artists, Albums, Tracks)
// with a contextual description panel.
package librarybrowser

import (
	"fmt"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

// Column represents which column has focus.
type Column int

const (
	ColumnArtists Column = iota
	ColumnAlbums
	ColumnTracks
)

// Model is the library browser state.
type Model struct {
	library *library.Library

	artists []string        // artist names
	albums  []library.Album // albums for selected artist
	tracks  []library.Track // tracks for selected album

	artistCursor cursor.Cursor
	albumCursor  cursor.Cursor
	trackCursor  cursor.Cursor

	activeColumn Column

	favorites map[int64]bool

	focused bool
	width   int
	height  int
}

// New creates a new library browser model.
func New(lib *library.Library) Model {
	return Model{
		library:      lib,
		artistCursor: cursor.New(5),
		albumCursor:  cursor.New(5),
		trackCursor:  cursor.New(5),
		activeColumn: ColumnArtists,
		favorites:    make(map[int64]bool),
	}
}

// SetFocused sets the focus state.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns true if the browser has focus.
func (m Model) IsFocused() bool {
	return m.focused
}

// SetSize updates the dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFavorites sets the favorite track IDs.
func (m *Model) SetFavorites(favorites map[int64]bool) {
	m.favorites = favorites
}

// ActiveColumn returns the currently active column.
func (m Model) ActiveColumn() Column {
	return m.activeColumn
}

// SelectedArtist returns the currently selected artist name, or empty string.
func (m Model) SelectedArtist() string {
	if len(m.artists) == 0 {
		return ""
	}
	return m.artists[m.artistCursor.Pos()]
}

// SelectedAlbum returns the currently selected album, or nil.
func (m Model) SelectedAlbum() *library.Album {
	if len(m.albums) == 0 {
		return nil
	}
	a := m.albums[m.albumCursor.Pos()]
	return &a
}

// SelectedTrack returns the currently selected track, or nil.
func (m Model) SelectedTrack() *library.Track {
	if len(m.tracks) == 0 {
		return nil
	}
	t := m.tracks[m.trackCursor.Pos()]
	return &t
}

// SelectedArtistName returns the selected artist name for state persistence.
func (m Model) SelectedArtistName() string {
	return m.SelectedArtist()
}

// SetActiveColumn sets the active column (for state restoration).
func (m *Model) SetActiveColumn(col Column) {
	m.activeColumn = col
}

// SelectArtist restores artist selection by name.
// Resets album and track cursors. Does not adjust scroll offset (call CenterCursors after resize).
func (m *Model) SelectArtist(name string) {
	for i, a := range m.artists {
		if a == name {
			m.artistCursor.SetPos(i)
			m.resetAlbumsAndTracks()
			return
		}
	}
}

// SelectAlbum restores album selection by name.
// Resets track cursor. Does not adjust scroll offset (call CenterCursors after resize).
func (m *Model) SelectAlbum(albumName string) {
	for i, a := range m.albums {
		if a.Name == albumName {
			m.albumCursor.SetPos(i)
			m.resetTracks()
			return
		}
	}
}

// SelectTrackByID restores track selection by track ID.
// Does not adjust scroll offset (call CenterCursors after resize).
func (m *Model) SelectTrackByID(trackID int64) {
	for i := range m.tracks {
		if m.tracks[i].ID == trackID {
			m.trackCursor.SetPos(i)
			return
		}
	}
}

// CenterCursors centers all cursor scroll offsets around the current position.
// Call this after resize when dimensions are known.
func (m *Model) CenterCursors() {
	h := m.columnHeight()
	m.artistCursor.Center(len(m.artists), h)
	m.albumCursor.Center(len(m.albums), h)
	m.trackCursor.Center(len(m.tracks), h)
}

// JumpToIndex jumps to an item by index in the given column,
// reloading child data as needed.
func (m *Model) JumpToIndex(col Column, idx int) {
	h := m.columnHeight()
	switch col {
	case ColumnArtists:
		m.artistCursor.Jump(idx, len(m.artists), h)
		m.resetAlbumsAndTracks()
	case ColumnAlbums:
		m.albumCursor.Jump(idx, len(m.albums), h)
		m.resetTracks()
	case ColumnTracks:
		m.trackCursor.Jump(idx, len(m.tracks), h)
	}
}

// CurrentColumnSearchItems returns the active column's items for local search.
func (m Model) CurrentColumnSearchItems() []search.Item {
	switch m.activeColumn {
	case ColumnArtists:
		items := make([]search.Item, len(m.artists))
		for i, a := range m.artists {
			items[i] = SearchItem{Column: ColumnArtists, Index: i, Name: a}
		}
		return items
	case ColumnAlbums:
		items := make([]search.Item, len(m.albums))
		for i, a := range m.albums {
			name := a.Name
			if a.Year > 0 {
				name = fmt.Sprintf("%s (%d)", name, a.Year)
			}
			items[i] = SearchItem{Column: ColumnAlbums, Index: i, Name: name}
		}
		return items
	case ColumnTracks:
		items := make([]search.Item, len(m.tracks))
		for i := range m.tracks {
			items[i] = SearchItem{Column: ColumnTracks, Index: i, Name: fmt.Sprintf("%02d. %s", m.tracks[i].TrackNumber, m.tracks[i].Title)}
		}
		return items
	}
	return nil
}

// SearchItem represents a browser column item for local search.
type SearchItem struct {
	Column Column
	Index  int
	Name   string
}

// FilterValue returns the text to match against.
func (b SearchItem) FilterValue() string { return b.Name }

// DisplayText returns the text to display in results.
func (b SearchItem) DisplayText() string { return b.Name }

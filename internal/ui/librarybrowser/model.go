// Package librarybrowser provides a 3-column library browser (Artists, Albums, Tracks)
// with a contextual description panel.
package librarybrowser

import (
	"github.com/llehouerou/waves/internal/library"
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

// SelectArtist restores artist selection by name.
func (m *Model) SelectArtist(name string) {
	for i, a := range m.artists {
		if a == name {
			m.artistCursor.Jump(i, len(m.artists), m.columnHeight())
			m.loadAlbumsForSelectedArtist()
			return
		}
	}
}

// SelectAlbum restores album selection by name.
func (m *Model) SelectAlbum(albumName string) {
	for i, a := range m.albums {
		if a.Name == albumName {
			m.albumCursor.Jump(i, len(m.albums), m.columnHeight())
			m.loadTracksForSelectedAlbum()
			return
		}
	}
}

// SelectTrackByID restores track selection by track ID.
func (m *Model) SelectTrackByID(trackID int64) {
	for i := range m.tracks {
		if m.tracks[i].ID == trackID {
			m.trackCursor.Jump(i, len(m.tracks), m.columnHeight())
			return
		}
	}
}

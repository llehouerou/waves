package librarybrowser

import (
	"slices"

	"github.com/llehouerou/waves/internal/core"
)

// Refresh reloads the library and keeps the selected artist, album and
// track when they still exist, wherever their rows moved. A column whose
// item is gone keeps its cursor on the same row, clamped to its new length;
// in the tracks column only while the same album is selected: when another
// album takes the row, its first track is selected.
func (m *Model) Refresh() error {
	artists, err := m.library.Artists()
	if err != nil {
		return err
	}
	artist, album, track := m.SelectedArtist(), m.SelectedAlbum(), m.SelectedTrack()
	trackCursor := m.trackCursor
	h := m.columnHeight()

	m.artists = artists
	m.artistCursor.ClampToBounds(len(m.artists))
	if i := slices.Index(m.artists, artist); i >= 0 {
		m.artistCursor.Jump(i, len(m.artists), h)
	}
	sameArtist := artist != "" && m.SelectedArtist() == artist
	m.loadAlbumsForSelectedArtist()

	if sameArtist && album != nil {
		i := slices.IndexFunc(m.albums, func(a core.Album) bool { return a.Name == album.Name })
		if i >= 0 && i != m.albumCursor.Pos() {
			m.albumCursor.Jump(i, len(m.albums), h)
			m.resetTracks()
		}
	}

	if now := m.SelectedAlbum(); sameArtist && album != nil && now != nil && now.Name == album.Name {
		m.trackCursor = trackCursor
		m.trackCursor.ClampToBounds(len(m.tracks))
		if track != nil {
			if i := slices.IndexFunc(m.tracks, func(t core.LibraryTrack) bool { return t.ID == track.ID }); i >= 0 {
				m.trackCursor.SetPos(i)
			}
		}
		m.trackCursor.EnsureVisible(len(m.tracks), h)
	}
	return nil
}

// loadAlbumsForSelectedArtist loads albums for the currently selected artist.
func (m *Model) loadAlbumsForSelectedArtist() {
	artist := m.SelectedArtist()
	if artist == "" {
		m.albums = nil
		m.tracks = nil
		m.albumCursor.Reset()
		m.trackCursor.Reset()
		return
	}

	albums, err := m.library.Albums(artist)
	if err != nil {
		m.albums = nil
		m.tracks = nil
		m.albumCursor.Reset()
		m.trackCursor.Reset()
		return
	}

	m.albums = albums
	m.albumCursor.ClampToBounds(len(m.albums))
	m.trackCursor.Reset()
	m.loadTracksForSelectedAlbum()
}

// loadTracksForSelectedAlbum loads tracks for the currently selected album.
func (m *Model) loadTracksForSelectedAlbum() {
	artist := m.SelectedArtist()
	album := m.SelectedAlbum()
	if artist == "" || album == nil {
		m.tracks = nil
		m.trackCursor.Reset()
		return
	}

	tracks, err := m.library.Tracks(artist, album.Name)
	if err != nil {
		m.tracks = nil
		m.trackCursor.Reset()
		return
	}

	m.tracks = tracks
	m.trackCursor.ClampToBounds(len(m.tracks))
}

// resetAlbumsAndTracks resets album cursor and reloads albums/tracks.
func (m *Model) resetAlbumsAndTracks() {
	m.albumCursor.Reset()
	m.loadAlbumsForSelectedArtist()
}

// resetTracks resets track cursor and reloads tracks.
func (m *Model) resetTracks() {
	m.trackCursor.Reset()
	m.loadTracksForSelectedAlbum()
}

package librarybrowser

// Refresh reloads artists from the library and resets state.
func (m *Model) Refresh() error {
	artists, err := m.library.Artists()
	if err != nil {
		return err
	}
	m.artists = artists
	m.artistCursor.ClampToBounds(len(m.artists))
	m.loadAlbumsForSelectedArtist()
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

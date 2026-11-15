package librarybrowser

// Refresh reloads the library. Each column keeps its cursor on the same
// row, clamped to its new length. The tracks column keeps its row while the
// same album is selected, so a removed track leaves the cursor on the track
// that followed it; when another album takes the row, its first track is
// selected.
func (m *Model) Refresh() error {
	artists, err := m.library.Artists()
	if err != nil {
		return err
	}
	artist, album := m.SelectedArtist(), m.SelectedAlbum()
	trackCursor := m.trackCursor

	m.artists = artists
	m.artistCursor.ClampToBounds(len(m.artists))
	m.loadAlbumsForSelectedArtist()

	if now := m.SelectedAlbum(); album != nil && now != nil &&
		m.SelectedArtist() == artist && now.Name == album.Name {
		m.trackCursor = trackCursor
		m.trackCursor.ClampToBounds(len(m.tracks))
		m.trackCursor.EnsureVisible(len(m.tracks), m.columnHeight())
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

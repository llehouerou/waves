package playlists

// IsFavorite checks if a track is in the Favorites playlist.
func (p *Playlists) IsFavorite(trackID int64) (bool, error) {
	var count int
	err := p.db.QueryRow(`
		SELECT COUNT(*) FROM playlist_tracks
		WHERE playlist_id = ? AND library_track_id = ?
	`, FavoritesPlaylistID, trackID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ToggleFavorite adds a track to Favorites if not there, removes if already favorited.
// Returns the new favorite status (true = now favorited).
func (p *Playlists) ToggleFavorite(trackID int64) (bool, error) {
	isFav, err := p.IsFavorite(trackID)
	if err != nil {
		return false, err
	}

	if isFav {
		// Remove from favorites
		_, err = p.db.Exec(`
			DELETE FROM playlist_tracks
			WHERE playlist_id = ? AND library_track_id = ?
		`, FavoritesPlaylistID, trackID)
		if err != nil {
			return false, err
		}
		return false, nil
	}

	// Add to favorites
	err = p.AddTracks(FavoritesPlaylistID, []int64{trackID})
	if err != nil {
		return false, err
	}
	return true, nil
}

// ToggleFavorites batch version for multiple tracks.
// Returns map of trackID -> new favorite status.
func (p *Playlists) ToggleFavorites(trackIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(trackIDs))
	for _, id := range trackIDs {
		status, err := p.ToggleFavorite(id)
		if err != nil {
			return nil, err
		}
		result[id] = status
	}
	return result, nil
}

// FavoriteTrackIDs returns all track IDs in Favorites as a map for efficient lookup.
func (p *Playlists) FavoriteTrackIDs() (map[int64]bool, error) {
	rows, err := p.db.Query(`
		SELECT library_track_id FROM playlist_tracks
		WHERE playlist_id = ?
	`, FavoritesPlaylistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	favorites := make(map[int64]bool)
	for rows.Next() {
		var trackID int64
		if err := rows.Scan(&trackID); err != nil {
			return nil, err
		}
		favorites[trackID] = true
	}
	return favorites, rows.Err()
}

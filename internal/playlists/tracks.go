package playlists

import (
	"database/sql"

	dbutil "github.com/llehouerou/waves/internal/db"
	"github.com/llehouerou/waves/internal/playlist"
)

// Tracks returns all tracks in a playlist, joined with library data.
// Returns playlist.Track instances ready for playback.
func (p *Playlists) Tracks(playlistID int64) ([]playlist.Track, error) {
	rows, err := p.db.Query(`
		SELECT pt.library_track_id, lt.path, lt.title, lt.artist, lt.album, lt.track_number
		FROM playlist_tracks pt
		JOIN library_tracks lt ON pt.library_track_id = lt.id
		WHERE pt.playlist_id = ?
		ORDER BY pt.position
	`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []playlist.Track
	for rows.Next() {
		var t playlist.Track
		var trackNum sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &trackNum); err != nil {
			return nil, err
		}
		t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// TrackCount returns the number of tracks in a playlist.
func (p *Playlists) TrackCount(playlistID int64) (int, error) {
	var count int
	err := p.db.QueryRow(`
		SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ?
	`, playlistID).Scan(&count)
	return count, err
}

// AddTracks adds tracks to a playlist by their library track IDs.
func (p *Playlists) AddTracks(playlistID int64, trackIDs []int64) error {
	if len(trackIDs) == 0 {
		return nil
	}

	// Get current max position
	var maxPos sql.NullInt64
	err := p.db.QueryRow(`
		SELECT MAX(position) FROM playlist_tracks WHERE playlist_id = ?
	`, playlistID).Scan(&maxPos)
	if err != nil {
		return err
	}

	nextPos := int(dbutil.NullInt64Value(maxPos))
	if maxPos.Valid {
		nextPos++
	}

	return dbutil.WithTx(p.db, func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`
			INSERT INTO playlist_tracks (playlist_id, position, library_track_id)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for i, trackID := range trackIDs {
			if _, err := stmt.Exec(playlistID, nextPos+i, trackID); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveTrack removes the track at the given position from a playlist.
func (p *Playlists) RemoveTrack(playlistID int64, position int) error {
	return dbutil.WithTx(p.db, func(tx *sql.Tx) error {
		// Delete the track
		_, err := tx.Exec(`
			DELETE FROM playlist_tracks WHERE playlist_id = ? AND position = ?
		`, playlistID, position)
		if err != nil {
			return err
		}

		// Shift down positions after the deleted track
		_, err = tx.Exec(`
			UPDATE playlist_tracks
			SET position = position - 1
			WHERE playlist_id = ? AND position > ?
		`, playlistID, position)
		return err
	})
}

// RemoveTracks removes tracks at the given positions from a playlist.
func (p *Playlists) RemoveTracks(playlistID int64, positions []int) error {
	if len(positions) == 0 {
		return nil
	}

	// Sort positions in descending order to delete from end first
	sorted := make([]int, len(positions))
	copy(sorted, positions)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] > sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return dbutil.WithTx(p.db, func(tx *sql.Tx) error {
		for _, pos := range sorted {
			// Delete the track
			_, err := tx.Exec(`
				DELETE FROM playlist_tracks WHERE playlist_id = ? AND position = ?
			`, playlistID, pos)
			if err != nil {
				return err
			}

			// Shift down positions after the deleted track
			_, err = tx.Exec(`
				UPDATE playlist_tracks
				SET position = position - 1
				WHERE playlist_id = ? AND position > ?
			`, playlistID, pos)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// MoveIndices moves tracks at the given positions by delta.
// Returns the new positions after the move.
func (p *Playlists) MoveIndices(playlistID int64, positions []int, delta int) ([]int, error) {
	if len(positions) == 0 || delta == 0 {
		return positions, nil
	}

	count, err := p.TrackCount(playlistID)
	if err != nil {
		return nil, err
	}

	calc := newPositionCalculator(positions, count, delta)
	if !calc.canMove() {
		return positions, nil
	}

	err = dbutil.WithTx(p.db, func(tx *sql.Tx) error {
		sorted := calc.sortedPositions()

		// Move selected tracks to negative positions to avoid conflicts
		for i, pos := range sorted {
			if _, err := tx.Exec(`
				UPDATE playlist_tracks SET position = ?
				WHERE playlist_id = ? AND position = ?
			`, -(i + 1), playlistID, pos); err != nil {
				return err
			}
		}

		// Shift other tracks to fill gaps and make room
		// Must update one row at a time in correct order to avoid UNIQUE constraint violations:
		// - When shifting up (delta < 0): process from lowest to highest
		// - When shifting down (delta > 0): process from highest to lowest
		for _, r := range calc.shiftRanges() {
			if r.delta > 0 {
				// Shifting down: process from highest to lowest
				for pos := r.end - 1; pos >= r.start; pos-- {
					if _, err := tx.Exec(`
						UPDATE playlist_tracks SET position = position + ?
						WHERE playlist_id = ? AND position = ?
					`, r.delta, playlistID, pos); err != nil {
						return err
					}
				}
			} else {
				// Shifting up: process from lowest to highest
				for pos := r.start; pos < r.end; pos++ {
					if _, err := tx.Exec(`
						UPDATE playlist_tracks SET position = position + ?
						WHERE playlist_id = ? AND position = ?
					`, r.delta, playlistID, pos); err != nil {
						return err
					}
				}
			}
		}

		// Move selected tracks to their final positions
		for i, pos := range sorted {
			if _, err := tx.Exec(`
				UPDATE playlist_tracks SET position = ?
				WHERE playlist_id = ? AND position = ?
			`, pos+delta, playlistID, -(i + 1)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return calc.newPositions(positions), nil
}

// ClearTracks removes all tracks from a playlist.
func (p *Playlists) ClearTracks(playlistID int64) error {
	_, err := p.db.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, playlistID)
	return err
}

// SetTracks replaces all tracks in a playlist with the given track IDs.
func (p *Playlists) SetTracks(playlistID int64, trackIDs []int64) error {
	return dbutil.WithTx(p.db, func(tx *sql.Tx) error {
		// Clear existing tracks
		_, err := tx.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, playlistID)
		if err != nil {
			return err
		}

		// Insert new tracks
		stmt, err := tx.Prepare(`
			INSERT INTO playlist_tracks (playlist_id, position, library_track_id)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for i, trackID := range trackIDs {
			if _, err := stmt.Exec(playlistID, i, trackID); err != nil {
				return err
			}
		}
		return nil
	})
}

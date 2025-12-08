package playlists

import (
	"database/sql"
	"time"

	dbutil "github.com/llehouerou/waves/internal/db"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playlist"
)

// Folder represents a folder for organizing playlists.
type Folder struct {
	ID        int64
	ParentID  *int64
	Name      string
	CreatedAt int64
}

// Playlist represents a playlist metadata (without tracks).
type Playlist struct {
	ID         int64
	FolderID   *int64
	Name       string
	CreatedAt  int64
	LastUsedAt int64
}

// Playlists provides database operations for playlists.
type Playlists struct {
	db  *sql.DB
	lib *library.Library
}

// New creates a new Playlists instance.
func New(db *sql.DB, lib *library.Library) *Playlists {
	return &Playlists{db: db, lib: lib}
}

// CreateFolder creates a new folder.
func (p *Playlists) CreateFolder(parentID *int64, name string) (int64, error) {
	now := time.Now().Unix()
	result, err := p.db.Exec(`
		INSERT INTO playlist_folders (parent_id, name, created_at)
		VALUES (?, ?, ?)
	`, parentID, name, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// RenameFolder renames a folder.
func (p *Playlists) RenameFolder(id int64, name string) error {
	_, err := p.db.Exec(`UPDATE playlist_folders SET name = ? WHERE id = ?`, name, id)
	return err
}

// DeleteFolder deletes a folder and all its contents.
func (p *Playlists) DeleteFolder(id int64) error {
	_, err := p.db.Exec(`DELETE FROM playlist_folders WHERE id = ?`, id)
	return err
}

// Folders returns all folders with the given parent ID.
// Pass nil for parentID to get root-level folders.
func (p *Playlists) Folders(parentID *int64) ([]Folder, error) {
	var rows *sql.Rows
	var err error

	if parentID == nil {
		rows, err = p.db.Query(`
			SELECT id, parent_id, name, created_at
			FROM playlist_folders
			WHERE parent_id IS NULL
			ORDER BY name COLLATE NOCASE
		`)
	} else {
		rows, err = p.db.Query(`
			SELECT id, parent_id, name, created_at
			FROM playlist_folders
			WHERE parent_id = ?
			ORDER BY name COLLATE NOCASE
		`, *parentID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []Folder
	for rows.Next() {
		var f Folder
		var parentID sql.NullInt64
		if err := rows.Scan(&f.ID, &parentID, &f.Name, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.ParentID = dbutil.NullInt64ToPtr(parentID)
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

// FolderByID returns a folder by its ID.
func (p *Playlists) FolderByID(id int64) (*Folder, error) {
	row := p.db.QueryRow(`
		SELECT id, parent_id, name, created_at
		FROM playlist_folders
		WHERE id = ?
	`, id)

	var f Folder
	var parentID sql.NullInt64
	if err := row.Scan(&f.ID, &parentID, &f.Name, &f.CreatedAt); err != nil {
		return nil, err
	}
	f.ParentID = dbutil.NullInt64ToPtr(parentID)
	return &f, nil
}

// Create creates a new playlist.
func (p *Playlists) Create(folderID *int64, name string) (int64, error) {
	now := time.Now().Unix()
	result, err := p.db.Exec(`
		INSERT INTO playlists (folder_id, name, created_at, last_used_at)
		VALUES (?, ?, ?, ?)
	`, folderID, name, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Rename renames a playlist.
func (p *Playlists) Rename(id int64, name string) error {
	_, err := p.db.Exec(`UPDATE playlists SET name = ? WHERE id = ?`, name, id)
	return err
}

// Delete deletes a playlist and all its tracks.
func (p *Playlists) Delete(id int64) error {
	_, err := p.db.Exec(`DELETE FROM playlists WHERE id = ?`, id)
	return err
}

// List returns all playlists in the given folder.
// Pass nil for folderID to get root-level playlists.
func (p *Playlists) List(folderID *int64) ([]Playlist, error) {
	var rows *sql.Rows
	var err error

	if folderID == nil {
		rows, err = p.db.Query(`
			SELECT id, folder_id, name, created_at, last_used_at
			FROM playlists
			WHERE folder_id IS NULL
			ORDER BY name COLLATE NOCASE
		`)
	} else {
		rows, err = p.db.Query(`
			SELECT id, folder_id, name, created_at, last_used_at
			FROM playlists
			WHERE folder_id = ?
			ORDER BY name COLLATE NOCASE
		`, *folderID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []Playlist
	for rows.Next() {
		var pl Playlist
		var folderID sql.NullInt64
		if err := rows.Scan(&pl.ID, &folderID, &pl.Name, &pl.CreatedAt, &pl.LastUsedAt); err != nil {
			return nil, err
		}
		pl.FolderID = dbutil.NullInt64ToPtr(folderID)
		playlists = append(playlists, pl)
	}
	return playlists, rows.Err()
}

// Get returns a playlist by its ID.
func (p *Playlists) Get(id int64) (*Playlist, error) {
	row := p.db.QueryRow(`
		SELECT id, folder_id, name, created_at, last_used_at
		FROM playlists
		WHERE id = ?
	`, id)

	var pl Playlist
	var folderID sql.NullInt64
	if err := row.Scan(&pl.ID, &folderID, &pl.Name, &pl.CreatedAt, &pl.LastUsedAt); err != nil {
		return nil, err
	}
	pl.FolderID = dbutil.NullInt64ToPtr(folderID)
	return &pl, nil
}

// UpdateLastUsed updates the last_used_at timestamp for a playlist.
func (p *Playlists) UpdateLastUsed(id int64) error {
	now := time.Now().Unix()
	_, err := p.db.Exec(`UPDATE playlists SET last_used_at = ? WHERE id = ?`, now, id)
	return err
}

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
		for _, r := range calc.shiftRanges() {
			if _, err := tx.Exec(`
				UPDATE playlist_tracks SET position = position + ?
				WHERE playlist_id = ? AND position >= ? AND position < ? AND position >= 0
			`, r.delta, playlistID, r.start, r.end); err != nil {
				return err
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

// IsPlaylistEmpty returns true if the playlist has no tracks.
func (p *Playlists) IsPlaylistEmpty(playlistID int64) (bool, error) {
	count, err := p.TrackCount(playlistID)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// IsFolderEmpty returns true if the folder has no subfolders or playlists.
func (p *Playlists) IsFolderEmpty(folderID int64) (bool, error) {
	var count int
	err := p.db.QueryRow(`
		SELECT (SELECT COUNT(*) FROM playlist_folders WHERE parent_id = ?) +
		       (SELECT COUNT(*) FROM playlists WHERE folder_id = ?)
	`, folderID, folderID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
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

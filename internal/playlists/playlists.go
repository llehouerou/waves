package playlists

import (
	"database/sql"
	"time"

	dbutil "github.com/llehouerou/waves/internal/db"
	"github.com/llehouerou/waves/internal/library"
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

// IsPlaylistEmpty returns true if the playlist has no tracks.
func (p *Playlists) IsPlaylistEmpty(playlistID int64) (bool, error) {
	count, err := p.TrackCount(playlistID)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

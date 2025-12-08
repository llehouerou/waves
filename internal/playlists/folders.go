package playlists

import (
	"database/sql"
	"time"

	dbutil "github.com/llehouerou/waves/internal/db"
)

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

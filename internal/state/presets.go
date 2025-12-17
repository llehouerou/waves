// internal/state/presets.go
package state

import (
	"database/sql"
	"time"

	"github.com/llehouerou/waves/internal/ui/albumview"
)

// ListAlbumPresets returns all saved album view presets.
func (m *Manager) ListAlbumPresets() ([]albumview.Preset, error) {
	return listAlbumPresets(m.db)
}

// SaveAlbumPreset saves a new album view preset.
func (m *Manager) SaveAlbumPreset(name string, settings albumview.Settings) (int64, error) {
	return saveAlbumPreset(m.db, name, settings)
}

// DeleteAlbumPreset deletes an album view preset by ID.
func (m *Manager) DeleteAlbumPreset(id int64) error {
	return deleteAlbumPreset(m.db, id)
}

func listAlbumPresets(db *sql.DB) ([]albumview.Preset, error) {
	rows, err := db.Query(`
		SELECT id, name, group_fields, sort_criteria
		FROM album_view_presets
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var presets []albumview.Preset
	for rows.Next() {
		var p albumview.Preset
		var groupFieldsJSON, sortCriteriaJSON string

		if err := rows.Scan(&p.ID, &p.Name, &groupFieldsJSON, &sortCriteriaJSON); err != nil {
			return nil, err
		}

		settings, err := albumview.SettingsFromJSON(groupFieldsJSON, sortCriteriaJSON)
		if err != nil {
			// Skip invalid presets
			continue
		}
		p.Settings = settings

		presets = append(presets, p)
	}

	return presets, rows.Err()
}

func saveAlbumPreset(db *sql.DB, name string, settings albumview.Settings) (int64, error) {
	groupFieldsJSON, sortCriteriaJSON, err := settings.ToJSON()
	if err != nil {
		return 0, err
	}

	now := time.Now().Unix()

	// Try to update existing preset with same name
	result, err := db.Exec(`
		UPDATE album_view_presets
		SET group_fields = ?, sort_criteria = ?, updated_at = ?
		WHERE name = ?
	`, groupFieldsJSON, sortCriteriaJSON, now, name)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// Return existing ID
		var id int64
		err := db.QueryRow("SELECT id FROM album_view_presets WHERE name = ?", name).Scan(&id)
		return id, err
	}

	// Insert new preset
	result, err = db.Exec(`
		INSERT INTO album_view_presets (name, group_fields, sort_criteria, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, name, groupFieldsJSON, sortCriteriaJSON, now, now)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func deleteAlbumPreset(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM album_view_presets WHERE id = ?", id)
	return err
}

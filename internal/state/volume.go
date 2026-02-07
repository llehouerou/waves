package state

import "database/sql"

// VolumeState represents the saved volume state.
type VolumeState struct {
	Volume float64
	Muted  bool
}

// GetVolume returns the saved volume state.
func (m *Manager) GetVolume() (*VolumeState, error) {
	var volume float64
	var muted bool

	row := m.db.QueryRow(`SELECT volume, muted FROM queue_state WHERE id = 1`)
	err := row.Scan(&volume, &muted)
	if err == sql.ErrNoRows {
		return &VolumeState{Volume: 1.0, Muted: false}, nil
	}
	if err != nil {
		return nil, err
	}

	return &VolumeState{Volume: volume, Muted: muted}, nil
}

// SaveVolume persists the volume level to the database.
func (m *Manager) SaveVolume(volume float64, muted bool) error {
	_, err := m.db.Exec(`
		INSERT INTO queue_state (id, current_index, repeat_mode, shuffle, volume, muted)
		VALUES (1, -1, 0, 0, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			volume = excluded.volume,
			muted = excluded.muted
	`, volume, muted)
	return err
}

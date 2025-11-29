package state

import (
	"database/sql"
	"errors"
)

type NavigationState struct {
	CurrentPath  string
	SelectedName string
}

func getNavigation(db *sql.DB) (*NavigationState, error) {
	row := db.QueryRow(`
		SELECT current_path, selected_name FROM navigation_state WHERE id = 1
	`)

	var state NavigationState
	var selectedName sql.NullString

	err := row.Scan(&state.CurrentPath, &selectedName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no saved state is valid on first run
	}
	if err != nil {
		return nil, err
	}

	if selectedName.Valid {
		state.SelectedName = selectedName.String
	}

	return &state, nil
}

func saveNavigation(db *sql.DB, state NavigationState) error {
	_, err := db.Exec(`
		INSERT INTO navigation_state (id, current_path, selected_name)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			current_path = excluded.current_path,
			selected_name = excluded.selected_name
	`, state.CurrentPath, state.SelectedName)

	return err
}

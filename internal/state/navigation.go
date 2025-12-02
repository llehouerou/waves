package state

import (
	"database/sql"
	"errors"
)

type NavigationState struct {
	CurrentPath         string
	SelectedName        string
	ViewMode            string // "library", "file", or "playlists"
	LibrarySelectedID   string
	PlaylistsSelectedID string
}

func getNavigation(db *sql.DB) (*NavigationState, error) {
	row := db.QueryRow(`
		SELECT current_path, selected_name, view_mode, library_selected_id, playlists_selected_id
		FROM navigation_state WHERE id = 1
	`)

	var state NavigationState
	var selectedName, viewMode, librarySelectedID, playlistsSelectedID sql.NullString

	err := row.Scan(&state.CurrentPath, &selectedName, &viewMode, &librarySelectedID, &playlistsSelectedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no saved state is valid on first run
	}
	if err != nil {
		return nil, err
	}

	state.SelectedName = selectedName.String
	state.ViewMode = viewMode.String
	state.LibrarySelectedID = librarySelectedID.String
	state.PlaylistsSelectedID = playlistsSelectedID.String

	return &state, nil
}

func saveNavigation(db *sql.DB, state NavigationState) error {
	_, err := db.Exec(`
		INSERT INTO navigation_state (id, current_path, selected_name, view_mode, library_selected_id, playlists_selected_id)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			current_path = excluded.current_path,
			selected_name = excluded.selected_name,
			view_mode = excluded.view_mode,
			library_selected_id = excluded.library_selected_id,
			playlists_selected_id = excluded.playlists_selected_id
	`, state.CurrentPath, state.SelectedName, state.ViewMode, state.LibrarySelectedID, state.PlaylistsSelectedID)

	return err
}

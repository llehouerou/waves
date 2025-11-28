package state

import (
	"database/sql"
)

const currentSchemaVersion = 1

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		);

		CREATE TABLE IF NOT EXISTS navigation_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			current_path TEXT NOT NULL,
			selected_name TEXT
		);
	`)
	if err != nil {
		return err
	}

	// Set initial version if not exists
	_, err = db.Exec(`
		INSERT OR IGNORE INTO schema_version (version) VALUES (?)
	`, currentSchemaVersion)

	return err
}

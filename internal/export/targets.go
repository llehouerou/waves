package export

import (
	"database/sql"
	"fmt"
)

// FolderStructure defines how exported files are organized.
type FolderStructure string

const (
	FolderStructureFlat         FolderStructure = "flat"         // Artist - Album/01 - Track.mp3
	FolderStructureHierarchical FolderStructure = "hierarchical" // Artist/Album/01 - Track.mp3
	FolderStructureSingle       FolderStructure = "single"       // Artist - Album - 01 - Track.mp3
)

// Target represents a saved export destination.
type Target struct {
	ID              int64
	Name            string
	DeviceUUID      string
	DeviceLabel     string
	Subfolder       string
	FolderStructure FolderStructure
	CreatedAt       int64
}

// TargetRepository handles persistence of export targets.
type TargetRepository struct {
	db *sql.DB
}

// NewTargetRepository creates a new repository.
func NewTargetRepository(db *sql.DB) *TargetRepository {
	return &TargetRepository{db: db}
}

// Create adds a new target and returns its ID.
func (r *TargetRepository) Create(t Target) (int64, error) {
	result, err := r.db.Exec(`
		INSERT INTO export_targets (name, device_uuid, device_label, subfolder, folder_structure)
		VALUES (?, ?, ?, ?, ?)
	`, t.Name, t.DeviceUUID, t.DeviceLabel, t.Subfolder, t.FolderStructure)
	if err != nil {
		return 0, fmt.Errorf("insert target: %w", err)
	}
	return result.LastInsertId()
}

// List returns all saved targets.
func (r *TargetRepository) List() ([]Target, error) {
	rows, err := r.db.Query(`
		SELECT id, name, device_uuid, device_label, subfolder, folder_structure, created_at
		FROM export_targets
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query targets: %w", err)
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.Name, &t.DeviceUUID, &t.DeviceLabel,
			&t.Subfolder, &t.FolderStructure, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// Get returns a target by ID.
func (r *TargetRepository) Get(id int64) (Target, error) {
	var t Target
	err := r.db.QueryRow(`
		SELECT id, name, device_uuid, device_label, subfolder, folder_structure, created_at
		FROM export_targets
		WHERE id = ?
	`, id).Scan(&t.ID, &t.Name, &t.DeviceUUID, &t.DeviceLabel,
		&t.Subfolder, &t.FolderStructure, &t.CreatedAt)
	if err != nil {
		return Target{}, fmt.Errorf("get target: %w", err)
	}
	return t, nil
}

// Update modifies an existing target.
func (r *TargetRepository) Update(t Target) error {
	_, err := r.db.Exec(`
		UPDATE export_targets
		SET name = ?, device_uuid = ?, device_label = ?, subfolder = ?, folder_structure = ?
		WHERE id = ?
	`, t.Name, t.DeviceUUID, t.DeviceLabel, t.Subfolder, t.FolderStructure, t.ID)
	if err != nil {
		return fmt.Errorf("update target: %w", err)
	}
	return nil
}

// Delete removes a target.
func (r *TargetRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM export_targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	return nil
}

// FindByUUID returns the targets matching a device UUID.
func (r *TargetRepository) FindByUUID(uuid string) ([]Target, error) {
	rows, err := r.db.Query(`
		SELECT id, name, device_uuid, device_label, subfolder, folder_structure, created_at
		FROM export_targets
		WHERE device_uuid = ?
		ORDER BY name
	`, uuid)
	if err != nil {
		return nil, fmt.Errorf("query targets by uuid: %w", err)
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.Name, &t.DeviceUUID, &t.DeviceLabel,
			&t.Subfolder, &t.FolderStructure, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

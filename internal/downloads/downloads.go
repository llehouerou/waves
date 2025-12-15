// Package downloads provides download tracking and synchronization with slskd.
package downloads

import (
	"database/sql"
	"encoding/json"
	"time"

	dbutil "github.com/llehouerou/waves/internal/db"
	"github.com/llehouerou/waves/internal/musicbrainz"
)

// Status constants for download and file states.
const (
	StatusPending     = "pending"
	StatusDownloading = "downloading"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
)

// Download represents an album-level download job.
type Download struct {
	ID               int64
	MBReleaseGroupID string
	MBReleaseID      string // Specific release selected for import
	MBArtistName     string
	MBAlbumTitle     string
	MBReleaseYear    string
	// Full MusicBrainz data for importing (stored as JSON in DB)
	MBReleaseGroup   *musicbrainz.ReleaseGroup   // Release group metadata
	MBReleaseDetails *musicbrainz.ReleaseDetails // Full release with tracks
	SlskdUsername    string
	SlskdDirectory   string
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Files            []DownloadFile
}

// DownloadFile represents an individual file within a download.
type DownloadFile struct {
	ID             int64
	DownloadID     int64
	Filename       string
	Size           int64
	Status         string
	BytesRead      int64
	VerifiedOnDisk bool // True if file exists on disk with matching size
}

// Progress returns the download progress as completed files count and percentage.
func (d *Download) Progress() (completed, total int, percent float64) {
	total = len(d.Files)
	if total == 0 {
		return 0, 0, 0
	}

	var totalBytes, readBytes int64
	for _, f := range d.Files {
		totalBytes += f.Size
		readBytes += f.BytesRead
		if f.Status == StatusCompleted {
			completed++
		}
	}

	if totalBytes > 0 {
		percent = float64(readBytes) / float64(totalBytes) * 100
	}
	return completed, total, percent
}

// Manager provides database operations for downloads.
type Manager struct {
	db *sql.DB
}

// New creates a new Manager instance.
func New(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Create creates a new download with its files.
func (m *Manager) Create(download Download) (int64, error) {
	now := time.Now().Unix()

	tx, err := m.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize MusicBrainz data to JSON
	var releaseGroupJSON, releaseDetailsJSON []byte
	if download.MBReleaseGroup != nil {
		releaseGroupJSON, err = json.Marshal(download.MBReleaseGroup)
		if err != nil {
			return 0, err
		}
	}
	if download.MBReleaseDetails != nil {
		releaseDetailsJSON, err = json.Marshal(download.MBReleaseDetails)
		if err != nil {
			return 0, err
		}
	}

	result, err := tx.Exec(`
		INSERT INTO downloads (
			mb_release_group_id, mb_release_id, mb_artist_name, mb_album_title, mb_release_year,
			mb_release_group_json, mb_release_details_json,
			slskd_username, slskd_directory, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, download.MBReleaseGroupID, download.MBReleaseID, download.MBArtistName, download.MBAlbumTitle,
		download.MBReleaseYear, nullString(releaseGroupJSON), nullString(releaseDetailsJSON),
		download.SlskdUsername, download.SlskdDirectory, StatusPending, now, now)
	if err != nil {
		return 0, err
	}

	downloadID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO download_files (download_id, filename, size, status, bytes_read)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, f := range download.Files {
		_, err = stmt.Exec(downloadID, f.Filename, f.Size, StatusPending, 0)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return downloadID, nil
}

// List returns all downloads ordered by creation date (newest first).
func (m *Manager) List() ([]Download, error) {
	rows, err := m.db.Query(`
		SELECT id, mb_release_group_id, mb_release_id, mb_artist_name, mb_album_title, mb_release_year,
		       mb_release_group_json, mb_release_details_json,
		       slskd_username, slskd_directory, status, created_at, updated_at
		FROM downloads
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []Download
	for rows.Next() {
		var d Download
		var releaseID, releaseYear sql.NullString
		var releaseGroupJSON, releaseDetailsJSON sql.NullString
		var createdAt, updatedAt int64

		if err := rows.Scan(
			&d.ID, &d.MBReleaseGroupID, &releaseID, &d.MBArtistName, &d.MBAlbumTitle,
			&releaseYear, &releaseGroupJSON, &releaseDetailsJSON,
			&d.SlskdUsername, &d.SlskdDirectory, &d.Status,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		d.MBReleaseID = dbutil.NullStringValue(releaseID)
		d.MBReleaseYear = dbutil.NullStringValue(releaseYear)
		d.MBReleaseGroup = unmarshalReleaseGroup(releaseGroupJSON)
		d.MBReleaseDetails = unmarshalReleaseDetails(releaseDetailsJSON)
		d.CreatedAt = time.Unix(createdAt, 0)
		d.UpdatedAt = time.Unix(updatedAt, 0)

		// Load files for this download
		files, err := m.listFiles(d.ID)
		if err != nil {
			return nil, err
		}
		d.Files = files

		downloads = append(downloads, d)
	}

	return downloads, rows.Err()
}

// Get returns a download by its ID with all files.
func (m *Manager) Get(id int64) (*Download, error) {
	row := m.db.QueryRow(`
		SELECT id, mb_release_group_id, mb_release_id, mb_artist_name, mb_album_title, mb_release_year,
		       mb_release_group_json, mb_release_details_json,
		       slskd_username, slskd_directory, status, created_at, updated_at
		FROM downloads
		WHERE id = ?
	`, id)

	var d Download
	var releaseID, releaseYear sql.NullString
	var releaseGroupJSON, releaseDetailsJSON sql.NullString
	var createdAt, updatedAt int64

	if err := row.Scan(
		&d.ID, &d.MBReleaseGroupID, &releaseID, &d.MBArtistName, &d.MBAlbumTitle,
		&releaseYear, &releaseGroupJSON, &releaseDetailsJSON,
		&d.SlskdUsername, &d.SlskdDirectory, &d.Status,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}

	d.MBReleaseID = dbutil.NullStringValue(releaseID)
	d.MBReleaseYear = dbutil.NullStringValue(releaseYear)
	d.MBReleaseGroup = unmarshalReleaseGroup(releaseGroupJSON)
	d.MBReleaseDetails = unmarshalReleaseDetails(releaseDetailsJSON)
	d.CreatedAt = time.Unix(createdAt, 0)
	d.UpdatedAt = time.Unix(updatedAt, 0)

	files, err := m.listFiles(d.ID)
	if err != nil {
		return nil, err
	}
	d.Files = files

	return &d, nil
}

// Delete removes a download and all its files (via CASCADE).
func (m *Manager) Delete(id int64) error {
	_, err := m.db.Exec(`DELETE FROM downloads WHERE id = ?`, id)
	return err
}

// DeleteCompleted removes all completed downloads.
func (m *Manager) DeleteCompleted() error {
	_, err := m.db.Exec(`DELETE FROM downloads WHERE status = ?`, StatusCompleted)
	return err
}

// listFiles returns all files for a download.
func (m *Manager) listFiles(downloadID int64) ([]DownloadFile, error) {
	rows, err := m.db.Query(`
		SELECT id, download_id, filename, size, status, bytes_read, verified_on_disk
		FROM download_files
		WHERE download_id = ?
		ORDER BY filename
	`, downloadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []DownloadFile
	for rows.Next() {
		var f DownloadFile
		var verifiedOnDisk int
		if err := rows.Scan(&f.ID, &f.DownloadID, &f.Filename, &f.Size, &f.Status, &f.BytesRead, &verifiedOnDisk); err != nil {
			return nil, err
		}
		f.VerifiedOnDisk = verifiedOnDisk != 0
		files = append(files, f)
	}

	return files, rows.Err()
}

// nullString converts a byte slice to sql.NullString for JSON storage.
func nullString(b []byte) sql.NullString {
	if b == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

// unmarshalReleaseGroup deserializes a ReleaseGroup from JSON string.
func unmarshalReleaseGroup(s sql.NullString) *musicbrainz.ReleaseGroup {
	if !s.Valid || s.String == "" {
		return nil
	}
	var rg musicbrainz.ReleaseGroup
	if err := json.Unmarshal([]byte(s.String), &rg); err != nil {
		return nil
	}
	return &rg
}

// unmarshalReleaseDetails deserializes a ReleaseDetails from JSON string.
func unmarshalReleaseDetails(s sql.NullString) *musicbrainz.ReleaseDetails {
	if !s.Valid || s.String == "" {
		return nil
	}
	var rd musicbrainz.ReleaseDetails
	if err := json.Unmarshal([]byte(s.String), &rd); err != nil {
		return nil
	}
	return &rd
}

// VerifyOnDisk checks all downloads against files on disk and updates status.
// Files that exist on disk with matching size are marked as completed.
func (m *Manager) VerifyOnDisk(completedPath string) error {
	if completedPath == "" {
		return nil
	}

	downloads, err := m.List()
	if err != nil {
		return err
	}

	for i := range downloads {
		d := &downloads[i]

		// Verify each file
		results := VerifyDownloadFiles(completedPath, d)

		for _, f := range d.Files {
			result, ok := results[f.ID]
			if !ok {
				continue
			}

			// Mark file as verified if it exists on disk with matching size
			verified := result.Exists && result.SizeMatches
			if verified != f.VerifiedOnDisk {
				if err := m.updateFileVerified(f.ID, verified); err != nil {
					return err
				}
			}

			// Update file status to completed if verified and not already completed
			if verified && f.Status != StatusCompleted {
				if err := m.updateFileStatus(f.ID, StatusCompleted, f.Size); err != nil {
					return err
				}
			}
		}

		// Check if all files are now completed
		if err := m.updateDownloadStatusIfComplete(d.ID); err != nil {
			return err
		}
	}

	return nil
}

// updateFileStatus updates the status and bytes_read of a download file.
func (m *Manager) updateFileStatus(fileID int64, status string, bytesRead int64) error {
	_, err := m.db.Exec(`
		UPDATE download_files
		SET status = ?, bytes_read = ?
		WHERE id = ?
	`, status, bytesRead, fileID)
	return err
}

// updateFileVerified marks a file as verified on disk.
func (m *Manager) updateFileVerified(fileID int64, verified bool) error {
	verifiedInt := 0
	if verified {
		verifiedInt = 1
	}
	_, err := m.db.Exec(`
		UPDATE download_files
		SET verified_on_disk = ?
		WHERE id = ?
	`, verifiedInt, fileID)
	return err
}

// updateDownloadStatusIfComplete checks if all files are completed and updates download status.
func (m *Manager) updateDownloadStatusIfComplete(downloadID int64) error {
	// Count incomplete files
	var incompleteCount int
	err := m.db.QueryRow(`
		SELECT COUNT(*) FROM download_files
		WHERE download_id = ? AND status != ?
	`, downloadID, StatusCompleted).Scan(&incompleteCount)
	if err != nil {
		return err
	}

	// If all files are completed, mark download as completed
	if incompleteCount == 0 {
		_, err = m.db.Exec(`
			UPDATE downloads
			SET status = ?, updated_at = ?
			WHERE id = ?
		`, StatusCompleted, time.Now().Unix(), downloadID)
		return err
	}

	return nil
}

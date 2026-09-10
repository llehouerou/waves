package downloads

import (
	"strings"
	"time"

	"github.com/llehouerou/waves/internal/slskd"
)

// slskdKey creates a unique key for matching slskd downloads.
type slskdKey struct {
	Username string
	Filename string
}

// UpdateFromSlskd synchronizes local download state with slskd transfer status.
// It matches files by (username, filename) and updates status and progress.
func (m *Manager) UpdateFromSlskd(slskdDownloads []slskd.Download) error {
	// Build lookup map: (username, filename) -> slskd.Download.
	// slskd may report several records for the same file (e.g. an errored
	// transfer and its retry); keep the most advanced one.
	slskdMap := make(map[slskdKey]slskd.Download)
	for _, d := range slskdDownloads {
		key := slskdKey{Username: d.Username, Filename: d.Filename}
		if prev, ok := slskdMap[key]; ok && !supersedes(d, prev) {
			continue
		}
		slskdMap[key] = d
	}

	// Get all non-completed downloads from our database
	downloads, err := m.listPending()
	if err != nil {
		return err
	}

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()

	for i := range downloads {
		download := &downloads[i]
		var hasInProgress, hasCompleted, hasFailed bool
		allCompleted := true

		for j := range download.Files {
			file := &download.Files[j]
			key := slskdKey{Username: download.SlskdUsername, Filename: file.Filename}
			slskdDownload, found := slskdMap[key]

			newStatus, slskdState, bytesRead := file.Status, file.SlskdState, file.BytesRead
			if found {
				newStatus = mapSlskdState(slskdDownload.State)
				slskdState = slskdDownload.State
				bytesRead = slskdDownload.BytesTransferred
			}
			// Not found in slskd: keep current status.

			// Track aggregate state
			switch newStatus {
			case StatusCompleted:
				hasCompleted = true
			case StatusDownloading:
				hasInProgress = true
				allCompleted = false
			case StatusFailed:
				hasFailed = true
				allCompleted = false
			default:
				allCompleted = false
			}

			// Update file if anything changed
			if newStatus != file.Status || bytesRead != file.BytesRead || slskdState != file.SlskdState {
				_, err = tx.Exec(`
					UPDATE download_files
					SET status = ?, bytes_read = ?, slskd_state = ?
					WHERE id = ?
				`, newStatus, bytesRead, slskdState, file.ID)
				if err != nil {
					return err
				}
			}
		}

		// Determine overall download status
		var newDownloadStatus string
		switch {
		case allCompleted && hasCompleted:
			newDownloadStatus = StatusCompleted
		case hasFailed && !hasInProgress:
			newDownloadStatus = StatusFailed
		case hasInProgress || hasCompleted:
			newDownloadStatus = StatusDownloading
		default:
			newDownloadStatus = StatusPending
		}

		// Update download status if changed
		if newDownloadStatus != download.Status {
			_, err = tx.Exec(`
				UPDATE downloads SET status = ?, updated_at = ? WHERE id = ?
			`, newDownloadStatus, now, download.ID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// listPending returns all downloads that are not completed.
func (m *Manager) listPending() ([]Download, error) {
	rows, err := m.db.Query(`
		SELECT id, mb_release_group_id, mb_artist_name, mb_album_title, mb_release_year,
		       slskd_username, slskd_directory, status, created_at, updated_at
		FROM downloads
		WHERE status != ?
		ORDER BY created_at DESC
	`, StatusCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []Download
	for rows.Next() {
		var d Download
		var releaseYear, slskdDir string
		var createdAt, updatedAt int64

		if err := rows.Scan(
			&d.ID, &d.MBReleaseGroupID, &d.MBArtistName, &d.MBAlbumTitle,
			&releaseYear, &d.SlskdUsername, &slskdDir, &d.Status,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		d.MBReleaseYear = releaseYear
		d.SlskdDirectory = slskdDir
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

// listCompletedUnverified returns downloads where slskd reports completion
// but at least one file hasn't been verified on disk yet.
// This avoids disk I/O for in-progress downloads.
func (m *Manager) listCompletedUnverified() ([]Download, error) {
	// Get downloads that are completed (all files done per slskd)
	// but have at least one file not yet verified on disk
	rows, err := m.db.Query(`
		SELECT DISTINCT d.id, d.mb_release_group_id, d.mb_artist_name, d.mb_album_title, d.mb_release_year,
		       d.slskd_username, d.slskd_directory, d.status, d.created_at, d.updated_at
		FROM downloads d
		INNER JOIN download_files f ON d.id = f.download_id
		WHERE d.status = ? AND f.verified_on_disk = 0
		ORDER BY d.created_at DESC
	`, StatusCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []Download
	for rows.Next() {
		var d Download
		var releaseYear, slskdDir string
		var createdAt, updatedAt int64

		if err := rows.Scan(
			&d.ID, &d.MBReleaseGroupID, &d.MBArtistName, &d.MBAlbumTitle,
			&releaseYear, &d.SlskdUsername, &slskdDir, &d.Status,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		d.MBReleaseYear = releaseYear
		d.SlskdDirectory = slskdDir
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

// supersedes reports whether slskd record b should replace a for the same file:
// a non-failed record beats a failed one, otherwise more bytes transferred wins.
func supersedes(b, a slskd.Download) bool {
	aFailed := mapSlskdState(a.State) == StatusFailed
	bFailed := mapSlskdState(b.State) == StatusFailed
	if aFailed != bFailed {
		return aFailed
	}
	return b.BytesTransferred > a.BytesTransferred
}

// mapSlskdState converts slskd state string to our status constant.
// Terminal states are compound: "Completed, Succeeded", "Completed, Errored",
// "Completed, Rejected", ... so the failure reason must be checked before
// "Completed", and only "Succeeded" means success.
func mapSlskdState(state string) string {
	switch {
	case strings.Contains(state, "Errored"), strings.Contains(state, "Cancelled"),
		strings.Contains(state, "TimedOut"), strings.Contains(state, "Rejected"),
		strings.Contains(state, "Aborted"):
		return StatusFailed
	case strings.Contains(state, "Succeeded"):
		return StatusCompleted
	case strings.Contains(state, "Completed"):
		// Terminal without a success reason: fail closed.
		return StatusFailed
	case strings.Contains(state, "InProgress"), strings.Contains(state, "Initializing"),
		strings.Contains(state, "Requested"):
		return StatusDownloading
	default:
		// Queued, None, empty, unknown
		return StatusPending
	}
}

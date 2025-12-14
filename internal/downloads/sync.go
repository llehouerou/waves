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
	// Build lookup map: (username, filename) -> slskd.Download
	slskdMap := make(map[slskdKey]slskd.Download)
	for _, d := range slskdDownloads {
		key := slskdKey{Username: d.Username, Filename: d.Filename}
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

			var newStatus string
			var bytesRead int64

			if found {
				// Map slskd state to our status
				newStatus = mapSlskdState(slskdDownload.State)
				bytesRead = slskdDownload.BytesTransferred
			} else {
				// Not found in slskd - keep current status
				// If it was pending/downloading, it might have been removed or completed
				newStatus = file.Status
				bytesRead = file.BytesRead
			}

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

			// Update file if status or progress changed
			if newStatus != file.Status || bytesRead != file.BytesRead {
				_, err = tx.Exec(`
					UPDATE download_files
					SET status = ?, bytes_read = ?
					WHERE id = ?
				`, newStatus, bytesRead, file.ID)
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

// mapSlskdState converts slskd state string to our status constant.
// States can be compound like "Completed, Succeeded" or "Queued, Remotely".
func mapSlskdState(state string) string {
	// Check for completed states
	if strings.Contains(state, "Completed") || strings.Contains(state, "Succeeded") {
		return StatusCompleted
	}

	// Check for in-progress states
	if strings.Contains(state, "InProgress") || strings.Contains(state, "Initializing") ||
		strings.Contains(state, "Requested") {
		return StatusDownloading
	}

	// Check for failed states
	if strings.Contains(state, "Errored") || strings.Contains(state, "Cancelled") ||
		strings.Contains(state, "TimedOut") || strings.Contains(state, "Rejected") ||
		strings.Contains(state, "Aborted") {
		return StatusFailed
	}

	// Check for queued/pending states
	if strings.Contains(state, "Queued") || state == "None" || state == "" {
		return StatusPending
	}

	// Unknown state - treat as pending
	return StatusPending
}

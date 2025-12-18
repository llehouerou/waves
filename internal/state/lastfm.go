package state

import (
	"database/sql"
	"time"
)

// LastfmSession represents a stored Last.fm session.
type LastfmSession struct {
	Username   string
	SessionKey string
	LinkedAt   time.Time
}

// PendingScrobble represents a scrobble queued for retry.
type PendingScrobble struct {
	ID            int64
	Artist        string
	Track         string
	Album         string
	DurationSecs  int
	Timestamp     time.Time
	MBRecordingID string
	Attempts      int
	LastError     string
	CreatedAt     time.Time
}

// GetLastfmSession returns the stored Last.fm session, or nil if not linked.
func (m *Manager) GetLastfmSession() (*LastfmSession, error) {
	var username, sessionKey string
	var linkedAt int64

	err := m.db.QueryRow(`
		SELECT username, session_key, linked_at FROM lastfm_session WHERE id = 1
	`).Scan(&username, &sessionKey, &linkedAt)

	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // nil session means not linked, not an error
	}
	if err != nil {
		return nil, err
	}

	return &LastfmSession{
		Username:   username,
		SessionKey: sessionKey,
		LinkedAt:   time.Unix(linkedAt, 0),
	}, nil
}

// SaveLastfmSession stores the Last.fm session after successful authentication.
func (m *Manager) SaveLastfmSession(username, sessionKey string) error {
	now := time.Now().Unix()
	_, err := m.db.Exec(`
		INSERT INTO lastfm_session (id, username, session_key, linked_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			session_key = excluded.session_key,
			linked_at = excluded.linked_at
	`, username, sessionKey, now)
	return err
}

// DeleteLastfmSession removes the stored Last.fm session (unlink).
func (m *Manager) DeleteLastfmSession() error {
	_, err := m.db.Exec(`DELETE FROM lastfm_session WHERE id = 1`)
	return err
}

// AddPendingScrobble queues a scrobble for later submission.
func (m *Manager) AddPendingScrobble(s PendingScrobble) error {
	now := time.Now().Unix()
	_, err := m.db.Exec(`
		INSERT INTO lastfm_pending_scrobbles
		(artist, track, album, duration_seconds, timestamp, mb_recording_id, attempts, last_error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.Artist, s.Track, s.Album, s.DurationSecs, s.Timestamp.Unix(), s.MBRecordingID, 0, "", now)
	return err
}

// GetPendingScrobbles returns all pending scrobbles ordered by creation time.
func (m *Manager) GetPendingScrobbles() ([]PendingScrobble, error) {
	rows, err := m.db.Query(`
		SELECT id, artist, track, album, duration_seconds, timestamp, mb_recording_id, attempts, last_error, created_at
		FROM lastfm_pending_scrobbles
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scrobbles []PendingScrobble
	for rows.Next() {
		var s PendingScrobble
		var album sql.NullString
		var mbRecordingID sql.NullString
		var lastError sql.NullString
		var timestamp, createdAt int64

		err := rows.Scan(
			&s.ID, &s.Artist, &s.Track, &album, &s.DurationSecs,
			&timestamp, &mbRecordingID, &s.Attempts, &lastError, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		s.Album = album.String
		s.MBRecordingID = mbRecordingID.String
		s.LastError = lastError.String
		s.Timestamp = time.Unix(timestamp, 0)
		s.CreatedAt = time.Unix(createdAt, 0)

		scrobbles = append(scrobbles, s)
	}

	return scrobbles, rows.Err()
}

// DeletePendingScrobble removes a successfully submitted scrobble.
func (m *Manager) DeletePendingScrobble(id int64) error {
	_, err := m.db.Exec(`DELETE FROM lastfm_pending_scrobbles WHERE id = ?`, id)
	return err
}

// UpdatePendingScrobbleAttempt increments attempt count and sets error message.
func (m *Manager) UpdatePendingScrobbleAttempt(id int64, errMsg string) error {
	_, err := m.db.Exec(`
		UPDATE lastfm_pending_scrobbles
		SET attempts = attempts + 1, last_error = ?
		WHERE id = ?
	`, errMsg, id)
	return err
}

// DeleteOldPendingScrobbles removes pending scrobbles older than the given duration.
func (m *Manager) DeleteOldPendingScrobbles(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge).Unix()
	_, err := m.db.Exec(`DELETE FROM lastfm_pending_scrobbles WHERE created_at < ?`, cutoff)
	return err
}

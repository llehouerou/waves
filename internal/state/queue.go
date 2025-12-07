package state

import (
	"database/sql"
	"errors"

	dbutil "github.com/llehouerou/waves/internal/db"
)

// QueueTrack represents a track in the saved queue.
type QueueTrack struct {
	TrackID     int64
	Path        string
	Title       string
	Artist      string
	Album       string
	TrackNumber int
}

// QueueState represents the saved queue state.
type QueueState struct {
	CurrentIndex int
	RepeatMode   int
	Shuffle      bool
	Tracks       []QueueTrack
}

func getQueue(db *sql.DB) (*QueueState, error) {
	// Get queue state
	var currentIndex, repeatMode int
	var shuffle bool
	row := db.QueryRow(`SELECT current_index, repeat_mode, shuffle FROM queue_state WHERE id = 1`)
	err := row.Scan(&currentIndex, &repeatMode, &shuffle)
	if errors.Is(err, sql.ErrNoRows) {
		return &QueueState{CurrentIndex: -1}, nil
	}
	if err != nil {
		return nil, err
	}

	// Get tracks
	rows, err := db.Query(`
		SELECT track_id, path, title, artist, album, track_number
		FROM queue_tracks
		ORDER BY position
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []QueueTrack
	for rows.Next() {
		var t QueueTrack
		var trackID sql.NullInt64
		var artist, album sql.NullString
		var trackNumber sql.NullInt64

		err := rows.Scan(&trackID, &t.Path, &t.Title, &artist, &album, &trackNumber)
		if err != nil {
			return nil, err
		}

		t.TrackID = dbutil.NullInt64Value(trackID)
		t.Artist = dbutil.NullStringValue(artist)
		t.Album = dbutil.NullStringValue(album)
		t.TrackNumber = int(dbutil.NullInt64Value(trackNumber))
		tracks = append(tracks, t)
	}

	return &QueueState{
		CurrentIndex: currentIndex,
		RepeatMode:   repeatMode,
		Shuffle:      shuffle,
		Tracks:       tracks,
	}, nil
}

func saveQueue(sqlDB *sql.DB, state QueueState) error {
	return dbutil.WithTx(sqlDB, func(tx *sql.Tx) error {
		// Clear existing queue
		_, err := tx.Exec(`DELETE FROM queue_tracks`)
		if err != nil {
			return err
		}

		// Save queue state
		_, err = tx.Exec(`
			INSERT INTO queue_state (id, current_index, repeat_mode, shuffle)
			VALUES (1, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				current_index = excluded.current_index,
				repeat_mode = excluded.repeat_mode,
				shuffle = excluded.shuffle
		`, state.CurrentIndex, state.RepeatMode, state.Shuffle)
		if err != nil {
			return err
		}

		// Insert tracks
		stmt, err := tx.Prepare(`
			INSERT INTO queue_tracks (position, track_id, path, title, artist, album, track_number)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for i, t := range state.Tracks {
			var trackID any
			if t.TrackID > 0 {
				trackID = t.TrackID
			}
			_, err = stmt.Exec(i, trackID, t.Path, t.Title, t.Artist, t.Album, t.TrackNumber)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

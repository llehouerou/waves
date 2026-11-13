package library

import (
	"database/sql"
	"time"

	dbutil "github.com/llehouerou/waves/internal/db"
)

// Artists returns all unique album artists in the library.
func (l *Library) Artists() ([]string, error) {
	rows, err := l.db.Query(`
		SELECT DISTINCT album_artist FROM library_tracks ORDER BY album_artist COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var artist string
		if err := rows.Scan(&artist); err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}
	return artists, rows.Err()
}

// Albums returns all albums for a given album artist.
func (l *Library) Albums(albumArtist string) ([]Album, error) {
	rows, err := l.db.Query(`
		SELECT album, MAX(year) as year
		FROM library_tracks
		WHERE album_artist = ?
		GROUP BY album
		ORDER BY (year IS NULL OR year = 0), year, album COLLATE NOCASE
	`, albumArtist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var a Album
		var year sql.NullInt64
		if err := rows.Scan(&a.Name, &year); err != nil {
			return nil, err
		}
		a.Year = int(dbutil.NullInt64Value(year))
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

// Tracks returns all tracks for a given album artist and album.
func (l *Library) Tracks(albumArtist, album string) ([]Track, error) {
	rows, err := l.db.Query(`
		SELECT id, path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre, original_date, release_date, label
		FROM library_tracks
		WHERE album_artist = ? AND album = ?
		ORDER BY disc_number, track_number, title COLLATE NOCASE
	`, albumArtist, album)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var t Track
		var discNum, trackNum, year sql.NullInt64
		var genre, originalDate, releaseDate, label sql.NullString

		if err := rows.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
			&discNum, &trackNum, &year, &genre, &originalDate, &releaseDate, &label); err != nil {
			return nil, err
		}
		t.DiscNumber = int(dbutil.NullInt64Value(discNum))
		t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
		t.Year = int(dbutil.NullInt64Value(year))
		t.Genre = dbutil.NullStringValue(genre)
		t.OriginalDate = dbutil.NullStringValue(originalDate)
		t.ReleaseDate = dbutil.NullStringValue(releaseDate)
		t.Label = dbutil.NullStringValue(label)
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// TrackCount returns the total number of tracks in the library.
func (l *Library) TrackCount() (int, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM library_tracks`).Scan(&count)
	return count, err
}

// TrackByID returns a track by its ID.
func (l *Library) TrackByID(id int64) (*Track, error) {
	row := l.db.QueryRow(`
		SELECT id, path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre, original_date, release_date, label
		FROM library_tracks
		WHERE id = ?
	`, id)

	var t Track
	var discNum, trackNum, year sql.NullInt64
	var genre, originalDate, releaseDate, label sql.NullString

	err := row.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
		&discNum, &trackNum, &year, &genre, &originalDate, &releaseDate, &label)
	if err != nil {
		return nil, err
	}
	t.DiscNumber = int(dbutil.NullInt64Value(discNum))
	t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
	t.Year = int(dbutil.NullInt64Value(year))
	t.Genre = dbutil.NullStringValue(genre)
	t.OriginalDate = dbutil.NullStringValue(originalDate)
	t.ReleaseDate = dbutil.NullStringValue(releaseDate)
	t.Label = dbutil.NullStringValue(label)
	return &t, nil
}

// TrackByPath returns a track by its file path.
func (l *Library) TrackByPath(path string) (*Track, error) {
	return trackByPathWithExecutor(l.db, path)
}

// trackByPathWithExecutor is the internal implementation that accepts an executor.
func trackByPathWithExecutor(ex executor, path string) (*Track, error) {
	row := ex.QueryRow(`
		SELECT id, path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre, original_date, release_date, label
		FROM library_tracks
		WHERE path = ?
	`, path)

	var t Track
	var discNum, trackNum, year sql.NullInt64
	var genre, originalDate, releaseDate, label sql.NullString

	err := row.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
		&discNum, &trackNum, &year, &genre, &originalDate, &releaseDate, &label)
	if err != nil {
		return nil, err
	}
	t.DiscNumber = int(dbutil.NullInt64Value(discNum))
	t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
	t.Year = int(dbutil.NullInt64Value(year))
	t.Genre = dbutil.NullStringValue(genre)
	t.OriginalDate = dbutil.NullStringValue(originalDate)
	t.ReleaseDate = dbutil.NullStringValue(releaseDate)
	t.Label = dbutil.NullStringValue(label)
	return &t, nil
}

// ArtistCount returns the number of unique album artists.
func (l *Library) ArtistCount() (int, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(DISTINCT album_artist) FROM library_tracks`).Scan(&count)
	return count, err
}

// AlbumCount returns the number of unique albums.
func (l *Library) AlbumCount() (int, error) {
	var count int
	err := l.db.QueryRow(`SELECT COUNT(DISTINCT album_artist || album) FROM library_tracks`).Scan(&count)
	return count, err
}

// AlbumHasMultipleDiscs returns true if the album has tracks with disc number > 1.
func (l *Library) AlbumHasMultipleDiscs(albumArtist, album string) (bool, error) {
	var count int
	err := l.db.QueryRow(`
		SELECT COUNT(*) FROM library_tracks
		WHERE album_artist = ? AND album = ? AND disc_number > 1
	`, albumArtist, album).Scan(&count)
	return count > 0, err
}

// ArtistTracks returns all tracks for an artist, ordered by album year then disc/track number.
func (l *Library) ArtistTracks(albumArtist string) ([]Track, error) {
	rows, err := l.db.Query(`
		SELECT id, path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre, original_date, release_date, label
		FROM library_tracks
		WHERE album_artist = ?
		ORDER BY (year IS NULL OR year = 0), year, album COLLATE NOCASE, disc_number, track_number, title COLLATE NOCASE
	`, albumArtist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var t Track
		var discNum, trackNum, year sql.NullInt64
		var genre, originalDate, releaseDate, label sql.NullString

		if err := rows.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
			&discNum, &trackNum, &year, &genre, &originalDate, &releaseDate, &label); err != nil {
			return nil, err
		}
		t.DiscNumber = int(dbutil.NullInt64Value(discNum))
		t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
		t.Year = int(dbutil.NullInt64Value(year))
		t.Genre = dbutil.NullStringValue(genre)
		t.OriginalDate = dbutil.NullStringValue(originalDate)
		t.ReleaseDate = dbutil.NullStringValue(releaseDate)
		t.Label = dbutil.NullStringValue(label)
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (l *Library) albumTrackIDs(albumArtist, album string) ([]int64, error) {
	rows, err := l.db.Query(`
		SELECT id FROM library_tracks
		WHERE album_artist = ? AND album = ?
		ORDER BY disc_number, track_number, title COLLATE NOCASE
	`, albumArtist, album)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteTrack removes a track from the library by its ID.
// Also updates the FTS index incrementally within a transaction.
func (l *Library) DeleteTrack(id int64) error {
	// First fetch track info for FTS cleanup
	track, err := l.TrackByID(id)
	if err != nil {
		return err
	}

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Delete from library_tracks
	if _, err := tx.Exec(`DELETE FROM library_tracks WHERE id = ?`, id); err != nil {
		return err
	}
	if err := closePlaylistGaps(tx); err != nil {
		return err
	}

	// Remove from FTS index
	if err := removeTrackFromFTS(tx, track); err != nil {
		return err
	}

	return tx.Commit()
}

// closePlaylistGaps renumbers from 0, in the same order, the playlists a delete
// from library_tracks has just left a gap in: the cascade on
// playlist_tracks.library_track_id drops their entries, and the playlists
// address a track by its index in the playlist.
func closePlaylistGaps(tx *sql.Tx) error {
	// Two passes, through negative positions, so that no row ever takes a
	// position another one of its playlist still holds.
	if _, err := tx.Exec(`
		UPDATE playlist_tracks SET position = -1 - r.idx
		FROM (
			SELECT id, ROW_NUMBER() OVER (PARTITION BY playlist_id ORDER BY position) - 1 AS idx
			FROM playlist_tracks
			WHERE playlist_id IN (
				SELECT playlist_id FROM playlist_tracks
				GROUP BY playlist_id HAVING MAX(position) + 1 <> COUNT(*)
			)
		) AS r
		WHERE playlist_tracks.id = r.id
	`); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE playlist_tracks SET position = -1 - position WHERE position < 0`)
	return err
}

// AllAlbums returns all albums in the library with aggregated track data.
func (l *Library) AllAlbums() ([]AlbumEntry, error) {
	rows, err := l.db.Query(`
		SELECT
			album_artist,
			album,
			MAX(COALESCE(original_date, '')) as original_date,
			MAX(COALESCE(release_date, '')) as release_date,
			MIN(added_at) as added_at,
			COUNT(*) as track_count,
			(SELECT genre FROM library_tracks t2
			 WHERE t2.album_artist = t1.album_artist
			   AND t2.album = t1.album
			   AND genre IS NOT NULL AND genre != ''
			 GROUP BY genre ORDER BY COUNT(*) DESC LIMIT 1) as genre,
			(SELECT label FROM library_tracks t3
			 WHERE t3.album_artist = t1.album_artist
			   AND t3.album = t1.album
			   AND label IS NOT NULL AND label != ''
			 GROUP BY label ORDER BY COUNT(*) DESC LIMIT 1) as label
		FROM library_tracks t1
		GROUP BY album_artist, album
		ORDER BY original_date DESC, release_date DESC, added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []AlbumEntry
	for rows.Next() {
		var a AlbumEntry
		var addedAt int64
		var genre, label sql.NullString

		if err := rows.Scan(&a.AlbumArtist, &a.Album, &a.OriginalDate, &a.ReleaseDate, &addedAt, &a.TrackCount, &genre, &label); err != nil {
			return nil, err
		}
		a.AddedAt = time.Unix(addedAt, 0)
		a.Genre = dbutil.NullStringValue(genre)
		a.Label = dbutil.NullStringValue(label)
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

// AlbumTrackIDs returns track IDs for an album in playback order.
func (l *Library) AlbumTrackIDs(albumArtist, album string) ([]int64, error) {
	return l.albumTrackIDs(albumArtist, album)
}

// AlbumsForArtistNormalized returns a set of normalized album titles for a given artist.
// Used to check if a release group already exists in the library.
func (l *Library) AlbumsForArtistNormalized(albumArtist string) map[string]struct{} {
	result := make(map[string]struct{})

	rows, err := l.db.Query(`
		SELECT DISTINCT album FROM library_tracks WHERE album_artist = ?
	`, albumArtist)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var album string
		if err := rows.Scan(&album); err != nil {
			continue
		}
		normalized := NormalizeTitle(album)
		result[normalized] = struct{}{}
	}

	return result
}

package library

import (
	"database/sql"

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
		SELECT id, path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre
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
		var genre sql.NullString

		if err := rows.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
			&discNum, &trackNum, &year, &genre); err != nil {
			return nil, err
		}
		t.DiscNumber = int(dbutil.NullInt64Value(discNum))
		t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
		t.Year = int(dbutil.NullInt64Value(year))
		t.Genre = dbutil.NullStringValue(genre)
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
		SELECT id, path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre
		FROM library_tracks
		WHERE id = ?
	`, id)

	var t Track
	var discNum, trackNum, year sql.NullInt64
	var genre sql.NullString

	err := row.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
		&discNum, &trackNum, &year, &genre)
	if err != nil {
		return nil, err
	}
	t.DiscNumber = int(dbutil.NullInt64Value(discNum))
	t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
	t.Year = int(dbutil.NullInt64Value(year))
	t.Genre = dbutil.NullStringValue(genre)
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
		SELECT id, path, mtime, artist, album_artist, album, title, disc_number, track_number, year, genre
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
		var genre sql.NullString

		if err := rows.Scan(&t.ID, &t.Path, &t.Mtime, &t.Artist, &t.AlbumArtist, &t.Album, &t.Title,
			&discNum, &trackNum, &year, &genre); err != nil {
			return nil, err
		}
		t.DiscNumber = int(dbutil.NullInt64Value(discNum))
		t.TrackNumber = int(dbutil.NullInt64Value(trackNum))
		t.Year = int(dbutil.NullInt64Value(year))
		t.Genre = dbutil.NullStringValue(genre)
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// CollectTrackIDs returns track IDs for a given node.
// For artists: all tracks by that artist
// For albums: all tracks in that album
// For tracks: just that track ID
func (l *Library) CollectTrackIDs(node Node) ([]int64, error) {
	switch node.Level() {
	case LevelRoot:
		return nil, nil
	case LevelArtist:
		return l.artistTrackIDs(node.Artist())
	case LevelAlbum:
		return l.albumTrackIDs(node.Artist(), node.Album())
	case LevelTrack:
		if t := node.Track(); t != nil {
			return []int64{t.ID}, nil
		}
		return nil, nil
	}
	return nil, nil
}

func (l *Library) artistTrackIDs(albumArtist string) ([]int64, error) {
	rows, err := l.db.Query(`
		SELECT id FROM library_tracks
		WHERE album_artist = ?
		ORDER BY (year IS NULL OR year = 0), year, album COLLATE NOCASE, disc_number, track_number, title COLLATE NOCASE
	`, albumArtist)
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
func (l *Library) DeleteTrack(id int64) error {
	_, err := l.db.Exec(`DELETE FROM library_tracks WHERE id = ?`, id)
	return err
}

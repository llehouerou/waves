package state

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// holdTwoConnections opens a database with OpenAt and holds two connections of
// its pool at once: a pragma run on one of them only would show on the other.
func holdTwoConnections(t *testing.T) (*sql.DB, [2]*sql.Conn) {
	t.Helper()
	m, err := OpenAt(filepath.Join(t.TempDir(), "waves.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { m.Close() })

	var conns [2]*sql.Conn
	for i := range conns {
		conn, err := m.DB().Conn(context.Background())
		if err != nil {
			t.Fatalf("conn %d: %v", i, err)
		}
		t.Cleanup(func() { conn.Close() })
		conns[i] = conn
	}
	return m.DB(), conns
}

func exec(t *testing.T, conn *sql.Conn, query string, args ...any) int64 {
	t.Helper()
	res, err := conn.ExecContext(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func insertLibraryTrack(t *testing.T, conn *sql.Conn, path string) int64 {
	t.Helper()
	return exec(t, conn, `
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, added_at, updated_at)
		VALUES (?, 0, 'Artist', 'Artist', 'Album', ?, 0, 0)
	`, path, path)
}

func TestOpenAt_EveryConnectionEnforcesForeignKeysAndSynchronousNormal(t *testing.T) {
	_, conns := holdTwoConnections(t)

	for i, conn := range conns {
		var foreignKeys, synchronous int
		if err := conn.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
			t.Fatalf("conn %d: foreign_keys: %v", i, err)
		}
		if err := conn.QueryRowContext(context.Background(), `PRAGMA synchronous`).Scan(&synchronous); err != nil {
			t.Fatalf("conn %d: synchronous: %v", i, err)
		}
		if foreignKeys != 1 || synchronous != 1 {
			t.Errorf("conn %d: foreign_keys = %d, synchronous = %d, want 1 and 1", i, foreignKeys, synchronous)
		}
	}
}

func TestOpenAt_EveryConnectionRefusesAPlaylistTrackPointingNowhere(t *testing.T) {
	_, conns := holdTwoConnections(t)
	track := insertLibraryTrack(t, conns[0], "/music/a.mp3")

	for i, conn := range conns {
		if _, err := conn.ExecContext(context.Background(),
			`INSERT INTO playlist_tracks (playlist_id, position, library_track_id) VALUES (1, ?, 999)`, i,
		); err == nil {
			t.Errorf("conn %d: a playlist track of a library track that does not exist was accepted", i)
		}
		if _, err := conn.ExecContext(context.Background(),
			`INSERT INTO playlist_tracks (playlist_id, position, library_track_id) VALUES (999, ?, ?)`, i, track,
		); err == nil {
			t.Errorf("conn %d: a playlist track of a playlist that does not exist was accepted", i)
		}
	}
}

func TestOpenAt_EveryConnectionCascadesDeletes(t *testing.T) {
	for i := range 2 {
		t.Run(fmt.Sprintf("conn %d", i), func(t *testing.T) {
			db, conns := holdTwoConnections(t)
			conn := conns[i]

			track := insertLibraryTrack(t, conn, "/music/a.mp3")
			folder := exec(t, conn, `INSERT INTO playlist_folders (parent_id, name, created_at) VALUES (NULL, 'Folder', 0)`)
			sub := exec(t, conn, `INSERT INTO playlist_folders (parent_id, name, created_at) VALUES (?, 'Sub', 0)`, folder)
			inSub := exec(t, conn, `INSERT INTO playlists (folder_id, name, created_at, last_used_at) VALUES (?, 'In sub', 0, 0)`, sub)
			atRoot := exec(t, conn, `INSERT INTO playlists (folder_id, name, created_at, last_used_at) VALUES (NULL, 'At root', 0, 0)`)
			for _, playlist := range []int64{inSub, atRoot} {
				exec(t, conn, `INSERT INTO playlist_tracks (playlist_id, position, library_track_id) VALUES (?, 0, ?)`, playlist, track)
			}
			download := exec(t, conn, `
				INSERT INTO downloads (mb_release_group_id, mb_artist_name, mb_album_title, slskd_username, slskd_directory, created_at, updated_at)
				VALUES ('rg', 'Artist', 'Album', 'user', 'dir', 0, 0)
			`)
			exec(t, conn, `INSERT INTO download_files (download_id, filename, size) VALUES (?, 'a.mp3', 1)`, download)

			exec(t, conn, `DELETE FROM playlist_folders WHERE id = ?`, folder)
			exec(t, conn, `DELETE FROM playlists WHERE id = ?`, atRoot)
			exec(t, conn, `DELETE FROM downloads WHERE id = ?`, download)

			for table, want := range map[string]int{
				"playlist_folders": 0,
				"playlists":        1, // Favorites
				"playlist_tracks":  0,
				"download_files":   0,
			} {
				if got := count(t, db, table); got != want {
					t.Errorf("%s: %d rows, want %d", table, got, want)
				}
			}
		})
	}
}

// snapshot reads the tables the orphan cleanup touches, in a stable order.
func snapshot(t *testing.T, db *sql.DB) []string {
	t.Helper()
	r, err := db.Query(`
		SELECT 'file', id, download_id, 0 FROM download_files
		UNION ALL SELECT 'folder', id, COALESCE(parent_id, 0), 0 FROM playlist_folders
		UNION ALL SELECT 'playlist', id, COALESCE(folder_id, 0), 0 FROM playlists
		UNION ALL SELECT 'track', playlist_id, position, library_track_id FROM playlist_tracks
		ORDER BY 1, 2, 3
	`)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	defer r.Close()
	var rows []string
	for r.Next() {
		var kind string
		var a, b, c int64
		if err := r.Scan(&kind, &a, &b, &c); err != nil {
			t.Fatalf("scan: %v", err)
		}
		rows = append(rows, fmt.Sprintf("%s %d %d %d", kind, a, b, c))
	}
	return rows
}

// An older database, written while only one connection enforced foreign keys,
// comes out of OpenAt with no orphan and with contiguous playlist positions.
func TestOpenAt_RemovesTheOrphansOfAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waves.db")
	m, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	m.Close()

	// Seed the orphans on a connection without foreign keys, as version 19.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	raw.SetMaxOpenConns(1)
	for _, query := range []string{
		`DELETE FROM schema_version`,
		`INSERT INTO schema_version (version) VALUES (19)`,
		`INSERT INTO library_tracks (id, path, mtime, artist, album_artist, album, title, added_at, updated_at) VALUES
			(10, '/a', 0, '', '', '', '', 0, 0), (11, '/b', 0, '', '', '', '', 0, 0), (12, '/c', 0, '', '', '', '', 0, 0)`,
		`INSERT INTO playlist_folders (id, parent_id, name, created_at) VALUES
			(20, NULL, 'Kept', 0), (21, 999, 'Orphan', 0), (22, 21, 'Under orphan', 0)`,
		`INSERT INTO playlists (id, folder_id, name, created_at, last_used_at) VALUES
			(30, 20, 'Kept', 0, 0), (31, 998, 'Orphan', 0, 0), (32, 22, 'Under orphan', 0, 0)`,
		`INSERT INTO playlist_tracks (playlist_id, position, library_track_id) VALUES
			(30, 0, 10), (30, 2, 999), (30, 5, 11), (30, 7, 12),
			(1, 3, 12), (1, 4, NULL), (1, 9, 10),
			(31, 0, 10), (32, 0, 10), (997, 0, 10)`,
		`INSERT INTO downloads (id, mb_release_group_id, mb_artist_name, mb_album_title, slskd_username, slskd_directory, created_at, updated_at)
			VALUES (40, 'rg', '', '', '', '', 0, 0)`,
		`INSERT INTO download_files (id, download_id, filename, size) VALUES (50, 40, 'kept', 1), (51, 996, 'orphan', 1)`,
	} {
		if _, err := raw.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	raw.Close()

	m, err = OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt after seeding: %v", err)
	}
	got := snapshot(t, m.DB())
	m.Close()

	want := []string{
		"file 50 40 0",
		"folder 20 0 0",
		"playlist 1 0 0",
		"playlist 30 20 0",
		"track 1 0 12",
		"track 1 1 10",
		"track 30 0 10",
		"track 30 1 11",
		"track 30 2 12",
	}
	if !slices.Equal(got, want) {
		t.Errorf("after OpenAt:\n got %q\nwant %q", got, want)
	}

	m, err = OpenAt(path)
	if err != nil {
		t.Fatalf("second OpenAt: %v", err)
	}
	again := snapshot(t, m.DB())
	m.Close()
	if !slices.Equal(again, want) {
		t.Errorf("a second OpenAt changed the database:\n got %q\nwant %q", again, want)
	}
}

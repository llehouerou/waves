package state

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// earlierDatabase returns the path of a database as version 20 left it,
// holding the sources given, in that order, and what queries insert.
func earlierDatabase(t *testing.T, sources []string, queries ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "waves.db")
	m, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	m.Close()

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()
	for _, query := range []string{`DELETE FROM schema_version`, `INSERT INTO schema_version (version) VALUES (20)`} {
		if _, err := raw.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	for i, source := range sources {
		if _, err := raw.Exec(`INSERT INTO library_sources (path, added_at) VALUES (?, ?)`, source, 1000+i); err != nil {
			t.Fatalf("insert source %q: %v", source, err)
		}
	}
	for _, query := range queries {
		if _, err := raw.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	return path
}

// opened opens the database at path, reads it with read and closes it.
func opened[T any](t *testing.T, path string, read func(*testing.T, *sql.DB) T) T {
	t.Helper()
	m, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer m.Close()
	return read(t, m.DB())
}

// sourcePaths reads the paths of the sources, in the order Library.Sources
// lists them.
func sourcePaths(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows := sourceRows(t, db)
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		paths = append(paths, row.path)
	}
	return paths
}

type sourceRow struct {
	id      int64
	path    string
	addedAt int64
}

func sourceRows(t *testing.T, db *sql.DB) []sourceRow {
	t.Helper()
	r, err := db.Query(`SELECT id, path, added_at FROM library_sources ORDER BY added_at`)
	if err != nil {
		t.Fatalf("read sources: %v", err)
	}
	defer r.Close()
	var rows []sourceRow
	for r.Next() {
		var row sourceRow
		if err := r.Scan(&row.id, &row.path, &row.addedAt); err != nil {
			t.Fatalf("scan: %v", err)
		}
		rows = append(rows, row)
	}
	return rows
}

// tracksAndEntries reads the tracks with their IDs and the playlist entries.
func tracksAndEntries(t *testing.T, db *sql.DB) []string {
	t.Helper()
	r, err := db.Query(`
		SELECT 'track', id, path, 0 FROM library_tracks
		UNION ALL SELECT 'entry', playlist_id, CAST(library_track_id AS TEXT), position FROM playlist_tracks
		ORDER BY 1, 2, 3, 4
	`)
	if err != nil {
		t.Fatalf("read tracks: %v", err)
	}
	defer r.Close()
	var rows []string
	for r.Next() {
		var kind, text string
		var id, position int64
		if err := r.Scan(&kind, &id, &text, &position); err != nil {
			t.Fatalf("scan: %v", err)
		}
		rows = append(rows, fmt.Sprintf("%s %d %s %d", kind, id, text, position))
	}
	return rows
}

// tracksUnderMusic are tracks under /x/music, one of them in Favorites and in
// a playlist.
var tracksUnderMusic = []string{
	`INSERT INTO library_tracks (id, path, mtime, artist, album_artist, album, title, added_at, updated_at) VALUES
		(10, '/x/music/a/a.mp3', 0, '', '', '', '', 0, 0), (11, '/x/music/sub/b.mp3', 0, '', '', '', '', 0, 0)`,
	`INSERT INTO playlists (id, folder_id, name, created_at, last_used_at) VALUES (30, NULL, 'Mix', 0, 0)`,
	`INSERT INTO playlist_tracks (playlist_id, position, library_track_id) VALUES (1, 0, 11), (30, 0, 11), (30, 1, 10)`,
}

var wantTracksUnderMusic = []string{
	"entry 1 11 0",
	"entry 30 10 1",
	"entry 30 11 0",
	"track 10 /x/music/a/a.mp3 0",
	"track 11 /x/music/sub/b.mp3 0",
}

func TestOpenAt_CleansTheSourcesOfAnEarlierDatabase(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sources []string
		want    []string
	}{
		{"trailing separator", []string{"/x/music/"}, []string{"/x/music"}},
		{
			"unclean spellings, in their order",
			[]string{"/x//music", "/y/music/.", "/z/music/sub/.."},
			[]string{"/x/music", "/y/music", "/z/music"},
		},
		{"unclean double of a clean source", []string{"/x/music/", "/x/music"}, []string{"/x/music"}},
		{"two unclean doubles", []string{"/x/music/", "/x//music"}, []string{"/x/music"}},
		{"source inside another", []string{"/x/music/", "/x/music/sub/"}, []string{"/x/music", "/x/music/sub"}},
		{"relative sources", []string{"music/", "./music"}, []string{"music/", "./music"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := earlierDatabase(t, tc.sources, tracksUnderMusic...)

			if got := opened(t, path, sourcePaths); !slices.Equal(got, tc.want) {
				t.Errorf("sources = %q, want %q", got, tc.want)
			}
			if got := opened(t, path, tracksAndEntries); !slices.Equal(got, wantTracksUnderMusic) {
				t.Errorf("tracks and playlist entries:\n got %q\nwant %q", got, wantTracksUnderMusic)
			}
		})
	}
}

func TestOpenAt_LeavesTheCleanSourcesOfAnEarlierDatabaseAsTheyAre(t *testing.T) {
	path := earlierDatabase(t, []string{"/x/music", "/y/music", "music"})
	want := []sourceRow{{1, "/x/music", 1000}, {2, "/y/music", 1001}, {3, "music", 1002}}

	if got := opened(t, path, sourceRows); !slices.Equal(got, want) {
		t.Errorf("sources = %v, want %v", got, want)
	}
}

func TestOpenAt_ASecondOpeningLeavesTheCleanedSourcesAsTheyAre(t *testing.T) {
	path := earlierDatabase(t, []string{"/x/music/", "/y//music", "/x/music"})
	want := []sourceRow{{2, "/y/music", 1001}, {3, "/x/music", 1002}}

	if got := opened(t, path, sourceRows); !slices.Equal(got, want) {
		t.Errorf("sources after the first opening = %v, want %v", got, want)
	}
	if got := opened(t, path, sourceRows); !slices.Equal(got, want) {
		t.Errorf("sources after the second opening = %v, want %v", got, want)
	}
}

// MigrateSources adds every source of the configuration within one second:
// sources added at the same time are taken in the order they were added.
func TestOpenAt_KeepsTheFirstAddedOfDoublesAddedAtTheSameTime(t *testing.T) {
	path := earlierDatabase(t, nil,
		`INSERT INTO library_sources (id, path, added_at) VALUES (1, '/y/music/', 1000), (2, '/x/music/', 1000), (3, '/x//music', 1000)`)
	want := []sourceRow{{1, "/y/music", 1000}, {2, "/x/music", 1000}}

	m, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer m.Close()
	r, err := m.DB().Query(`SELECT id, path, added_at FROM library_sources ORDER BY id`)
	if err != nil {
		t.Fatalf("read sources: %v", err)
	}
	defer r.Close()
	var got []sourceRow
	for r.Next() {
		var row sourceRow
		if err := r.Scan(&row.id, &row.path, &row.addedAt); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, row)
	}
	if !slices.Equal(got, want) {
		t.Errorf("sources = %v, want %v", got, want)
	}
}

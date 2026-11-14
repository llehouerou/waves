package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/playlistnav"
)

// seedPlaylistOfAlbum seeds a library source at source holding one album of
// three tracks, Album Track 1 to 3, and a playlist P holding them in that
// order.
func seedPlaylistOfAlbum(t *testing.T, db *sql.DB, source string) {
	t.Helper()
	for i := 1; i <= 3; i++ {
		if _, err := db.Exec(`
			INSERT INTO library_tracks
				(path, mtime, artist, album_artist, album, title,
				 disc_number, track_number, year, added_at, updated_at)
			VALUES (?, 1000, 'Album Artist', 'Album Artist', 'Album Title', ?, 1, ?, 2024, 1000, 1000)
		`, filepath.Join(source, "Album Title", strconv.Itoa(i)+".mp3"), "Album Track "+strconv.Itoa(i), i); err != nil {
			t.Fatalf("insert track: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO library_sources (path, added_at) VALUES (?, 1)`, source); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	res, err := db.Exec(`INSERT INTO playlists (name, created_at, last_used_at) VALUES ('P', 1, 1)`)
	if err != nil {
		t.Fatalf("insert playlist: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("playlist id: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if _, err := db.Exec(`
			INSERT INTO playlist_tracks (playlist_id, position, library_track_id)
			SELECT ?, ?, id FROM library_tracks WHERE title = ?
		`, id, i-1, "Album Track "+strconv.Itoa(i)); err != nil {
			t.Fatalf("insert playlist track: %v", err)
		}
	}
}

// openPlaylistP opens P in the Playlists view, after Favorites, with the
// cursor on its second track.
func openPlaylistP(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF3})
	m, _ = updateModel(t, m, keyMsg("j"))
	m, _ = updateModel(t, m, keyMsg("l"))
	m, _ = updateModel(t, m, keyMsg("j"))
	if got := shownTitles(m); !slices.Equal(got, []string{"Album Track 1", "Album Track 2", "Album Track 3"}) {
		t.Fatalf("P shows %v before the change, want its three tracks", got)
	}
	return m
}

// shownTitles returns the titles of the tracks the playlist navigator lists.
func shownTitles(m Model) []string {
	items := m.Navigation.PlaylistNav().CurrentItems()
	titles := make([]string, 0, len(items))
	for _, n := range items {
		titles = append(titles, n.Track().Title)
	}
	return titles
}

// playlistTitles returns the titles of the tracks of the playlist the
// navigator is in, as the core has them.
func playlistTitles(t *testing.T, m Model) []string {
	t.Helper()
	id := m.Navigation.PlaylistNav().Current().PlaylistID()
	if id == nil {
		t.Fatal("the playlist navigator is not in a playlist")
	}
	tracks, err := m.Core.Playlists().Tracks(*id)
	if err != nil {
		t.Fatalf("playlist tracks: %v", err)
	}
	titles := make([]string, 0, len(tracks))
	for i := range tracks {
		titles = append(titles, tracks[i].Title)
	}
	return titles
}

// assertStillInP checks that the playlist navigator is still in P, its
// cursor on one of its rows or on none when it is empty, and that no error
// is shown.
func assertStillInP(t *testing.T, m Model) {
	t.Helper()
	nav := m.Navigation.PlaylistNav()
	if got := nav.Current().DisplayName(); got != "P" {
		t.Errorf("the playlist navigator is in %q, want in P", got)
	}
	selected := nav.Selected()
	if len(nav.CurrentItems()) == 0 {
		if selected != nil {
			t.Errorf("selected %q in an empty P, want nothing", selected.DisplayName())
		}
	} else if selected == nil || selected.Level() != playlistnav.LevelTrack {
		t.Errorf("selected %v in P, want one of its tracks", selected)
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error shown: %s", msg)
	}
}

func TestLibraryDelete_ReloadsThePlaylistsSoDRemovesTheTrackUnderTheCursor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		moves  []string // in the confirmation, to its option
		gone   bool     // the file is deleted
		up     bool     // d on the row above the cursor
		target string   // the track on the row d is pressed on
		left   string   // the track d leaves in P
	}{
		{name: "remove from library", target: "Album Track 3", left: "Album Track 2"},
		{name: "delete from disk", moves: []string{"j"}, gone: true, up: true, target: "Album Track 2", left: "Album Track 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := t.TempDir()
			if err := os.MkdirAll(filepath.Join(source, "Album Title"), 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			for i := 1; i <= 3; i++ {
				if err := os.WriteFile(filepath.Join(source, "Album Title", strconv.Itoa(i)+".mp3"), nil, 0o600); err != nil {
					t.Fatalf("write file: %v", err)
				}
			}
			m := newLibraryTestModel(t, func(db *sql.DB) { seedPlaylistOfAlbum(t, db, source) })
			m = openPlaylistP(t, m)

			// Delete Album Track 1 from the Library view.
			m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF1})
			m, _ = updateModel(t, m, keyMsg("l"))
			m, _ = updateModel(t, m, keyMsg("l"))
			m, _ = updateModel(t, m, keyMsg("d"))
			for _, k := range tc.moves {
				m, _ = updateModel(t, m, keyMsg(k))
			}
			m, cmd := updateModel(t, m, keyMsg(keyEnter))
			m, _ = updateModel(t, m, cmd())

			if got := playlistTitles(t, m); !slices.Equal(got, []string{"Album Track 2", "Album Track 3"}) {
				t.Fatalf("P holds %v after the deletion, want Album Track 2 and 3", got)
			}
			if _, err := os.Stat(filepath.Join(source, "Album Title", "1.mp3")); os.IsNotExist(err) != tc.gone {
				t.Errorf("file of Album Track 1: stat error %v, want it deleted: %v", err, tc.gone)
			}
			// Reloaded while the Library view is on screen.
			if got := shownTitles(m); !slices.Equal(got, []string{"Album Track 2", "Album Track 3"}) {
				t.Errorf("P shows %v after the deletion, want Album Track 2 and 3", got)
			}
			assertStillInP(t, m)
			// The cursor keeps its row, the second one, which now shows Album Track 3.
			if got := m.Navigation.PlaylistNav().Selected().Track().Title; got != "Album Track 3" {
				t.Fatalf("the cursor is on %q after the deletion, want on Album Track 3", got)
			}

			// d removes the track on the row it is pressed on, not the next one.
			m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF3})
			if tc.up {
				m, _ = updateModel(t, m, keyMsg("k"))
			}
			if got := m.Navigation.PlaylistNav().Selected().Track().Title; got != tc.target {
				t.Fatalf("the cursor is on %q, want on %s", got, tc.target)
			}
			m, _ = updateModel(t, m, keyMsg("d"))
			if got := playlistTitles(t, m); !slices.Equal(got, []string{tc.left}) {
				t.Errorf("P holds %v after d on %s, want %s only", got, tc.target, tc.left)
			}
		})
	}
}

func TestLibrarySourceRemoval_ReloadsThePlaylists(t *testing.T) {
	source := t.TempDir()
	m := newLibraryTestModel(t, func(db *sql.DB) { seedPlaylistOfAlbum(t, db, source) })
	m = openPlaylistP(t, m)

	// Remove the source from the library sources popup, over the Library view.
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF1})
	m, _ = updateModel(t, m, keyMsg("f"))
	m, _ = updateModel(t, m, keyMsg("p"))
	m, cmd := updateModel(t, m, keyMsg("d"))
	m, _ = updateModel(t, m, cmd())
	m, cmd = updateModel(t, m, keyMsg("y"))
	m, _ = updateModel(t, m, cmd())

	if got := playlistTitles(t, m); len(got) != 0 {
		t.Fatalf("P holds %v after the source removal, want nothing", got)
	}
	if got := shownTitles(m); len(got) != 0 {
		t.Errorf("P shows %v after the source removal, want nothing", got)
	}
	assertStillInP(t, m)
}

func TestLibraryScan_ReloadsThePlaylists(t *testing.T) {
	// A source that is there, with no music file left in it: an empty root
	// would be taken for an unmounted source, whose tracks a scan keeps.
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "cover.jpg"), nil, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	m := newLibraryTestModel(t, func(db *sql.DB) { seedPlaylistOfAlbum(t, db, source) })
	m = openPlaylistP(t, m)

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF1})
	m, _ = updateModel(t, m, keyMsg("f"))
	m, _ = updateModel(t, m, keyMsg("r"))
	m = pumpUntil[LibraryScanCompleteMsg](t, m)

	if got := playlistTitles(t, m); len(got) != 0 {
		t.Fatalf("P holds %v after the scan, want nothing", got)
	}
	if got := shownTitles(m); len(got) != 0 {
		t.Errorf("P shows %v after the scan, want nothing", got)
	}
	assertStillInP(t, m)
}

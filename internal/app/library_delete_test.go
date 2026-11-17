package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// seededAlbum is an album seedLibrary puts in the library: tracks tracks,
// titled "<name> Track <n>", each with a file in the source.
type seededAlbum struct {
	artist string
	name   string
	tracks int
}

// seedLibrary seeds a library source at source holding albums, with a file
// for each of their tracks.
func seedLibrary(t *testing.T, db *sql.DB, source string, albums ...seededAlbum) {
	t.Helper()
	for _, a := range albums {
		dir := filepath.Join(source, a.artist, a.name)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		for i := 1; i <= a.tracks; i++ {
			path := filepath.Join(dir, strconv.Itoa(i)+".mp3")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}
			if _, err := db.Exec(`
				INSERT INTO library_tracks
					(path, mtime, artist, album_artist, album, title,
					 disc_number, track_number, year, added_at, updated_at)
				VALUES (?, 1000, ?, ?, ?, ?, 1, ?, 2024, 1000, 1000)
			`, path, a.artist, a.artist, a.name, a.name+" Track "+strconv.Itoa(i), i); err != nil {
				t.Fatalf("insert track: %v", err)
			}
		}
	}
	if _, err := db.Exec(`INSERT INTO library_sources (path, added_at) VALUES (?, 1)`, source); err != nil {
		t.Fatalf("insert source: %v", err)
	}
}

// newBrowserDeleteTestModel is a model showing the library browser over
// albums, seeded in a source it returns.
func newBrowserDeleteTestModel(t *testing.T, albums ...seededAlbum) (m Model, source string) {
	t.Helper()
	source = t.TempDir()
	m = newLibraryTestModel(t, func(db *sql.DB) { seedLibrary(t, db, source, albums...) })
	if !m.Navigation.IsBrowserViewActive() {
		t.Fatal("the library browser is not on screen")
	}
	return m, source
}

// deleteSelectedTrack presses d, moves to option in the confirmation (0:
// remove from library, 1: delete from disk, 2: cancel) and confirms it.
func deleteSelectedTrack(t *testing.T, m Model, option int) Model {
	t.Helper()
	m = press(t, m, "d")
	if !m.Popups.IsVisible(popupctl.Confirm) {
		t.Fatal("d opens no confirmation")
	}
	for range option {
		m = press(t, m, "j")
	}
	return press(t, m, keyEnter)
}

// libraryTitles returns the titles of the library's tracks, sorted.
func libraryTitles(t *testing.T, m Model) []string {
	t.Helper()
	lib := m.Core.Library()
	artists, err := lib.Artists()
	if err != nil {
		t.Fatalf("library artists: %v", err)
	}
	var titles []string
	for _, artist := range artists {
		tracks, err := lib.ArtistTracks(artist)
		if err != nil {
			t.Fatalf("tracks of %s: %v", artist, err)
		}
		for i := range tracks {
			titles = append(titles, tracks[i].Title)
		}
	}
	slices.Sort(titles)
	return titles
}

// shownNames returns the names the rendered view shows, once for each line
// that shows it, in the order of the lines.
func shownNames(m Model, names ...string) []string {
	var shown []string
	for line := range strings.SplitSeq(testutil.StripANSI(m.View()), "\n") {
		for _, name := range names {
			if strings.Contains(line, name) {
				shown = append(shown, name)
			}
		}
	}
	return shown
}

// assertBrowserAfterDelete checks that the browser is still on its tracks
// column, that no error is shown, and that what it selects is in the library.
func assertBrowserAfterDelete(t *testing.T, m Model) {
	t.Helper()
	browser := m.Navigation.LibraryBrowser()
	if got := browser.ActiveColumn(); got != librarybrowser.ColumnTracks {
		t.Errorf("active column = %v after the removal, want the tracks column", got)
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error shown: %s", msg)
	}
	if track := browser.SelectedTrack(); track != nil && !slices.Contains(libraryTitles(t, m), track.Title) {
		t.Errorf("selected track %q is not in the library", track.Title)
	}
}

func TestLibraryBrowserDelete_ShowsTheLibraryWithoutTheTrackAndKeepsTheRow(t *testing.T) {
	all := []string{"Album Track 1", "Album Track 2", "Album Track 3", "Album Track 4"}
	for _, tc := range []struct {
		name     string
		down     int    // j presses in the tracks column
		option   int    // in the confirmation
		removed  string // the track d is pressed on
		selected string // the track selected after
	}{
		{name: "remove from library", down: 2, removed: "Album Track 3", selected: "Album Track 4"},
		{name: "delete from disk", down: 2, option: 1, removed: "Album Track 3", selected: "Album Track 4"},
		{name: "last track", down: 3, removed: "Album Track 4", selected: "Album Track 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, source := newBrowserDeleteTestModel(t, seededAlbum{"Artist", "Album", 4})
			m = press(t, m, "l", "l")
			for range tc.down {
				m = press(t, m, "j")
			}
			if got := m.Navigation.LibraryBrowser().SelectedTrack().Title; got != tc.removed {
				t.Fatalf("the cursor is on %q, want on %s", got, tc.removed)
			}

			m = deleteSelectedTrack(t, m, tc.option)

			left := slices.DeleteFunc(slices.Clone(all), func(s string) bool { return s == tc.removed })
			if got := libraryTitles(t, m); !slices.Equal(got, left) {
				t.Fatalf("library holds %v after the removal, want %v", got, left)
			}
			if got := shownNames(m, all...); !slices.Equal(got, left) {
				t.Errorf("the browser shows %v after the removal, want %v", got, left)
			}
			if got := m.Navigation.LibraryBrowser().SelectedTrack(); got == nil || got.Title != tc.selected {
				t.Errorf("selected track = %+v after the removal, want %s", got, tc.selected)
			}
			_, err := os.Stat(filepath.Join(source, "Artist", "Album", strconv.Itoa(tc.down+1)+".mp3"))
			if os.IsNotExist(err) != (tc.option == 1) {
				t.Errorf("file of %s: stat error %v, want it deleted: %v", tc.removed, err, tc.option == 1)
			}
			assertBrowserAfterDelete(t, m)
		})
	}
}

func TestLibraryBrowserDelete_TwiceInARowRemovesTwoTracks(t *testing.T) {
	m, _ := newBrowserDeleteTestModel(t, seededAlbum{"Artist", "Album", 4})
	m = press(t, m, "l", "l", "j", "j")

	m = deleteSelectedTrack(t, m, 0)
	assertBrowserAfterDelete(t, m)
	m = deleteSelectedTrack(t, m, 0)
	assertBrowserAfterDelete(t, m)

	if got, want := libraryTitles(t, m), []string{"Album Track 1", "Album Track 2"}; !slices.Equal(got, want) {
		t.Errorf("library holds %v after two removals, want %v", got, want)
	}
	if got := m.Navigation.LibraryBrowser().SelectedTrack(); got == nil || got.Title != "Album Track 2" {
		t.Errorf("selected track = %+v, want Album Track 2", got)
	}
}

func TestLibraryBrowserDelete_TheLastTrackOfAnAlbumTakesTheAlbumAway(t *testing.T) {
	m, _ := newBrowserDeleteTestModel(t,
		seededAlbum{"Artist", "Gone", 1},
		seededAlbum{"Artist", "Kept", 2},
	)
	m = press(t, m, "l")
	if got := m.Navigation.LibraryBrowser().SelectedAlbum().Name; got != "Gone" {
		t.Fatalf("the cursor is on album %q, want on Gone", got)
	}
	m = press(t, m, "l")

	m = deleteSelectedTrack(t, m, 0)

	browser := m.Navigation.LibraryBrowser()
	if got := browser.SelectedAlbum(); got == nil || got.Name != "Kept" {
		t.Errorf("selected album = %+v, want Kept", got)
	}
	if got := browser.SelectedTrack(); got == nil || got.Title != "Kept Track 1" {
		t.Errorf("selected track = %+v, want Kept Track 1", got)
	}
	if got := shownNames(m, "Gone"); len(got) != 0 {
		t.Errorf("the browser still shows the album Gone")
	}
	assertBrowserAfterDelete(t, m)

	// The album view, read from the library when V opens it, lists it no more.
	m = press(t, m, "V")
	if !m.Navigation.IsAlbumViewActive() {
		t.Fatal("V does not open the album view")
	}
	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "Gone") || !strings.Contains(view, "Kept") {
		t.Errorf("the album view lists Gone or not Kept after the removal:\n%s", view)
	}
}

func TestLibraryBrowserDelete_TheLastTrackOfAnArtistTakesTheArtistAway(t *testing.T) {
	m, _ := newBrowserDeleteTestModel(t,
		seededAlbum{"Artist A", "Gone", 1},
		seededAlbum{"Artist B", "Kept", 2},
	)
	m = press(t, m, "l", "l")
	if got := m.Navigation.LibraryBrowser().SelectedArtist(); got != "Artist A" {
		t.Fatalf("the cursor is on artist %q, want on Artist A", got)
	}

	m = deleteSelectedTrack(t, m, 0)

	browser := m.Navigation.LibraryBrowser()
	if got := browser.SelectedArtist(); got != "Artist B" {
		t.Errorf("selected artist = %q, want Artist B", got)
	}
	if got := browser.SelectedAlbum(); got == nil || got.Name != "Kept" {
		t.Errorf("selected album = %+v, want Kept", got)
	}
	if got := browser.SelectedTrack(); got == nil || got.Title != "Kept Track 1" {
		t.Errorf("selected track = %+v, want Kept Track 1", got)
	}
	if got := shownNames(m, "Artist A", "Gone"); len(got) != 0 {
		t.Errorf("the browser still shows %v", got)
	}
	assertBrowserAfterDelete(t, m)
}

func TestLibraryBrowserDelete_TheLastTrackOfTheLibraryEmptiesTheBrowser(t *testing.T) {
	m, _ := newBrowserDeleteTestModel(t, seededAlbum{"Solo Singer", "Single", 1})
	m = press(t, m, "l", "l")

	m = deleteSelectedTrack(t, m, 0)

	browser := m.Navigation.LibraryBrowser()
	if got := browser.SelectedTrack(); got != nil {
		t.Errorf("selected track = %+v in an empty library, want none", got)
	}
	if got := browser.SelectedAlbum(); got != nil {
		t.Errorf("selected album = %+v in an empty library, want none", got)
	}
	if got := browser.SelectedArtist(); got != "" {
		t.Errorf("selected artist = %q in an empty library, want none", got)
	}
	if got := shownNames(m, "Solo Singer", "Single"); len(got) != 0 {
		t.Errorf("the browser still shows %v", got)
	}
	assertBrowserAfterDelete(t, m)

	m = press(t, m, "d")
	if m.Popups.IsVisible(popupctl.Confirm) {
		t.Error("d opens a confirmation in an empty library")
	}
}

func TestLibraryBrowserDelete_CancelOrAFailureLeavesTheBrowser(t *testing.T) {
	all := []string{"Album Track 1", "Album Track 2", "Album Track 3"}
	for _, tc := range []struct {
		name   string
		option int
		fails  bool
	}{
		{name: "cancel", option: 2},
		{name: "failed deletion from disk", option: 1, fails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, source := newBrowserDeleteTestModel(t, seededAlbum{"Artist", "Album", 3})
			m = press(t, m, "l", "l", "j")
			if tc.fails {
				if err := os.Remove(filepath.Join(source, "Artist", "Album", "2.mp3")); err != nil {
					t.Fatalf("remove file: %v", err)
				}
			}

			m = deleteSelectedTrack(t, m, tc.option)

			if got := libraryTitles(t, m); !slices.Equal(got, all) {
				t.Errorf("library holds %v, want %v", got, all)
			}
			if got := shownNames(m, all...); !slices.Equal(got, all) {
				t.Errorf("the browser shows %v, want %v", got, all)
			}
			browser := m.Navigation.LibraryBrowser()
			if got := browser.SelectedTrack(); got == nil || got.Title != "Album Track 2" {
				t.Errorf("selected track = %+v, want Album Track 2", got)
			}
			if got := browser.ActiveColumn(); got != librarybrowser.ColumnTracks {
				t.Errorf("active column = %v, want the tracks column", got)
			}
			if failed := m.Popups.ErrorMsg() != ""; failed != tc.fails {
				t.Errorf("error shown: %v, want %v", failed, tc.fails)
			}
		})
	}
}

func TestLibraryMillerDelete_KeepsTheCursorOnTheRow(t *testing.T) {
	for _, tc := range []struct {
		name     string
		down     int
		selected string
	}{
		{name: "middle track", down: 1, selected: "Album Track 3"},
		{name: "last track", down: 2, selected: "Album Track 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newBrowserDeleteTestModel(t, seededAlbum{"Artist", "Album", 3})
			m.Navigation.SetLibrarySubMode(navctl.LibraryModeMiller)
			m.Navigation.SetFocus(m.Navigation.Focus())
			m = press(t, m, "l", "l")
			for range tc.down {
				m = press(t, m, "j")
			}

			m = deleteSelectedTrack(t, m, 0)

			selected := m.Navigation.LibraryNav().Selected()
			if selected == nil || selected.Track() == nil || selected.Track().Title != tc.selected {
				t.Errorf("selected %+v after the removal, want %s", selected, tc.selected)
			}
			if msg := m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error shown: %s", msg)
			}
		})
	}
}

func TestLibraryMillerDelete_TheLastTrackOfAContainerLeavesForOneThatExists(t *testing.T) {
	for _, tc := range []struct {
		name     string
		albums   []seededAlbum
		current  string // the container shown after the removal
		selected string // the item selected in it
	}{
		{
			name: "album of an artist with other albums",
			albums: []seededAlbum{
				{"Artist B", "Album B", 1},
				{"Artist B", "Album B2", 2},
				{"Artist C", "Album C", 1},
			},
			current:  "library:artist:Artist B",
			selected: "library:album:Artist B:Album B2",
		},
		{
			name: "only album of an artist",
			albums: []seededAlbum{
				{"Artist A", "Album A", 1},
				{"Artist C", "Album C", 1},
			},
			current:  "library:root",
			selected: "library:artist:Artist C",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newBrowserDeleteTestModel(t, tc.albums...)
			m.Navigation.SetLibrarySubMode(navctl.LibraryModeMiller)
			m.Navigation.SetFocus(m.Navigation.Focus())
			m = press(t, m, "l", "l")
			first := tc.albums[0]
			if got := m.Navigation.LibraryNav().Current().ID(); got != "library:album:"+first.artist+":"+first.name {
				t.Fatalf("the navigator is in %s, want in %s", got, first.name)
			}

			m = deleteSelectedTrack(t, m, 0)

			nav := m.Navigation.LibraryNav()
			if got := nav.Current().ID(); got != tc.current {
				t.Errorf("the navigator is in %s after the removal, want in %s", got, tc.current)
			}
			if got := nav.SelectedID(); got != tc.selected {
				t.Errorf("selected %q after the removal, want %s", got, tc.selected)
			}
			if msg := m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error shown: %s", msg)
			}
		})
	}
}

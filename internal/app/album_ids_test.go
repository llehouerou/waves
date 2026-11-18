package app

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// colonTwins is a library of two albums whose album artist and name, joined
// with a colon, read the same, next to a third album.
var colonTwins = []seededAlbum{
	{"AC", "DC:Back", 1},
	{"AC:DC", "Back", 1},
	{"Other", "Other Album", 1},
}

// newAlbumViewTwinsModel is a model showing the library's album view over
// colonTwins, on a database at database.
func newAlbumViewTwinsModel(t *testing.T, database string) Model {
	t.Helper()
	seedDatabase(t, database, func(db *sql.DB) { seedLibrary(t, db, t.TempDir(), colonTwins...) })
	m := startTestModel(t, core.Options{Database: database})
	m.openAlbumView("", "")
	if !m.Navigation.IsAlbumViewActive() {
		t.Fatal("the album view is not on screen")
	}
	return m
}

// assertAlbumSelected checks that the album view's cursor is on the album
// named album, by the album artist given.
func assertAlbumSelected(t *testing.T, m Model, artist, album string) {
	t.Helper()
	got := m.Navigation.AlbumView().SelectedAlbum()
	if got == nil {
		t.Fatalf("no album selected, want %q / %q", artist, album)
	}
	if got.AlbumArtist != artist || got.Album != album {
		t.Errorf("selected %q / %q, want %q / %q", got.AlbumArtist, got.Album, artist, album)
	}
}

// moveToAlbum moves the album view's cursor down to the album named album,
// by the album artist given, with the keys.
func moveToAlbum(t *testing.T, m Model, artist, album string) Model {
	t.Helper()
	m = press(t, m, "g", "g")
	for range 10 {
		if got := m.Navigation.AlbumView().SelectedAlbum(); got != nil && got.AlbumArtist == artist && got.Album == album {
			return m
		}
		m = press(t, m, "j")
	}
	t.Fatalf("the album view does not reach %q / %q", artist, album)
	return m
}

// forEachTwin runs test for each of the two albums of colonTwins that read
// the same once joined with a colon.
func forEachTwin(t *testing.T, test func(t *testing.T, artist, album string)) {
	t.Helper()
	for _, a := range colonTwins[:2] {
		t.Run(a.artist+" - "+a.name, func(t *testing.T) { test(t, a.artist, a.name) })
	}
}

func TestAlbumView_LibrarySearchResultSelectsTheAlbumAskedFor(t *testing.T) {
	forEachTwin(t, func(t *testing.T, artist, album string) {
		m := newAlbumViewTwinsModel(t, filepath.Join(t.TempDir(), "waves.db"))
		m = moveToAlbum(t, m, "Other", "Other Album")

		m.HandleLibrarySearchResult(core.SearchResult{Type: core.ResultAlbum, Artist: artist, Album: album})

		assertAlbumSelected(t, m, artist, album)
	})
}

func TestAlbumView_GoToSourceFromTheQueueSelectsTheAlbumOfTheTrack(t *testing.T) {
	forEachTwin(t, func(t *testing.T, artist, album string) {
		m := newAlbumViewTwinsModel(t, filepath.Join(t.TempDir(), "waves.db"))
		m = moveToAlbum(t, m, "Other", "Other Album")
		tracks, err := m.collectAlbumTracks(artist, album)
		if err != nil || len(tracks) == 0 {
			t.Fatalf("tracks of %q / %q = %v (err %v)", artist, album, tracks, err)
		}

		m.handleGoToSource(queuepanel.GoToSource{TrackID: tracks[0].ID, Path: tracks[0].Path})

		assertAlbumSelected(t, m, artist, album)
	})
}

func TestAlbumView_OpeningItOnAnAlbumSelectsThatAlbum(t *testing.T) {
	forEachTwin(t, func(t *testing.T, artist, album string) {
		m := newAlbumViewTwinsModel(t, filepath.Join(t.TempDir(), "waves.db"))
		m = moveToAlbum(t, m, "Other", "Other Album")

		m.openAlbumView(artist, album)

		assertAlbumSelected(t, m, artist, album)
	})
}

func TestAlbumView_TheToggleFromTheBrowserKeepsTheAlbum(t *testing.T) {
	forEachTwin(t, func(t *testing.T, artist, album string) {
		m := newAlbumViewTwinsModel(t, filepath.Join(t.TempDir(), "waves.db"))
		m = moveToAlbum(t, m, "Other", "Other Album")
		m.toggleLibraryViewMode()
		if !m.Navigation.IsBrowserViewActive() {
			t.Fatal("the toggle does not show the library browser")
		}
		m.selectAlbumInCurrentMode(artist, album)

		m.toggleLibraryViewMode()

		if !m.Navigation.IsAlbumViewActive() {
			t.Fatal("the toggle does not show the album view")
		}
		assertAlbumSelected(t, m, artist, album)
	})
}

func TestAlbumView_ARefreshKeepsTheCursorOnItsAlbum(t *testing.T) {
	forEachTwin(t, func(t *testing.T, artist, album string) {
		m := newAlbumViewTwinsModel(t, filepath.Join(t.TempDir(), "waves.db"))
		m = moveToAlbum(t, m, artist, album)

		m, _ = updateModel(t, m, LibraryScanCompleteMsg{})

		assertAlbumSelected(t, m, artist, album)
	})
}

func TestAlbumView_TheSavedSelectionIsRestoredAtTheNextStart(t *testing.T) {
	forEachTwin(t, func(t *testing.T, artist, album string) {
		database := filepath.Join(t.TempDir(), "waves.db")
		m := newAlbumViewTwinsModel(t, database)
		m = moveToAlbum(t, m, artist, album)
		m.SaveNavigationState()
		if err := m.Core.Close(); err != nil {
			t.Fatalf("close core: %v", err)
		}

		next := startTestModel(t, core.Options{Database: database})

		if !next.Navigation.IsAlbumViewActive() {
			t.Fatal("the next start does not show the album view")
		}
		assertAlbumSelected(t, next, artist, album)
	})
}

// An earlier version saved the album selection as its album artist and name
// joined with a colon: waves starts on an album of the list, with no error.
func TestAlbumView_ASelectionSavedByAnEarlierVersionStartsOnAnAlbum(t *testing.T) {
	database := filepath.Join(t.TempDir(), "waves.db")
	seedDatabase(t, database, func(db *sql.DB) { seedLibrary(t, db, t.TempDir(), colonTwins...) })
	seedNavigation(t, database, state.NavigationState{
		ViewMode:        string(navctl.ViewLibrary),
		LibrarySubMode:  "album",
		AlbumSelectedID: "AC:DC:Back",
	})

	m := startTestModel(t, core.Options{Database: database})

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error shown: %s", msg)
	}
	if !m.Navigation.IsAlbumViewActive() {
		t.Fatal("waves does not start on the album view")
	}
	if m.Navigation.AlbumView().SelectedAlbum() == nil {
		t.Error("the album view's cursor is on no album")
	}
}

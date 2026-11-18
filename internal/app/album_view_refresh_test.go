package app

import (
	"database/sql"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/librarysources"
)

// The album view groups by month added, the latest first: "Newer" is in the
// first group, "Older" alone in the last one.
var (
	newerAlbum = seededAlbum{"Artist", "Newer", 2}
	olderAlbum = seededAlbum{"Artist", "Older", 2}
)

// setAddedAt dates the tracks of a in db.
func setAddedAt(t *testing.T, db *sql.DB, a seededAlbum, at time.Time) {
	t.Helper()
	if _, err := db.Exec(`UPDATE library_tracks SET added_at = ? WHERE album_artist = ? AND album = ?`,
		at.Unix(), a.artist, a.name); err != nil {
		t.Fatalf("date %s: %v", a.name, err)
	}
}

// newTwoMonthsAlbumViewModel is a model showing the album view on olderAlbum,
// the last album of its list, each album seeded in a source of its own; it
// returns the source of olderAlbum.
func newTwoMonthsAlbumViewModel(t *testing.T) (m Model, olderSource string) {
	t.Helper()
	newerSource, olderSource := t.TempDir(), t.TempDir()
	m = newLibraryTestModel(t, func(db *sql.DB) {
		seedLibrary(t, db, newerSource, newerAlbum)
		seedLibrary(t, db, olderSource, olderAlbum)
		setAddedAt(t, db, newerAlbum, time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC))
		setAddedAt(t, db, olderAlbum, time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC))
	})
	m.openAlbumView("", "")
	if !m.Navigation.IsAlbumViewActive() {
		t.Fatal("the album view is not on screen")
	}
	m = moveToAlbum(t, m, olderAlbum.artist, olderAlbum.name)
	return m, olderSource
}

// deleteAlbum removes the tracks of a from the library.
func deleteAlbum(t *testing.T, m Model, a seededAlbum) {
	t.Helper()
	ids, err := m.Core.Library().AlbumTrackIDs(a.artist, a.name)
	if err != nil || len(ids) == 0 {
		t.Fatalf("tracks of %s: %v, %v", a.name, ids, err)
	}
	for _, id := range ids {
		if err := m.Core.Library().DeleteTrack(id); err != nil {
			t.Fatalf("delete track %d: %v", id, err)
		}
	}
}

func TestAlbumView_ARefreshThatRemovesTheLastAlbumSelectsTheRemainingOne(t *testing.T) {
	m, _ := newTwoMonthsAlbumViewModel(t)
	deleteAlbum(t, m, olderAlbum)

	m, _ = updateModel(t, m, LibraryScanCompleteMsg{})
	_ = m.View()

	assertAlbumSelected(t, m, newerAlbum.artist, newerAlbum.name)
}

func TestAlbumView_ARefreshThatRemovesItsLastAlbumWithTheBrowserOnScreenSelectsTheRemainingOne(t *testing.T) {
	m, _ := newTwoMonthsAlbumViewModel(t)
	m.toggleLibraryViewMode()
	if !m.Navigation.IsBrowserViewActive() {
		t.Fatal("the toggle does not show the library browser")
	}
	deleteAlbum(t, m, olderAlbum)

	m, _ = updateModel(t, m, LibraryScanCompleteMsg{})
	_ = m.View()

	assertAlbumSelected(t, m, newerAlbum.artist, newerAlbum.name)
}

func TestAlbumView_RemovingTheSourceOfTheLastAlbumSelectsAnAlbum(t *testing.T) {
	m, olderSource := newTwoMonthsAlbumViewModel(t)
	sources, err := m.Core.Library().Sources()
	if err != nil {
		t.Fatalf("sources: %v", err)
	}
	m.Popups.ShowLibrarySources(sources)

	m, _ = updateModel(t, m, librarysources.ActionMsg(librarysources.SourceRemoved{Path: olderSource}))
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	_ = m.View()

	assertAlbumSelected(t, m, newerAlbum.artist, newerAlbum.name)
}

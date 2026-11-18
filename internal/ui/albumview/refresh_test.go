package albumview

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/core"
)

// album is an album added to the library on the given day of 2024.
func album(name string, month time.Month, day int) core.AlbumEntry {
	return core.AlbumEntry{
		AlbumArtist: "Artist",
		Album:       name,
		AddedAt:     time.Date(2024, month, day, 0, 0, 0, 0, time.UTC),
		TrackCount:  1,
	}
}

// newRefreshModel is an album view over lib, with the default settings
// (grouped by month added, the latest first), sized and refreshed.
func newRefreshModel(t *testing.T, lib *fakeLibrary) Model {
	t.Helper()
	m := New(lib)
	m.SetSize(80, 40)
	if err := m.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	return m
}

// remove takes the album named name out of lib.
func remove(lib *fakeLibrary, name string) {
	*lib = slices.DeleteFunc(*lib, func(a core.AlbumEntry) bool { return a.Album == name })
}

// refresh refreshes m, then resizes and renders it as the app does after a
// library scan.
func refresh(t *testing.T, m *Model) {
	t.Helper()
	if err := m.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	m.SetSize(80, 40)
	m.SetSize(80, 5)
	_ = m.View()
}

func assertSelected(t *testing.T, m Model, want string) {
	t.Helper()
	got := m.SelectedAlbum()
	if got == nil {
		t.Fatalf("SelectedAlbum() = nil, want %q", want)
	}
	if got.Album != want {
		t.Errorf("SelectedAlbum() = %q, want %q", got.Album, want)
	}
}

func TestRefresh_LastAlbumAloneInItsGroupGone_SelectsTheNewLastAlbum(t *testing.T) {
	// March: March 20, March 10 / January: January 5
	lib := fakeLibrary{album("March 20", time.March, 20), album("March 10", time.March, 10), album("January 5", time.January, 5)}
	m := newRefreshModel(t, &lib)
	selectAlbum(t, &m, lib[2])

	remove(&lib, "January 5")
	refresh(t, &m)

	assertSelected(t, m, "March 10")
}

func TestRefresh_LastAlbumOfItsGroupGone_SelectsTheAlbumNowLast(t *testing.T) {
	// March: March 20 / January: January 9, January 5
	lib := fakeLibrary{album("March 20", time.March, 20), album("January 9", time.January, 9), album("January 5", time.January, 5)}
	m := newRefreshModel(t, &lib)
	selectAlbum(t, &m, lib[2])

	remove(&lib, "January 5")
	refresh(t, &m)

	assertSelected(t, m, "January 9")
}

func TestRefresh_LastAlbumOfAGroupBeforeAnotherGone_SelectsTheFirstAlbumOfTheNextGroup(t *testing.T) {
	// March: March 20, March 10 / January: January 9, January 5
	lib := fakeLibrary{
		album("March 20", time.March, 20), album("March 10", time.March, 10),
		album("January 9", time.January, 9), album("January 5", time.January, 5),
	}
	m := newRefreshModel(t, &lib)
	selectAlbum(t, &m, lib[1])

	remove(&lib, "March 10")
	refresh(t, &m)

	assertSelected(t, m, "January 9")
}

func TestRefresh_EveryAlbumGone_SelectsNothingThenTheAlbumsBack(t *testing.T) {
	albums := fakeLibrary{album("March 20", time.March, 20), album("January 5", time.January, 5)}
	lib := slices.Clone(albums)
	m := newRefreshModel(t, &lib)
	selectAlbum(t, &m, lib[1])

	lib = nil
	refresh(t, &m)

	if got := m.SelectedAlbum(); got != nil {
		t.Errorf("SelectedAlbum() = %+v on an empty library, want nil", got)
	}
	if got := m.SavedSelection(); got != "" {
		t.Errorf("SavedSelection() = %q on an empty library, want empty", got)
	}

	lib = albums
	refresh(t, &m)

	if got := m.SelectedAlbum(); got == nil {
		t.Error("SelectedAlbum() = nil once the albums are back, want an album")
	}
}

func TestRefresh_AnotherAlbumGone_KeepsTheSelectedAlbum(t *testing.T) {
	lib := fakeLibrary{album("March 20", time.March, 20), album("March 10", time.March, 10), album("January 5", time.January, 5)}
	m := newRefreshModel(t, &lib)
	selectAlbum(t, &m, lib[2])

	remove(&lib, "March 20")
	refresh(t, &m)

	assertSelected(t, m, "January 5")
}

func TestRefresh_ListShrunkBelowTheViewport_ShowsItFromItsFirstRow(t *testing.T) {
	// March: "Album 01" (the latest) to "Album 60" (the oldest)
	var lib fakeLibrary
	for day := 60; day >= 1; day-- {
		lib = append(lib, core.AlbumEntry{
			AlbumArtist: "Artist",
			Album:       fmt.Sprintf("Album %02d", 61-day),
			AddedAt:     time.Date(2024, time.March, 1, day, 0, 0, 0, time.UTC),
		})
	}
	m := New(&lib)
	m.SetSize(80, 30)
	if err := m.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	selectAlbum(t, &m, lib[59])

	lib = lib[:10]
	if err := m.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	assertSelected(t, m, "Album 10")
	if view := m.View(); !strings.Contains(view, "Album 01") {
		t.Errorf("the first album is scrolled out of a view the whole list fits in:\n%s", view)
	}
}

package app

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/tags/tagstest"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/librarynav"
	"github.com/llehouerou/waves/internal/ui/retag"
)

// newRetagLibraryTestModel builds a model whose library holds, two files
// each, "first" by "abc", "attic", "trust" and "umbrella" by "low", "middle"
// by "mid", "last" by "zed", and the albums others (album artist, album).
// With trustAlone, "trust" is the only album of "low". The Library view is
// on screen, the scan report closed.
func newRetagLibraryTestModel(t *testing.T, trustAlone bool, others ...[2]string) Model {
	t.Helper()
	albums := [][2]string{{"abc", "first"}, {"low", "trust"}, {"mid", "middle"}, {"zed", "last"}}
	if !trustAlone {
		albums = append(albums, [2]string{"low", "attic"}, [2]string{"low", "umbrella"})
	}
	albums = append(albums, others...)
	source := t.TempDir()
	for _, album := range albums {
		for number := 1; number <= 2; number++ {
			title := fmt.Sprintf("track %d", number)
			path := filepath.Join(source, album[0], album[1], title+".mp3")
			tagstest.WriteMP3(t, path, core.FileTags{
				Artist: album[0], AlbumArtist: album[0], Album: album[1], Title: title, TrackNumber: number,
			})
		}
	}
	m := newLibraryTestModel(t, func(*sql.DB) {})
	if err := m.Core.Library().AddSource(source); err != nil {
		t.Fatalf("add source: %v", err)
	}
	m = pumpUntil[LibraryScanCompleteMsg](t, m)
	m, _ = updateModel(t, m, keyMsg(keyEscape)) // the scan report
	m.Navigation.SetViewMode(navctl.ViewLibrary)
	return m
}

// retagLowTrustAs is the retag of "low" - "trust" as the release albumArtist
// - album, whose tracks are given to as many files of trust.
func retagLowTrustAs(albumArtist, album string, tracks int) core.RetagRequest {
	release := &core.MBReleaseDetails{Release: core.MBRelease{ID: "release", Title: album, Artist: albumArtist}}
	for i := 1; i <= tracks; i++ {
		release.Tracks = append(release.Tracks, core.MBTrack{Position: i, Title: fmt.Sprintf("Track %d", i)})
	}
	return core.RetagRequest{
		AlbumArtist: "low", Album: "trust",
		Release:      release,
		ReleaseGroup: &core.MBReleaseGroup{ID: "rg", PrimaryType: "Album"},
	}
}

// retagUntilFinished starts req and feeds the core's events to the model
// until it handled the end of the retag, which it returns.
func retagUntilFinished(t *testing.T, m Model, req core.RetagRequest) (Model, core.RetagFinished) {
	t.Helper()
	if err := m.Core.Library().Retag(req); err != nil {
		t.Fatalf("retag: %v", err)
	}
	m, msg := nextMatch(t, m, isRetagFinished)
	finished, _ := msg.(ServiceRetagMsg).Event.(core.RetagFinished)
	m, _ = updateModel(t, m, msg)
	return m, finished
}

// browseLowTrust shows the library browser on "low" - "trust", with the
// column active.
func browseLowTrust(t *testing.T, m Model, column librarybrowser.Column) Model {
	t.Helper()
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeBrowser)
	browser := m.Navigation.LibraryBrowser()
	browser.SelectArtist("low")
	browser.SelectAlbum("trust")
	browser.SetActiveColumn(column)
	assertBrowserOn(t, m, "low", "trust", column)
	return m
}

// assertBrowserOn fails unless the library browser selects the album
// albumArtist - album with the column active.
func assertBrowserOn(t *testing.T, m Model, albumArtist, album string, column librarybrowser.Column) {
	t.Helper()
	browser := m.Navigation.LibraryBrowser()
	got := browser.SelectedAlbum()
	if browser.SelectedArtist() != albumArtist || got == nil || got.Name != album {
		t.Errorf("the browser selects %q - %+v, want %s - %s", browser.SelectedArtist(), got, albumArtist, album)
	}
	if browser.ActiveColumn() != column {
		t.Errorf("the browser's active column is %v, want %v", browser.ActiveColumn(), column)
	}
}

// The retag renames the album the browser is on: the browser follows it.
func TestRetag_TheBrowserFollowsTheAlbumRenamed(t *testing.T) {
	m := browseLowTrust(t, newRetagLibraryTestModel(t, false), librarybrowser.ColumnAlbums)

	m, _ = retagUntilFinished(t, m, retagLowTrustAs("Now", "Trust", 2))

	assertBrowserOn(t, m, "Now", "Trust", librarybrowser.ColumnAlbums)
}

// The browser is on a track of the album renamed: it stays on that track, in
// the tracks column.
func TestRetag_TheBrowserFollowsTheTrackOfTheAlbumRenamed(t *testing.T) {
	m := browseLowTrust(t, newRetagLibraryTestModel(t, false), librarybrowser.ColumnTracks)
	browser := m.Navigation.LibraryBrowser()
	browser.JumpToIndex(librarybrowser.ColumnTracks, 1)
	second := browser.SelectedTrack()
	if second == nil || second.Title != "track 2" {
		t.Fatalf("the browser selects %+v, want track 2 of trust", second)
	}

	m, _ = retagUntilFinished(t, m, retagLowTrustAs("Now", "Trust", 2))

	assertBrowserOn(t, m, "Now", "Trust", librarybrowser.ColumnTracks)
	if got := m.Navigation.LibraryBrowser().SelectedTrack(); got == nil || got.ID != second.ID {
		t.Errorf("the browser selects the track %+v, want the track of ID %d", got, second.ID)
	}
}

// The user moved the browser off the album while it was retagged: it stays
// where the user put it.
func TestRetag_TheBrowserLeavesTheAlbumTheUserMovedTo(t *testing.T) {
	m := browseLowTrust(t, newRetagLibraryTestModel(t, false), librarybrowser.ColumnAlbums)

	if err := m.Core.Library().Retag(retagLowTrustAs("Now", "Trust", 2)); err != nil {
		t.Fatalf("retag: %v", err)
	}
	browser := m.Navigation.LibraryBrowser()
	browser.SelectArtist("abc")
	browser.SelectAlbum("first")
	m = pumpUntilMatch(t, m, isRetagFinished)

	assertBrowserOn(t, m, "abc", "first", librarybrowser.ColumnAlbums)
}

// The browser follows the album while another view is on screen.
func TestRetag_TheBrowserFollowsTheAlbumRenamedOffScreen(t *testing.T) {
	m := browseLowTrust(t, newRetagLibraryTestModel(t, false), librarybrowser.ColumnAlbums)

	if err := m.Core.Library().Retag(retagLowTrustAs("Now", "Trust", 2)); err != nil {
		t.Fatalf("retag: %v", err)
	}
	m.Navigation.SetViewMode(navctl.ViewFileBrowser)
	m = pumpUntilMatch(t, m, isRetagFinished)
	m.Navigation.SetViewMode(navctl.ViewLibrary)

	assertBrowserOn(t, m, "Now", "Trust", librarybrowser.ColumnAlbums)
}

// The retag gives the album the names of one the library already has: the
// browser is on that album.
func TestRetag_TheBrowserFollowsTheAlbumIntoOneItJoins(t *testing.T) {
	m := browseLowTrust(t, newRetagLibraryTestModel(t, false, [2]string{"Now", "Trust"}), librarybrowser.ColumnAlbums)

	m, _ = retagUntilFinished(t, m, retagLowTrustAs("Now", "Trust", 2))

	assertBrowserOn(t, m, "Now", "Trust", librarybrowser.ColumnAlbums)
}

// A file the release has no track for keeps its tags: the album is still in
// the library under its old names, and the browser stays on it.
func TestRetag_ThatFailsForAFileLeavesTheBrowserOnTheOldAlbum(t *testing.T) {
	m := browseLowTrust(t, newRetagLibraryTestModel(t, false), librarybrowser.ColumnAlbums)

	m, finished := retagUntilFinished(t, m, retagLowTrustAs("Now", "Trust", 1))

	if got := workFailure("Retag", "low - trust", finished.Files, finished.Err); got != "Retag of low - trust: 1 of 2 files failed" {
		t.Errorf("the retag ended as %q, want one file of two failed", got)
	}
	assertBrowserOn(t, m, "low", "trust", librarybrowser.ColumnAlbums)
}

// completeOf is what the retag popup emits once it handled finished with no
// failure (internal/ui/retag).
func completeOf(finished core.RetagFinished) tea.Msg {
	return retag.ActionMsg(retag.Complete{AlbumArtist: finished.LibraryAlbumArtist, Album: finished.LibraryAlbum})
}

// The popup was left open to the end of a retag that renamed the album: once
// it is done, the browser shows the album under its new names.
func TestRetagComplete_TheBrowserShowsTheAlbumRenamed(t *testing.T) {
	for _, renamed := range [][2]string{{"Now", "Trust"}, {"low", "Zen"}} {
		t.Run(renamed[0]+" - "+renamed[1], func(t *testing.T) {
			m := browseLowTrust(t, newRetagLibraryTestModel(t, false), librarybrowser.ColumnAlbums)
			m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())

			m, finished := retagUntilFinished(t, m, retagLowTrustAs(renamed[0], renamed[1], 2))
			assertBrowserOn(t, m, renamed[0], renamed[1], librarybrowser.ColumnAlbums)

			m, _ = updateModel(t, m, completeOf(finished))
			assertBrowserOn(t, m, renamed[0], renamed[1], librarybrowser.ColumnAlbums)
			if m.Popups.IsVisible(popupctl.Retag) {
				t.Error("the retag popup is still open")
			}
		})
	}
}

// millerIn shows the Miller columns inside the container of ID container,
// with the cursor on the node of ID selected.
func millerIn(t *testing.T, m Model, container, selected string) Model {
	t.Helper()
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeMiller)
	nav := m.Navigation.LibraryNav()
	if !nav.NavigateTo(container) || !nav.SelectByID(selected) {
		t.Fatalf("the Miller columns cannot be put in %q on %q", container, selected)
	}
	assertMillerIn(t, m, container, selected)
	return m
}

// assertMillerIn fails unless the Miller columns are inside the container of
// ID container with the cursor on the node of ID selected.
func assertMillerIn(t *testing.T, m Model, container, selected string) {
	t.Helper()
	nav := m.Navigation.LibraryNav()
	if got := nav.Current().ID(); got != container {
		t.Errorf("the Miller columns are inside %q, want %q", got, container)
	}
	if got := nav.SelectedID(); got != selected {
		t.Errorf("the Miller columns' cursor is on %q, want %q", got, selected)
	}
}

// trackIDs are the IDs of the tracks of the album albumArtist - album, in
// track order.
func trackIDs(t *testing.T, m Model, albumArtist, album string) []int64 {
	t.Helper()
	ids, err := m.Core.Library().AlbumTrackIDs(albumArtist, album)
	if err != nil || len(ids) != 2 {
		t.Fatalf("tracks of %s - %s = %v (err %v), want two", albumArtist, album, ids, err)
	}
	return ids
}

// The Miller columns' cursor is on the row of the album renamed: it is on
// that row under the album's new artist, and the end of the popup enters the
// album, on its first track. Whether the artist keeps other albums or not.
func TestRetag_TheMillerColumnsFollowTheAlbumRenamed(t *testing.T) {
	for _, trustAlone := range []bool{false, true} {
		t.Run(fmt.Sprintf("trust alone %v", trustAlone), func(t *testing.T) {
			m := newRetagLibraryTestModel(t, trustAlone)
			m = millerIn(t, m, librarynav.ArtistNodeID("low"), librarynav.AlbumNodeID("low", "trust"))
			m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())

			m, finished := retagUntilFinished(t, m, retagLowTrustAs("Now", "Trust", 2))
			assertMillerIn(t, m, librarynav.ArtistNodeID("Now"), librarynav.AlbumNodeID("Now", "Trust"))

			m, _ = updateModel(t, m, completeOf(finished))
			ids := trackIDs(t, m, "Now", "Trust")
			assertMillerIn(t, m, librarynav.AlbumNodeID("Now", "Trust"), librarynav.TrackNodeID(ids[0]))
			if items := m.Navigation.LibraryNav().CurrentItems(); len(items) != 2 {
				t.Errorf("the album lists %d tracks, want 2", len(items))
			}
		})
	}
}

// The Miller columns are inside the album renamed: they are inside it under
// its new names, on the same track.
func TestRetag_TheMillerColumnsFollowTheTrackOfTheAlbumRenamed(t *testing.T) {
	for _, trustAlone := range []bool{false, true} {
		t.Run(fmt.Sprintf("trust alone %v", trustAlone), func(t *testing.T) {
			m := newRetagLibraryTestModel(t, trustAlone)
			second := trackIDs(t, m, "low", "trust")[1]
			m = millerIn(t, m, librarynav.AlbumNodeID("low", "trust"), librarynav.TrackNodeID(second))

			m, _ = retagUntilFinished(t, m, retagLowTrustAs("Now", "Trust", 2))

			assertMillerIn(t, m, librarynav.AlbumNodeID("Now", "Trust"), librarynav.TrackNodeID(second))
		})
	}
}

// The Miller columns are on another album: they stay there.
func TestRetag_TheMillerColumnsOnAnotherAlbumStay(t *testing.T) {
	m := newRetagLibraryTestModel(t, false)
	m = millerIn(t, m, librarynav.ArtistNodeID("abc"), librarynav.AlbumNodeID("abc", "first"))

	m, _ = retagUntilFinished(t, m, retagLowTrustAs("Now", "Trust", 2))

	assertMillerIn(t, m, librarynav.ArtistNodeID("abc"), librarynav.AlbumNodeID("abc", "first"))
}

// A retag that keeps the album's names leaves the Miller columns on its row,
// and the end of the popup enters it.
func TestRetag_ThatKeepsTheNamesLeavesTheMillerColumnsOnTheAlbum(t *testing.T) {
	m := newRetagLibraryTestModel(t, false)
	m = millerIn(t, m, librarynav.ArtistNodeID("low"), librarynav.AlbumNodeID("low", "trust"))
	m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())

	m, finished := retagUntilFinished(t, m, retagLowTrustAs("low", "trust", 2))
	assertMillerIn(t, m, librarynav.ArtistNodeID("low"), librarynav.AlbumNodeID("low", "trust"))

	m, _ = updateModel(t, m, completeOf(finished))
	ids := trackIDs(t, m, "low", "trust")
	assertMillerIn(t, m, librarynav.AlbumNodeID("low", "trust"), librarynav.TrackNodeID(ids[0]))
}

// The retag names no album the library has: the end of the popup enters no
// album, least of all the one it was opened on, which the library no longer
// holds.
func TestRetagComplete_WithNoAlbumInTheLibraryEntersNone(t *testing.T) {
	m := newRetagLibraryTestModel(t, false)
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeMiller)
	m.Popups.ShowRetag("gone", "album", m.Core.MusicBrainz(), m.Core.Library())
	finished := core.RetagFinished{
		AlbumArtist: "gone", Album: "album",
		Files: []core.FileProgress{{Path: "/music/gone/album/one.mp3", State: core.FileDone}},
	}

	m, _ = updateModel(t, m, ServiceRetagMsg{Event: finished})
	m, _ = updateModel(t, m, completeOf(finished))

	if got := m.Navigation.LibraryNav().Current().ID(); got == librarynav.AlbumNodeID("gone", "album") {
		t.Errorf("the Miller columns are inside %q, an album the library does not hold", got)
	}
}

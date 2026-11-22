package app

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/albumpreset"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/tags/tagstest"
	"github.com/llehouerou/waves/internal/ui/albumview"
	"github.com/llehouerou/waves/internal/ui/playlistnav"
	"github.com/llehouerou/waves/internal/ui/retag"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// newRetagTestModel builds a model whose library holds one album, "trust" by
// "low", of one file, shown in the library's album view.
func newRetagTestModel(t *testing.T) Model {
	t.Helper()
	source := t.TempDir()
	path := filepath.Join(source, "low", "trust", "canada.mp3")
	tagstest.WriteMP3(t, path, core.FileTags{Artist: "low", AlbumArtist: "low", Album: "trust", Title: "canada", TrackNumber: 1})
	m := newLibraryTestModel(t, func(*sql.DB) {})
	if err := m.Core.Library().AddSource(source); err != nil {
		t.Fatalf("add source: %v", err)
	}
	m = pumpUntil[LibraryScanCompleteMsg](t, m)
	m.Navigation.SetViewMode(navctl.ViewLibrary)
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
	if err := m.Navigation.AlbumView().Refresh(); err != nil {
		t.Fatalf("album view: %v", err)
	}
	return m
}

// asLowTrust is what the retag popup asks the core once the user confirmed.
func asLowTrust() core.RetagRequest {
	return core.RetagRequest{
		AlbumArtist: "low", Album: "trust",
		Release: &core.MBReleaseDetails{
			Release: core.MBRelease{ID: "release", Title: "Trust", Artist: "Low"},
			Tracks:  []core.MBTrack{{Position: 1, Title: "Canada"}},
		},
		ReleaseGroup: &core.MBReleaseGroup{ID: "rg", PrimaryType: "Album"},
	}
}

// The user closed the popup while the core retagged: when it is done the
// library views show the album under the names the retag wrote, with nothing
// asked of the popup.
func TestRetag_TheCoreRunsItAndTheLibraryViewsFollow(t *testing.T) {
	m := newRetagTestModel(t)
	if album := m.Navigation.AlbumView().SelectedAlbum(); album == nil || album.Album != "trust" {
		t.Fatalf("selected album = %+v, want trust before the retag", album)
	}

	if err := m.Core.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	m = pumpUntilMatch(t, m, isRetagFinished)

	if album := m.Navigation.AlbumView().SelectedAlbum(); album == nil || album.AlbumArtist != "Low" || album.Album != "Trust" {
		t.Errorf("album view shows %+v, want Low - Trust", album)
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
}

// With no popup left to show it, a failure is an error popup.
func TestRetagFinished_WithThePopupClosedShowsAFailure(t *testing.T) {
	failed := []core.FileProgress{{Path: "/music/low/trust/canada.mp3", State: core.FileFailed, Err: errors.New("read-only file")}}
	tests := []struct {
		name     string
		finished core.RetagFinished
		want     string
	}{
		{"a file failed", core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: failed},
			"Retag of low - trust: 1 of 1 files failed"},
		{"the library was not updated", core.RetagFinished{AlbumArtist: "low", Album: "trust", Err: errors.New("database is locked")},
			"Retag of low - trust: library not updated: database is locked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newRetagTestModel(t)

			m.handleRetagEvent(tt.finished)

			if got := m.Popups.ErrorMsg(); got != tt.want {
				t.Errorf("error popup = %q, want %q", got, tt.want)
			}
		})
	}
}

// The popup shows its own failures, and closes itself when there is none.
func TestRetagFinished_WithThePopupOpenLeavesItToThePopup(t *testing.T) {
	m := newRetagTestModel(t)
	m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())
	failed := []core.FileProgress{{Path: "/music/low/trust/canada.mp3", State: core.FileFailed, Err: errors.New("read-only file")}}

	m.handleRetagEvent(core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: failed})

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want the failure left to the retag popup", msg)
	}
	if !m.Popups.IsVisible(popupctl.Retag) {
		t.Error("the retag popup was closed on a failure")
	}
}

// The popup is done with its retag: the model closes it. Stopping playback of
// a track being retagged is no longer asked of the model: the core does it.
func TestRetagComplete_ClosesThePopup(t *testing.T) {
	m := newRetagTestModel(t)
	m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())

	m, _ = updateModel(t, m, retag.ActionMsg(retag.Complete{AlbumArtist: "low", Album: "trust"}))

	if m.Popups.IsVisible(popupctl.Retag) {
		t.Error("the retag popup is still open")
	}
}

// playlistOfTrust puts the track of "trust" in a playlist, P or Favorites,
// and opens the Playlists view on it: inside it, or with the cursor on it at
// the root. P holds the track twice, and inside it the cursor is on its
// second row. It goes back to the Library view, where a retag is started.
func playlistOfTrust(t *testing.T, m Model, favorites, enter bool) (opened Model, playlistID int64) {
	t.Helper()
	m, _ = updateModel(t, m, keyMsg(keyEscape)) // the scan report
	ids, err := m.Core.Library().AlbumTrackIDs("low", "trust")
	if err != nil || len(ids) != 1 {
		t.Fatalf("tracks of trust = %v (err %v), want one", ids, err)
	}
	var id int64
	if favorites {
		if _, err := m.Core.Playlists().ToggleFavorites(ids); err != nil {
			t.Fatalf("favorite: %v", err)
		}
		id = favoritesID(t, m)
	} else {
		if id, err = m.Core.Playlists().Create(nil, "P"); err != nil {
			t.Fatalf("create P: %v", err)
		}
		if err := m.Core.Playlists().AddTracks(id, []int64{ids[0], ids[0]}); err != nil {
			t.Fatalf("add to P: %v", err)
		}
	}
	m.refreshPlaylistNavigator()

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF3})
	if !favorites {
		m, _ = updateModel(t, m, keyMsg("j")) // P, after Favorites
	}
	if enter {
		m, _ = updateModel(t, m, keyMsg("l"))
		if !favorites {
			m, _ = updateModel(t, m, keyMsg("j"))
		}
	}
	if got := playlistNavView(m); !strings.Contains(got, "01. low - canada") {
		t.Fatalf("the Playlists view does not show the track before the retag:\n%s", got)
	}
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF1})
	return m, id
}

// favoritesID returns the ID of the Favorites playlist.
func favoritesID(t *testing.T, m Model) int64 {
	t.Helper()
	lists, err := m.Core.Playlists().List(nil)
	if err != nil {
		t.Fatalf("playlists: %v", err)
	}
	for _, pl := range lists {
		if core.IsFavoritesPlaylist(pl.ID) {
			return pl.ID
		}
	}
	t.Fatal("no Favorites playlist")
	return 0
}

// playlistNavView is what the playlist navigator renders, without styles.
func playlistNavView(m Model) string {
	return testutil.StripANSI(m.Navigation.PlaylistNav().View())
}

// playlistNodeNames returns what the playlist navigator shows of nodes.
func playlistNodeNames(nodes []playlistnav.Node) []string {
	shown := make([]string, 0, len(nodes))
	for _, n := range nodes {
		shown = append(shown, n.DisplayName())
	}
	return shown
}

// isRetagFinished accepts the end of a retag.
func isRetagFinished(msg tea.Msg) bool {
	retagMsg, ok := msg.(ServiceRetagMsg)
	if !ok {
		return false
	}
	_, finished := retagMsg.Event.(core.RetagFinished)
	return finished
}

// A playlist holding a track of the retagged album names it as the library
// now does, as soon as the retag is done, inside the playlist or in the
// preview of it, with the cursor left where it was, and whether the retag
// popup is still open or not.
func TestRetag_ThePlaylistsFollow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		favorites bool
		enter     bool
		popup     bool
	}{
		{name: "inside P", enter: true},
		{name: "inside P with the retag popup open", enter: true, popup: true},
		{name: "inside Favorites", favorites: true, enter: true},
		{name: "P under the cursor at the root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, id := playlistOfTrust(t, newRetagTestModel(t), tc.favorites, tc.enter)
			before := m.Navigation.PlaylistNav().SelectedID()
			if tc.popup {
				m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())
			}

			if err := m.Core.Library().Retag(asLowTrust()); err != nil {
				t.Fatalf("retag: %v", err)
			}
			m = pumpUntilMatch(t, m, isRetagFinished)

			tracks, err := m.Core.Playlists().Tracks(id)
			if err != nil || len(tracks) == 0 {
				t.Fatalf("the playlist holds %+v (err %v), want the track of trust", tracks, err)
			}
			for i := range tracks {
				if tracks[i].Artist != "Low" || tracks[i].Title != "Canada" {
					t.Fatalf("the playlist holds %+v, want Low - Canada", tracks)
				}
			}
			// Reloaded while the Library view is on screen.
			nav := m.Navigation.PlaylistNav()
			if tc.enter {
				items := nav.CurrentItems()
				if len(items) != len(tracks) {
					t.Errorf("the playlist navigator lists %v, want %d rows", playlistNodeNames(items), len(tracks))
				}
				for _, n := range items {
					if n.Track().Artist != "Low" || n.Track().Title != "Canada" {
						t.Errorf("the playlist navigator lists %v, want 01. Low - Canada", playlistNodeNames(items))
						break
					}
				}
			}
			if got := nav.SelectedID(); got != before {
				t.Errorf("the cursor is on %q after the retag, want on %q", got, before)
			}
			if msg := m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error popup = %q, want none", msg)
			}

			m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyF3})
			got := playlistNavView(m)
			if !strings.Contains(got, "01. Low - Canada") || strings.Contains(got, "low - canada") {
				t.Errorf("the Playlists view shows, after the retag:\n%s\nwant 01. Low - Canada", got)
			}
		})
	}
}

// A retag that failed wrote some files, or all of them: the playlists follow
// what the library holds, and the failure is still told.
func TestRetagFinished_WithAFailureThePlaylistsStillFollow(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail func(*core.RetagFinished)
		want string
	}{
		{name: "a file failed", fail: func(f *core.RetagFinished) {
			f.Files = []core.FileProgress{{Path: "/music/low/trust/canada.mp3", State: core.FileFailed, Err: errors.New("read-only file")}}
		}, want: "Retag of low - trust: 1 of 1 files failed"},
		{name: "with an error", fail: func(f *core.RetagFinished) {
			f.Err = errors.New("database is locked")
		}, want: "Retag of low - trust: library not updated: database is locked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := playlistOfTrust(t, newRetagTestModel(t), false, true)

			if err := m.Core.Library().Retag(asLowTrust()); err != nil {
				t.Fatalf("retag: %v", err)
			}
			m, msg := nextMatch(t, m, isRetagFinished)
			finished, _ := msg.(ServiceRetagMsg).Event.(core.RetagFinished)
			tc.fail(&finished)
			m, _ = updateModel(t, m, ServiceRetagMsg{Event: finished})

			items := m.Navigation.PlaylistNav().CurrentItems()
			if len(items) != 2 || items[0].Track().Title != "Canada" || items[1].Track().Title != "Canada" {
				t.Errorf("the playlist navigator lists %v, want Canada twice as the library has it", playlistNodeNames(items))
			}
			if got := m.Popups.ErrorMsg(); got != tc.want {
				t.Errorf("error popup = %q, want %q", got, tc.want)
			}
		})
	}
}

// newRetagGroupedTestModel builds a model whose library holds "first" by
// "abc", "trust" by "low" and "last" by "zed", one file each but trust, which
// has trustFiles, and the albums others (album artist, album) of one file
// each, shown in the library's album view grouped by album artist and sorted
// by album artist, ascending, with the cursor on trust.
func newRetagGroupedTestModel(t *testing.T, trustFiles int, others ...[2]string) Model {
	t.Helper()
	return newRetagGroupedTestModelOn(t, filepath.Join(t.TempDir(), "waves.db"), trustFiles, others...)
}

// newRetagGroupedTestModelOn is newRetagGroupedTestModel on a core over
// database, for a test that starts waves again on it.
func newRetagGroupedTestModelOn(t *testing.T, database string, trustFiles int, others ...[2]string) Model {
	t.Helper()
	source := t.TempDir()
	write := func(artist, album string, number int) {
		title := fmt.Sprintf("track %d", number)
		path := filepath.Join(source, artist, album, title+".mp3")
		tagstest.WriteMP3(t, path, core.FileTags{Artist: artist, AlbumArtist: artist, Album: album, Title: title, TrackNumber: number})
	}
	write("abc", "first", 1)
	for i := 1; i <= trustFiles; i++ {
		write("low", "trust", i)
	}
	write("zed", "last", 1)
	for _, other := range others {
		write(other[0], other[1], 1)
	}
	m := startTestModel(t, core.Options{Database: database})
	if err := m.Core.Library().AddSource(source); err != nil {
		t.Fatalf("add source: %v", err)
	}
	m = pumpUntil[LibraryScanCompleteMsg](t, m)
	m.Navigation.SetViewMode(navctl.ViewLibrary)
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
	view := m.Navigation.AlbumView()
	view.SetSettings(albumview.Settings{Settings: albumpreset.Settings{
		GroupFields:    []albumpreset.GroupField{albumpreset.GroupFieldArtist},
		GroupSortOrder: albumpreset.SortAsc,
		SortCriteria:   []albumpreset.SortCriterion{{Field: albumpreset.SortFieldArtist, Order: albumpreset.SortAsc}},
	}})
	if err := view.Refresh(); err != nil {
		t.Fatalf("album view: %v", err)
	}
	if !view.SelectAlbum("low", "trust") {
		t.Fatal("trust is not in the album view")
	}
	return m
}

// assertAlbumViewOn fails unless the album view's cursor is on the album
// albumArtist - album.
func assertAlbumViewOn(t *testing.T, m Model, albumArtist, album string) {
	t.Helper()
	got := m.Navigation.AlbumView().SelectedAlbum()
	if got == nil || got.AlbumArtist != albumArtist || got.Album != album {
		t.Errorf("album view selects %+v, want %s - %s", got, albumArtist, album)
	}
}

// The retag renames the album under the cursor, which the sorting moves: the
// cursor follows it, whether the popup is left open to its end or not.
func TestRetag_TheAlbumViewFollowsTheAlbumRenamed(t *testing.T) {
	for _, popup := range []bool{false, true} {
		t.Run(fmt.Sprintf("popup open %v", popup), func(t *testing.T) {
			m := newRetagGroupedTestModel(t, 1)
			if popup {
				m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())
			}

			m, finished := retagUntilFinished(t, m, asLowTrust())
			assertAlbumViewOn(t, m, "Low", "Trust")

			m, _ = updateModel(t, m, completeOf(finished))
			assertAlbumViewOn(t, m, "Low", "Trust")
		})
	}
}

// The retag gives the album the names of one the library already has: the
// cursor is on that album.
func TestRetag_TheAlbumViewFollowsTheAlbumIntoOneItJoins(t *testing.T) {
	m := newRetagGroupedTestModel(t, 1, [2]string{"Low", "Trust"})

	if err := m.Core.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	m = pumpUntilMatch(t, m, isRetagFinished)

	assertAlbumViewOn(t, m, "Low", "Trust")
}

// The user moved off the album while it was retagged: the cursor stays where
// the user put it.
func TestRetag_TheAlbumViewLeavesTheCursorTheUserMoved(t *testing.T) {
	m := newRetagGroupedTestModel(t, 1)

	if err := m.Core.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	m.Navigation.AlbumView().SelectAlbum("abc", "first")
	m = pumpUntilMatch(t, m, isRetagFinished)

	assertAlbumViewOn(t, m, "abc", "first")
}

// A file the release has no track for keeps its tags: the album is still in
// the library under its old names, and the cursor stays on it.
func TestRetag_ThatFailsForAFileLeavesTheAlbumViewOnTheOldAlbum(t *testing.T) {
	m := newRetagGroupedTestModel(t, 2)

	if err := m.Core.Library().Retag(asLowTrust()); err != nil { // one track only
		t.Fatalf("retag: %v", err)
	}
	m, msg := nextMatch(t, m, isRetagFinished)
	finished, _ := msg.(ServiceRetagMsg).Event.(core.RetagFinished)
	m, _ = updateModel(t, m, msg)

	if got := workFailure("Retag", "low - trust", finished.Files, finished.Err); got != "Retag of low - trust: 1 of 2 files failed" {
		t.Errorf("the retag ended as %q, want one file of two failed", got)
	}
	for _, album := range [][2]string{{"low", "trust"}, {"Low", "Trust"}} {
		if ids, err := m.Core.Library().AlbumTrackIDs(album[0], album[1]); err != nil || len(ids) != 1 {
			t.Errorf("tracks of %s - %s = %v (err %v), want one", album[0], album[1], ids, err)
		}
	}
	assertAlbumViewOn(t, m, "low", "trust")
}

// The popup was left open on a retag that failed for a file: once the user
// closes it, the album view is still on the album the files that failed keep.
func TestRetagComplete_AfterAFailureLeavesTheAlbumViewOnTheOldAlbum(t *testing.T) {
	m := newRetagGroupedTestModel(t, 2)
	m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())

	m, _ = retagUntilFinished(t, m, asLowTrust()) // one track only
	assertAlbumViewOn(t, m, "low", "trust")

	// What the popup emits on Enter after a failure (internal/ui/retag).
	m, _ = updateModel(t, m, retag.ActionMsg(retag.Complete{AlbumArtist: "low", Album: "trust"}))
	assertAlbumViewOn(t, m, "low", "trust")
	if m.Popups.IsVisible(popupctl.Retag) {
		t.Error("the retag popup is still open")
	}
}

// The retag ends while another view is on screen: back on the album view,
// the cursor is on the album under the names the retag wrote.
func TestRetag_TheAlbumViewFollowsTheAlbumRenamedOffScreen(t *testing.T) {
	m := newRetagGroupedTestModel(t, 1)
	m.Navigation.SetViewMode(navctl.ViewPlaylists)

	m, _ = retagUntilFinished(t, m, asLowTrust())
	m.Navigation.SetViewMode(navctl.ViewLibrary)

	assertAlbumViewOn(t, m, "Low", "Trust")
}

// The user quits right after a retag renamed the album under the cursor,
// whatever the way out: the next start is on that album under its new names.
func TestRetag_TheNextStartIsOnTheAlbumRenamedBeforeTheQuit(t *testing.T) {
	quits := []struct {
		name string
		msg  tea.Msg
	}{
		{"quit key", keyMsg("q")},
		{"quit from outside the model (SIGTERM)", tea.QuitMsg{}},
		{"interrupt (SIGINT)", tea.InterruptMsg{}},
	}
	for _, quit := range quits {
		t.Run(quit.name, func(t *testing.T) {
			database := filepath.Join(t.TempDir(), "waves.db")
			// Abba is the first album: the one a start that finds no saved album is on.
			m := newRetagGroupedTestModelOn(t, database, 1, [2]string{"Abba", "Arrival"})
			m, _ = updateModel(t, m, keyMsg(keyEscape)) // the scan report
			m.SaveNavigationState()                     // where the user last moved
			m, _ = retagUntilFinished(t, m, asLowTrust())
			assertAlbumViewOn(t, m, "Low", "Trust")

			p := newHeadlessProgram(m)
			done := startRun(p, m.Core)
			p.Send(quit.msg)
			_ = waitRun(t, done) //nolint:errcheck // an interrupt ends Run with tea.ErrInterrupted

			next := startTestModel(t, core.Options{Database: database})
			if !next.Navigation.IsAlbumViewActive() {
				t.Fatal("the next start does not show the album view")
			}
			assertAlbumViewOn(t, next, "Low", "Trust")
		})
	}
}

// The queue panel names a queued track of the retagged album as the library
// now does, as soon as the retag is done: it reads the queue from the core.
func TestRetag_TheQueuePanelNamesTheTrackAsTheLibraryNowDoes(t *testing.T) {
	m := newRetagTestModel(t)
	m.Layout.ShowQueue()
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	tracks, err := m.Core.Library().Tracks("low", "trust")
	if err != nil || len(tracks) != 1 {
		t.Fatalf("tracks of trust = %+v (err %v), want one", tracks, err)
	}
	m.Core.Playback().AddTracks(core.TracksFromLibrary(tracks)...)

	if err := m.Core.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	m = pumpUntilMatch(t, m, isRetagFinished)

	view := testutil.StripANSI(m.Layout.QueuePanel().View())
	line := testutil.FindLine(view, "Canada")
	if !strings.Contains(line, "Canada") || !strings.Contains(line, "Low") {
		t.Errorf("the queue panel shows:\n%s\nwant the track as Canada by Low", view)
	}
	if strings.Contains(view, "canada") {
		t.Errorf("the queue panel still shows the old title:\n%s", view)
	}
}

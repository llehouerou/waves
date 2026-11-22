package app

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/playlistnav"
)

// mixFixture is a trustFixture whose playlists hold a folder "Rock" and, in
// it, a playlist "Mix" of the three tracks of Trust, with the Playlists view
// shown at its root.
type mixFixture struct {
	*trustFixture
	folder, playlist int64
}

func newMixFixture(t *testing.T) *mixFixture {
	t.Helper()
	f := newTrustFixture(t)
	pls := f.m.Core.Playlists()
	folder, err := pls.CreateFolder(nil, "Rock")
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	playlist, err := pls.Create(&folder, "Mix")
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	if err := pls.AddTracks(playlist, f.ids); err != nil {
		t.Fatalf("add tracks: %v", err)
	}
	f.m.Navigation.SetViewMode(navctl.ViewPlaylists)
	f.m.SetFocus(navctl.FocusNavigator)
	return &mixFixture{trustFixture: f, folder: folder, playlist: playlist}
}

// spoilPlaylist makes the row of the playlist Mix one Get cannot read.
func (f *mixFixture) spoilPlaylist(t *testing.T) {
	t.Helper()
	if _, err := f.db.Exec(`UPDATE playlists SET created_at = 'not a date' WHERE id = ?`, f.playlist); err != nil {
		t.Fatalf("spoil row: %v", err)
	}
}

// mixResult is the deep search result of the playlist Mix, or of its track
// at position when position is not negative.
func (f *mixFixture) mixResult(position int) playlistnav.DeepSearchItem {
	return playlistnav.DeepSearchItem{PlaylistDeepSearchItem: core.PlaylistDeepSearchItem{
		PlaylistID:    f.playlist,
		PlaylistName:  "Mix",
		FolderPath:    "Rock",
		TrackPosition: position,
	}}
}

// assertPlaylistNavStill checks that the playlists navigator is still in
// current, on selected.
func assertPlaylistNavStill(t *testing.T, m Model, current, selected string) {
	t.Helper()
	nav := m.Navigation.PlaylistNav()
	if got := nav.Current().ID(); got != current {
		t.Errorf("the navigator moved to %q, want %q", got, current)
	}
	if got := nav.SelectedID(); got != selected {
		t.Errorf("the navigator selects %q, want %q", got, selected)
	}
}

func TestPlaylistDeepSearchResult_TellsANodeItCannotReadAndStaysWhereItWas(t *testing.T) {
	cases := map[string]struct {
		spoil    func(f *mixFixture, t *testing.T)
		position int
	}{
		"a track of a playlist whose tracks cannot be read": {
			spoil:    func(f *mixFixture, t *testing.T) { f.spoil(t, 3) },
			position: 0,
		},
		"a playlist whose row cannot be read": {
			spoil:    (*mixFixture).spoilPlaylist,
			position: -1,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newMixFixture(t)
			nav := f.m.Navigation.PlaylistNav()
			current, selected := nav.Current().ID(), nav.SelectedID()
			tc.spoil(f, t)

			f.m.navigateToSearchResult(f.mixResult(tc.position))

			assertErrorTold(t, f.m, errmsg.OpNavigate)
			assertPlaylistNavStill(t, f.m, current, selected)
		})
	}
}

func TestPlaylistDeepSearchResult_SaysNothingOfAPlaylistDeletedSince(t *testing.T) {
	for name, position := range map[string]int{"the playlist": -1, "a track of it": 0} {
		t.Run(name, func(t *testing.T) {
			f := newMixFixture(t)
			nav := f.m.Navigation.PlaylistNav()
			current, selected := nav.Current().ID(), nav.SelectedID()
			if err := f.m.Core.Playlists().Delete(f.playlist); err != nil {
				t.Fatalf("delete playlist: %v", err)
			}

			f.m.navigateToSearchResult(f.mixResult(position))

			if msg := f.m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error popup = %q, want none", msg)
			}
			assertPlaylistNavStill(t, f.m, current, selected)
		})
	}
}

func TestStartup_ASavedPlaylistsSelectionOnATrackItCannotReadIsTold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		database := filepath.Join(t.TempDir(), "waves.db")
		var playlist int64
		seedDatabase(t, database, func(db *sql.DB) {
			insertSingleAlbum(t, db, "Low", "Trust", 3)
			res, err := db.Exec(`INSERT INTO playlists (name, created_at, last_used_at) VALUES ('Mix', 1, 1)`)
			if err != nil {
				t.Fatalf("insert playlist: %v", err)
			}
			if playlist, err = res.LastInsertId(); err != nil {
				t.Fatalf("playlist id: %v", err)
			}
			if _, err := db.Exec(`
				INSERT INTO playlist_tracks (playlist_id, position, library_track_id)
				SELECT ?, track_number - 1, id FROM library_tracks WHERE album = 'Trust'
			`, playlist); err != nil {
				t.Fatalf("insert playlist tracks: %v", err)
			}
			if _, err := db.Exec(`UPDATE library_tracks SET year = 'not a year' WHERE path = '/lib/Trust/2.mp3'`); err != nil {
				t.Fatalf("spoil row: %v", err)
			}
		})
		seedNavigation(t, database, state.NavigationState{
			ViewMode:            string(navctl.ViewPlaylists),
			PlaylistsSelectedID: fmt.Sprintf("playlists:track:%d:0", playlist),
		})
		s := startupOn(t, database, false)

		s.send(s.init)
		s.deliver(time.Second)

		if s.m.loadingState != loadingDone {
			t.Fatalf("loading state = %v, want done", s.m.loadingState)
		}
		assertErrorTold(t, s.m, errmsg.OpSelectionRestore)
		if cur := s.m.Navigation.PlaylistNav().Current(); cur.Level() != playlistnav.LevelRoot {
			t.Errorf("the playlists navigator opened in %q, want the root", cur.ID())
		}
	})
}

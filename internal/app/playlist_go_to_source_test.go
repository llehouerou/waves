package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/playlistnav"
)

// locateFixture is a trustFixture with the Playlists view shown, for `L` on
// the tracks of Trust queued.
type locateFixture struct {
	*trustFixture
}

func newLocateFixture(t *testing.T) *locateFixture {
	t.Helper()
	f := newTrustFixture(t)
	f.m.Navigation.SetViewMode(navctl.ViewPlaylists)
	return &locateFixture{trustFixture: f}
}

// folder creates a folder of playlists in parent (nil for the root).
func (f *locateFixture) folder(t *testing.T, parent *int64, name string) *int64 {
	t.Helper()
	id, err := f.m.Core.Playlists().CreateFolder(parent, name)
	if err != nil {
		t.Fatalf("create folder %q: %v", name, err)
	}
	return &id
}

// playlist creates a playlist in folder (nil for the root) holding the
// tracks of Trust numbered by tracks (from 1), in that order.
func (f *locateFixture) playlist(t *testing.T, folder *int64, name string, tracks ...int) int64 {
	t.Helper()
	id, err := f.m.Core.Playlists().Create(folder, name)
	if err != nil {
		t.Fatalf("create playlist %q: %v", name, err)
	}
	f.add(t, id, tracks...)
	return id
}

// add appends the tracks of Trust numbered by tracks (from 1) to a playlist.
func (f *locateFixture) add(t *testing.T, playlist int64, tracks ...int) {
	t.Helper()
	ids := make([]int64, 0, len(tracks))
	for _, n := range tracks {
		ids = append(ids, f.ids[n-1])
	}
	if err := f.m.Core.Playlists().AddTracks(playlist, ids); err != nil {
		t.Fatalf("add tracks: %v", err)
	}
}

// favorite puts the track n of Trust (from 1) in Favorites.
func (f *locateFixture) favorite(t *testing.T, n int) {
	t.Helper()
	if _, err := f.m.Core.Playlists().ToggleFavorites([]int64{f.ids[n-1]}); err != nil {
		t.Fatalf("favorite: %v", err)
	}
}

// atRoot shows the root of the playlists as they are now, the cursor on the
// first node the view lists.
func (f *locateFixture) atRoot(t *testing.T) {
	t.Helper()
	f.enter(t, playlistnav.NewSource(f.m.Core.Playlists()).Root().ID())
}

// focus puts the playlists navigator on the node id.
func (f *locateFixture) focus(t *testing.T, id string) {
	t.Helper()
	if ok, err := f.m.Navigation.PlaylistNav().FocusByID(id); !ok || err != nil {
		t.Fatalf("FocusByID(%q) = %v, %v", id, ok, err)
	}
}

// enter puts the playlists navigator in the container id.
func (f *locateFixture) enter(t *testing.T, id string) {
	t.Helper()
	if ok, err := f.m.Navigation.PlaylistNav().NavigateTo(id); !ok || err != nil {
		t.Fatalf("NavigateTo(%q) = %v, %v", id, ok, err)
	}
}

// locateSecond queues Trust, focuses the queue, puts its cursor on the
// second track and presses L.
func (f *locateFixture) locateSecond(t *testing.T) Model {
	t.Helper()
	f.queueTrust(t)
	return pressWithoutEvent(t, f.m, "j", "L")
}

// assertOnTrack checks that the navigator is in the playlist, on its track
// at position, and has the focus.
func assertOnTrack(t *testing.T, m Model, playlist int64, position int) {
	t.Helper()
	nav := m.Navigation.PlaylistNav()
	if id := nav.Current().PlaylistID(); id == nil || *id != playlist {
		t.Errorf("the navigator is in %q, want the playlist %d", nav.Current().ID(), playlist)
	}
	if sel := nav.Selected(); sel == nil || sel.PlaylistID() == nil || *sel.PlaylistID() != playlist || sel.Position() != position {
		t.Errorf("the navigator selects %q, want the track at %d of the playlist %d", nav.SelectedID(), position, playlist)
	}
	if !m.Navigation.IsNavigatorFocused() {
		t.Error("the focus is not on the navigator")
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
}

// assertStayed checks that the navigator is still in current, on selected,
// and that the focus is still on the queue.
func assertStayed(t *testing.T, m Model, current, selected string) {
	t.Helper()
	assertPlaylistNavStill(t, m, current, selected)
	if !m.Navigation.IsQueueFocused() {
		t.Error("the focus left the queue")
	}
}

func TestLocate_InThePlaylistsViewGoesToThePlaylistThatHoldsTheTrack(t *testing.T) {
	f := newLocateFixture(t)
	mix := f.playlist(t, nil, "Mix", 1, 2)
	f.atRoot(t)

	m := f.locateSecond(t)

	assertOnTrack(t, m, mix, 1)
}

func TestLocate_GoesIntoAPlaylistKeptInAFolder(t *testing.T) {
	f := newLocateFixture(t)
	rock := f.folder(t, nil, "Rock")
	indie := f.folder(t, rock, "Indie")
	mix := f.playlist(t, indie, "Mix", 3, 2)
	f.atRoot(t)

	m := f.locateSecond(t)

	assertOnTrack(t, m, mix, 1)
	m, _ = updateModel(t, m, keyMsg("h"))
	if id := m.Navigation.PlaylistNav().Current().FolderID(); id == nil || *id != *indie {
		t.Errorf("back from Mix, the navigator is in %q, want the folder Indie", m.Navigation.PlaylistNav().Current().ID())
	}
}

func TestLocate_GoesToFavoritesBeforeAPlaylistKeptInAFolder(t *testing.T) {
	f := newLocateFixture(t)
	rock := f.folder(t, nil, "Rock")
	f.playlist(t, rock, "Mix", 2)
	f.favorite(t, 1)
	f.favorite(t, 2)
	f.atRoot(t)
	if sel := f.m.Navigation.PlaylistNav().Selected(); sel == nil || sel.Level() != playlistnav.LevelFolder {
		t.Fatalf("the navigator selects %q, want the folder Rock", f.m.Navigation.PlaylistNav().SelectedID())
	}

	m := f.locateSecond(t)

	assertOnTrack(t, m, core.FavoritesPlaylistID, 1)
}

func TestLocate_GoesToThePlaylistTheViewListsFirst(t *testing.T) {
	cases := map[string]func(f *locateFixture, t *testing.T) (first int64){
		"a playlist of a folder before one of the root": func(f *locateFixture, t *testing.T) int64 {
			f.playlist(t, nil, "Alpha", 2)
			zed := f.folder(t, nil, "Zed")
			return f.playlist(t, zed, "Omega", 2)
		},
		"two playlists of the root, by name": func(f *locateFixture, t *testing.T) int64 {
			f.playlist(t, nil, "beta", 2)
			return f.playlist(t, nil, "Alpha", 2)
		},
		"a sub-folder before the playlists of its folder": func(f *locateFixture, t *testing.T) int64 {
			rock := f.folder(t, nil, "Rock")
			f.playlist(t, rock, "Alpha", 2)
			zed := f.folder(t, rock, "Zed")
			return f.playlist(t, zed, "Omega", 2)
		},
		"an earlier folder with everything under it": func(f *locateFixture, t *testing.T) int64 {
			bravo := f.folder(t, nil, "Bravo")
			f.playlist(t, bravo, "Alpha", 2)
			alpha := f.folder(t, nil, "Alpha")
			deep := f.folder(t, alpha, "Deep")
			return f.playlist(t, deep, "Omega", 2)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newLocateFixture(t)
			first := setup(f, t)
			f.atRoot(t)

			m := f.locateSecond(t)

			assertOnTrack(t, m, first, 0)
		})
	}
}

func TestLocate_StaysInThePlaylistTheNavigatorIsInWhenItHoldsTheTrack(t *testing.T) {
	f := newLocateFixture(t)
	rock := f.folder(t, nil, "Rock")
	mix := f.playlist(t, rock, "Mix", 1, 3, 2)
	f.favorite(t, 2)
	f.focus(t, playlistnav.TrackNodeID(mix, 0))

	m := f.locateSecond(t)

	assertOnTrack(t, m, mix, 2)
}

func TestLocate_GoesIntoThePlaylistTheNavigatorIsOnWhenItHoldsTheTrack(t *testing.T) {
	for name, folder := range map[string]bool{"in a folder": true, "at the root": false} {
		t.Run(name, func(t *testing.T) {
			f := newLocateFixture(t)
			var parent *int64
			if folder {
				parent = f.folder(t, nil, "Rock")
			}
			f.playlist(t, parent, "Alpha", 2)
			mix := f.playlist(t, parent, "Mix", 3, 2)
			f.favorite(t, 2)
			f.focus(t, playlistnav.PlaylistNodeID(mix))

			m := f.locateSecond(t)

			assertOnTrack(t, m, mix, 1)
		})
	}
}

func TestLocate_LeavesAPlaylistThatDoesNotHoldTheTrack(t *testing.T) {
	cases := map[string]struct {
		favorite bool
		enter    bool
	}{
		"in it, to Favorites":                        {favorite: true, enter: true},
		"on it, to Favorites":                        {favorite: true},
		"in it, to the first playlist that holds it": {enter: true},
		"on it, to the first playlist that holds it": {},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newLocateFixture(t)
			other := f.playlist(t, nil, "Alpha", 1)
			holder := f.playlist(t, nil, "Mix", 3, 2)
			if tc.favorite {
				f.favorite(t, 2)
			}
			if tc.enter {
				f.enter(t, playlistnav.PlaylistNodeID(other))
			} else {
				f.focus(t, playlistnav.PlaylistNodeID(other))
			}

			m := f.locateSecond(t)

			if tc.favorite {
				assertOnTrack(t, m, core.FavoritesPlaylistID, 0)
			} else {
				assertOnTrack(t, m, holder, 1)
			}
		})
	}
}

func TestLocate_GoesToTheFirstPlaceOfATrackThePlaylistHoldsTwice(t *testing.T) {
	f := newLocateFixture(t)
	mix := f.playlist(t, nil, "Mix", 1, 2, 3, 2)
	f.focus(t, playlistnav.TrackNodeID(mix, 3))

	m := f.locateSecond(t)

	assertOnTrack(t, m, mix, 1)
}

func TestLocate_SaysNothingOfATrackInNoPlaylist(t *testing.T) {
	f := newLocateFixture(t)
	f.playlist(t, nil, "Mix", 1, 3)
	f.favorite(t, 1)
	f.atRoot(t)
	nav := f.m.Navigation.PlaylistNav()
	current, selected := nav.Current().ID(), nav.SelectedID()

	m := f.locateSecond(t)

	assertStayed(t, m, current, selected)
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
}

func TestLocate_SaysNothingOfATrackNotInTheLibrary(t *testing.T) {
	f := newLocateFixture(t)
	f.playlist(t, nil, "Mix", 1, 2, 3)
	f.atRoot(t)
	nav := f.m.Navigation.PlaylistNav()
	current, selected := nav.Current().ID(), nav.SelectedID()
	f.m.Core.Playback().AddTracks(core.Track{Path: "/elsewhere/song.mp3", Title: "Song"})
	f.m.queueEdited()
	f.m.Layout.ShowQueue()
	f.m.SetFocus(navctl.FocusQueue)

	m := pressWithoutEvent(t, f.m, "L")

	assertStayed(t, m, current, selected)
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
}

func TestLocate_TellsAPlaylistItCannotReadAndStaysOnTheQueue(t *testing.T) {
	f := newLocateFixture(t)
	f.playlist(t, nil, "Mix", 1, 2)
	f.atRoot(t)
	nav := f.m.Navigation.PlaylistNav()
	current, selected := nav.Current().ID(), nav.SelectedID()
	f.spoil(t, 1)

	m := f.locateSecond(t)

	assertErrorTold(t, m, errmsg.OpNavigate)
	assertStayed(t, m, current, selected)
}

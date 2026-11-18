package app

import (
	"path/filepath"
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/librarynav"
)

// acdcAlbums is a library whose artist holds a colon in their name, with two
// albums, one of them with a single track, next to another artist.
var acdcAlbums = []seededAlbum{
	{"AC:DC", "Back in Black", 2},
	{"AC:DC", "Live: 1975", 1},
	{"Other", "Other Album", 1},
}

// newMillerACDCModel is a model showing the Miller columns over acdcAlbums.
func newMillerACDCModel(t *testing.T) Model {
	t.Helper()
	m, _ := newBrowserDeleteTestModel(t, acdcAlbums...)
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeMiller)
	m.Navigation.SetFocus(m.Navigation.Focus())
	return m
}

// assertInACDC checks that the Miller columns are in the container of level
// and album of AC:DC (no album: the artist), and show the items named want.
func assertInACDC(t *testing.T, m Model, level librarynav.Level, album string, want ...string) {
	t.Helper()
	const artist = "AC:DC"
	nav := m.Navigation.LibraryNav()
	cur := nav.Current()
	if cur.Level() != level || cur.Artist() != artist || cur.Album() != album {
		t.Fatalf("the navigator is in level %v, artist %q, album %q; want level %v, artist %q, album %q",
			cur.Level(), cur.Artist(), cur.Album(), level, artist, album)
	}
	items := nav.CurrentItems()
	var got []string
	for _, item := range items {
		if item.Level() == librarynav.LevelTrack {
			got = append(got, item.Track().Title)
		} else {
			got = append(got, item.Album())
		}
	}
	if len(got) != len(want) {
		t.Fatalf("the navigator shows %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("the navigator shows %q, want %q", got, want)
		}
	}
}

func TestLibrarySearchResult_ReachesAnArtistWithAColonInItsName(t *testing.T) {
	m := newMillerACDCModel(t)

	m.HandleLibrarySearchResult(core.SearchResult{Type: core.ResultArtist, Artist: "AC:DC"})

	assertInACDC(t, m, librarynav.LevelArtist, "", "Back in Black", "Live: 1975")
}

func TestLibrarySearchResult_ReachesAnAlbumOfAnArtistWithAColonInItsName(t *testing.T) {
	m := newMillerACDCModel(t)

	m.HandleLibrarySearchResult(core.SearchResult{Type: core.ResultAlbum, Artist: "AC:DC", Album: "Back in Black"})

	assertInACDC(t, m, librarynav.LevelAlbum, "Back in Black",
		"Back in Black Track 1", "Back in Black Track 2")
}

func TestFocusByID_SelectsAnAlbumOfAnArtistWithAColonInItsName(t *testing.T) {
	m := newMillerACDCModel(t)
	m.HandleLibrarySearchResult(core.SearchResult{Type: core.ResultArtist, Artist: "AC:DC"})
	m = press(t, m, "j")
	id := m.Navigation.LibraryNav().SelectedID()
	m.HandleLibrarySearchResult(core.SearchResult{Type: core.ResultArtist, Artist: "Other"})

	if !m.Navigation.LibraryNav().FocusByID(id) {
		t.Fatalf("FocusByID(%q) = false", id)
	}

	assertInACDC(t, m, librarynav.LevelArtist, "", "Back in Black", "Live: 1975")
	if got := m.Navigation.LibraryNav().Selected(); got == nil || got.Album() != "Live: 1975" {
		t.Errorf("selected %+v, want the album Live: 1975", got)
	}
}

func TestLibraryMillerDelete_TheLastTrackOfAnAlbumOfAnArtistWithAColonLeavesForTheArtist(t *testing.T) {
	m := newMillerACDCModel(t)
	m.HandleLibrarySearchResult(core.SearchResult{Type: core.ResultAlbum, Artist: "AC:DC", Album: "Live: 1975"})
	assertInACDC(t, m, librarynav.LevelAlbum, "Live: 1975", "Live: 1975 Track 1")

	m = deleteSelectedTrack(t, m, 0)

	assertInACDC(t, m, librarynav.LevelArtist, "", "Back in Black")
	if got := m.Navigation.LibraryNav().Selected(); got == nil || got.Album() != "Back in Black" {
		t.Errorf("selected %+v after the removal, want the album Back in Black", got)
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error shown: %s", msg)
	}
}

// An earlier version joined the names of a library selection with colons:
// the selection it saved is not restored, and the Miller columns start in a
// container that has items.
func TestStartup_ALibrarySelectionSavedWithAColonInANameStartsInAContainerWithItems(t *testing.T) {
	for _, saved := range []string{"library:album:AC:DC:Back in Black", "library:artist:AC:DC"} {
		t.Run(saved, func(t *testing.T) {
			database := filepath.Join(t.TempDir(), "waves.db")
			mgr, err := state.OpenAt(database)
			if err != nil {
				t.Fatalf("open state: %v", err)
			}
			seedLibrary(t, mgr.DB(), t.TempDir(), acdcAlbums...)
			if err := mgr.Close(); err != nil {
				t.Fatalf("close state: %v", err)
			}
			seedNavigation(t, database, state.NavigationState{
				ViewMode:          string(navctl.ViewLibrary),
				LibrarySubMode:    "miller",
				LibrarySelectedID: saved,
			})

			init := initialized(t, newWaitingModel(t, database, t.TempDir()))

			libNav, ok := init.LibNav.(navigator.Model[librarynav.Node])
			if !ok {
				t.Fatalf("library navigator = %T", init.LibNav)
			}
			if cur := libNav.Current(); len(libNav.CurrentItems()) == 0 {
				t.Errorf("the navigator starts in level %v, artist %q, album %q, with no item",
					cur.Level(), cur.Artist(), cur.Album())
			}
		})
	}
}

package app

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/librarynav"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// trustFixture is a model whose library holds Low's "Trust", three tracks
// all readable when the model starts, and a second connection on its
// database, to make the library's reads fail while the model runs.
type trustFixture struct {
	m   Model
	db  *sql.DB
	ids []int64 // the tracks of Trust, in playback order
}

func newTrustFixture(t *testing.T) *trustFixture {
	t.Helper()
	database := filepath.Join(t.TempDir(), "waves.db")
	seedDatabase(t, database, func(db *sql.DB) { insertSingleAlbum(t, db, "Low", "Trust", 3) })
	m := startTestModel(t, core.Options{Database: database})
	second, err := state.OpenAt(database)
	if err != nil {
		t.Fatalf("open a second connection: %v", err)
	}
	t.Cleanup(func() { second.Close() })
	ids, err := m.Core.Library().AlbumTrackIDs("Low", "Trust")
	if err != nil || len(ids) != 3 {
		t.Fatalf("track IDs of Trust = %v (err %v), want 3", ids, err)
	}
	return &trustFixture{m: m, db: second.DB(), ids: ids}
}

// spoil makes the row of the track n of Trust (from 1) one TrackByID,
// Tracks and ArtistTracks cannot read; AlbumTrackIDs still lists it.
func (f *trustFixture) spoil(t *testing.T, n int) {
	t.Helper()
	spoilTrack(t, f.db, f.ids[n-1])
}

// spoilTrack makes the library row of the track id one TrackByID cannot read.
func spoilTrack(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	if _, err := db.Exec(`UPDATE library_tracks SET year = 'not a year' WHERE id = ?`, id); err != nil {
		t.Fatalf("spoil row: %v", err)
	}
}

// breakLibrary makes every read of the library fail.
func (f *trustFixture) breakLibrary(t *testing.T) {
	t.Helper()
	if _, err := f.db.Exec(`ALTER TABLE library_tracks RENAME TO library_tracks_gone`); err != nil {
		t.Fatalf("rename table: %v", err)
	}
}

// inAlbumView shows the album view on Trust.
func (f *trustFixture) inAlbumView(t *testing.T) {
	t.Helper()
	f.m.openAlbumView("Low", "Trust")
	if got := f.m.Navigation.AlbumView().SelectedAlbum(); got == nil || got.Album != "Trust" {
		t.Fatalf("album view on %+v, want Trust", got)
	}
}

// inBrowser shows the library browser on Low, in column, Trust selected.
func (f *trustFixture) inBrowser(t *testing.T, column librarybrowser.Column) {
	t.Helper()
	f.m.Navigation.SetViewMode(navctl.ViewLibrary)
	f.m.Navigation.SetLibrarySubMode(navctl.LibraryModeBrowser)
	f.m.SetFocus(navctl.FocusNavigator)
	browser := f.m.Navigation.LibraryBrowser()
	browser.SelectArtist("Low")
	browser.SelectAlbum("Trust")
	browser.SetActiveColumn(column)
	if browser.SelectedArtist() != "Low" || browser.SelectedAlbum() == nil || browser.SelectedAlbum().Name != "Trust" {
		t.Fatal("the browser is not on Low - Trust")
	}
}

// inMiller shows the Miller columns on the node id, selected.
func (f *trustFixture) inMiller(t *testing.T, id string) {
	t.Helper()
	f.m.Navigation.SetViewMode(navctl.ViewLibrary)
	f.m.Navigation.SetLibrarySubMode(navctl.LibraryModeMiller)
	f.m.SetFocus(navctl.FocusNavigator)
	if ok, err := f.m.Navigation.LibraryNav().FocusByID(id); !ok || err != nil {
		t.Fatalf("FocusByID(%q) = %v, %v", id, ok, err)
	}
}

// queueIDs are the IDs of the tracks in the queue, in its order.
func queueIDs(m Model) []int64 {
	tracks := m.Core.Playback().QueueTracks()
	ids := make([]int64, 0, len(tracks))
	for _, track := range tracks {
		ids = append(ids, track.ID)
	}
	return ids
}

// assertErrorTold checks that the error popup tells op failed.
func assertErrorTold(t *testing.T, m Model, op errmsg.Op) {
	t.Helper()
	if msg := m.Popups.ErrorMsg(); !strings.HasPrefix(msg, "Failed to "+string(op)+": ") {
		t.Errorf("error popup = %q, want %q told", msg, op)
	}
}

func TestQueueAlbum_QueuesNothingWhenATrackOfTheAlbumCannotBeRead(t *testing.T) {
	setups := map[string]func(f *trustFixture, t *testing.T){
		"album view": (*trustFixture).inAlbumView,
		"library browser": func(f *trustFixture, t *testing.T) {
			f.inBrowser(t, librarybrowser.ColumnAlbums)
		},
	}
	for name, setup := range setups {
		for _, key := range []string{"a", keyEnter} {
			t.Run(name+", "+key, func(t *testing.T) {
				f := newTrustFixture(t)
				setup(f, t)
				f.spoil(t, 2)

				m := press(t, f.m, key)

				if ids := queueIDs(m); len(ids) != 0 {
					t.Errorf("queue = %v, want nothing queued", ids)
				}
				if !m.Core.Playback().IsStopped() {
					t.Error("playback started")
				}
				assertErrorTold(t, m, errmsg.OpQueueAdd)
			})
		}
	}
}

func TestQueueAlbum_QueuesAnAlbumItCanReadWholeInPlaybackOrder(t *testing.T) {
	setups := map[string]func(f *trustFixture, t *testing.T){
		"album view": (*trustFixture).inAlbumView,
		"library browser": func(f *trustFixture, t *testing.T) {
			f.inBrowser(t, librarybrowser.ColumnAlbums)
		},
	}
	for name, setup := range setups {
		t.Run(name, func(t *testing.T) {
			f := newTrustFixture(t)
			setup(f, t)

			m := press(t, f.m, "a")

			if ids := queueIDs(m); len(ids) != 3 || ids[0] != f.ids[0] || ids[1] != f.ids[1] || ids[2] != f.ids[2] {
				t.Errorf("queue = %v, want %v", ids, f.ids)
			}
			if msg := m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error popup = %q, want none", msg)
			}
		})
	}
}

func TestExport_TellsASelectionWhoseTracksCannotBeListed(t *testing.T) {
	setups := map[string]func(f *trustFixture, t *testing.T){
		"album view, an album": (*trustFixture).inAlbumView,
		"library browser, an artist": func(f *trustFixture, t *testing.T) {
			f.inBrowser(t, librarybrowser.ColumnArtists)
		},
		"library browser, an album": func(f *trustFixture, t *testing.T) {
			f.inBrowser(t, librarybrowser.ColumnAlbums)
		},
		"Miller columns, an artist": func(f *trustFixture, t *testing.T) {
			f.inMiller(t, librarynav.ArtistNodeID("Low"))
		},
		"Miller columns, an album": func(f *trustFixture, t *testing.T) {
			f.inMiller(t, librarynav.AlbumNodeID("Low", "Trust"))
		},
	}
	for name, setup := range setups {
		t.Run(name, func(t *testing.T) {
			f := newTrustFixture(t)
			setup(f, t)
			f.breakLibrary(t)

			m := press(t, f.m, "e")

			assertErrorTold(t, m, errmsg.OpExportTracks)
			if m.Popups.Get(popupctl.Export) != nil {
				t.Error("the export popup opened")
			}
		})
	}
}

// queueTrust puts the tracks of Trust in the queue, and the focus on it.
func (f *trustFixture) queueTrust(t *testing.T) {
	t.Helper()
	for i, id := range f.ids {
		f.m.Core.Playback().AddTracks(core.Track{ID: id, Path: fmt.Sprintf("/lib/Trust/%d.mp3", i+1)})
	}
	f.m.queueEdited()
	f.m.Layout.ShowQueue()
	f.m.SetFocus(navctl.FocusQueue)
}

// librarySubModes put the library view in each of its sub-modes.
var librarySubModes = map[string]navctl.LibrarySubMode{
	"album view":      navctl.LibraryModeAlbum,
	"library browser": navctl.LibraryModeBrowser,
	"Miller columns":  navctl.LibraryModeMiller,
}

func TestGoToSource_TellsATrackItCannotReadAndStaysOnTheQueue(t *testing.T) {
	for name, mode := range librarySubModes {
		t.Run(name, func(t *testing.T) {
			f := newTrustFixture(t)
			f.m.Navigation.SetViewMode(navctl.ViewLibrary)
			f.m.Navigation.SetLibrarySubMode(mode)
			f.queueTrust(t)
			f.spoil(t, 2)

			f.m.handleGoToSource(queuepanel.GoToSource{TrackID: f.ids[1]})

			if f.m.Popups.ErrorMsg() == "" || !strings.Contains(f.m.Popups.ErrorMsg(), `"year"`) {
				t.Errorf("error popup = %q, want the failed read told", f.m.Popups.ErrorMsg())
			}
			if !f.m.Navigation.IsQueueFocused() {
				t.Error("the focus left the queue")
			}
		})
	}
}

func TestGoToSource_SaysNothingOfATrackTheLibraryDoesNotHave(t *testing.T) {
	for name, mode := range librarySubModes {
		t.Run(name, func(t *testing.T) {
			f := newTrustFixture(t)
			f.m.Navigation.SetViewMode(navctl.ViewLibrary)
			f.m.Navigation.SetLibrarySubMode(mode)
			f.queueTrust(t)

			f.m.handleGoToSource(queuepanel.GoToSource{TrackID: 999})

			if msg := f.m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error popup = %q, want none", msg)
			}
			if !f.m.Navigation.IsQueueFocused() {
				t.Error("the focus left the queue")
			}
		})
	}
}

func TestLibrarySearchResult_TellsATrackItCannotReadAndLeavesTheViewAsItIs(t *testing.T) {
	f := newTrustFixture(t)
	f.inAlbumView(t) // a track result goes to the Miller columns from there
	f.spoil(t, 2)

	f.m.HandleLibrarySearchResult(core.SearchResult{Type: core.ResultTrack, Artist: "Low", Album: "Trust", TrackID: f.ids[1]})

	if msg := f.m.Popups.ErrorMsg(); !strings.Contains(msg, `"year"`) {
		t.Errorf("error popup = %q, want the failed read told", msg)
	}
	if cur := f.m.Navigation.LibraryNav().Current(); cur.Level() != librarynav.LevelRoot {
		t.Errorf("the navigator moved to %q", cur.ID())
	}
	if !f.m.Navigation.IsAlbumViewActive() {
		t.Errorf("the view switched to library sub-mode %v, want the album view", f.m.Navigation.LibrarySubMode())
	}
}

func TestStartup_ASavedSelectionOnATrackItCannotReadIsToldAndStartsAtTheRoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		database := filepath.Join(t.TempDir(), "waves.db")
		var id int64
		seedDatabase(t, database, func(db *sql.DB) {
			insertSingleAlbum(t, db, "Low", "Trust", 3)
			row := db.QueryRow(`SELECT id FROM library_tracks WHERE path = '/lib/Trust/2.mp3'`)
			if err := row.Scan(&id); err != nil {
				t.Fatalf("track ID: %v", err)
			}
			spoilTrack(t, db, id)
		})
		seedNavigation(t, database, state.NavigationState{
			ViewMode:          string(navctl.ViewLibrary),
			LibrarySelectedID: librarynav.TrackNodeID(id),
		})
		s := startupOn(t, database, false)

		s.send(s.init)
		s.deliver(time.Second)

		if s.m.loadingState != loadingDone {
			t.Fatalf("loading state = %v, want done", s.m.loadingState)
		}
		if msg := s.m.Popups.ErrorMsg(); !strings.Contains(msg, `"year"`) {
			t.Errorf("error popup = %q, want the failed read told", msg)
		}
		if cur := s.m.Navigation.LibraryNav().Current(); cur.Level() != librarynav.LevelRoot {
			t.Errorf("the library navigator opened in %q, want the root", cur.ID())
		}
	})
}

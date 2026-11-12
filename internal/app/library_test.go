package app

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/librarynav"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// newLibraryTestModel builds a model whose core has a library and playlists,
// on a real database in a temp dir, and takes it through the initialization
// the app runs at startup. seed fills the database before the core opens it.
func newLibraryTestModel(t *testing.T, seed func(db *sql.DB)) Model {
	t.Helper()
	mgr, err := state.OpenAt(filepath.Join(t.TempDir(), "waves.db"))
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })
	if seed != nil {
		seed(mgr.DB())
	}

	mock := player.NewMock()
	c := core.New(core.Options{Player: mock, State: mgr})
	testPlayers.Store(c, mock)
	t.Cleanup(func() { c.Close() })

	m := Model{
		Navigation: navctl.New(),
		Core:       c,
		Layout:     NewLayoutManager(queuepanel.New(c.Playback())),
		Popups:     popupctl.New(),
		Keys:       keymap.NewResolver(keymap.Bindings),
		StateMgr:   mgr,
		initConfig: &initConfig{cfg: &config.Config{DefaultFolder: t.TempDir()}, stateMgr: mgr},
	}
	m, _ = updateModel(t, m, m.startInitialization()())
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Fatalf("initialization failed: %s", msg)
	}
	m.loadingState = loadingDone // skip the first launch splash
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// pumpUntil feeds the core's events to the model, as the running program
// does, until the model has handled one of type E.
func pumpUntil[E tea.Msg](t *testing.T, m Model) Model {
	t.Helper()
	for {
		next := make(chan tea.Msg, 1)
		go func() { next <- m.WatchServiceEvents()() }()
		select {
		case msg := <-next:
			m, _ = updateModel(t, m, msg)
			if _, ok := msg.(E); ok {
				return m
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no %T from the core", *new(E))
		}
	}
}

func TestLibraryScan_TheCoreRunsItAndTheTUIShowsItsProgressAndItsReport(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		// An album the library knows under a source that is gone.
		insertSingleAlbum(t, db, "Album Artist", "Album Title", 3)
		if _, err := db.Exec(`INSERT INTO library_sources (path, added_at) VALUES ('/lib', 1)`); err != nil {
			t.Fatalf("insert source: %v", err)
		}
	})
	if got := m.Navigation.LibraryBrowser().SelectedArtist(); got != "Album Artist" {
		t.Fatalf("selected artist before the scan = %q, want Album Artist", got)
	}

	m, _ = updateModel(t, m, keyMsg("f"))
	m, _ = updateModel(t, m, keyMsg("r"))

	m = pumpUntil[LibraryScanProgressMsg](t, m)
	if m.LibraryScanJob == nil {
		t.Error("no scan job in the job bar while the scan runs")
	}

	m = pumpUntil[LibraryScanCompleteMsg](t, m)
	if m.LibraryScanJob != nil {
		t.Errorf("scan job still in the job bar after the scan: %+v", m.LibraryScanJob)
	}
	if !m.Popups.IsVisible(popupctl.ScanReport) {
		t.Error("the scan report is not shown")
	}
	if got := m.Navigation.LibraryBrowser().SelectedArtist(); got != "" {
		t.Errorf("selected artist after the scan = %q, want none: the album is gone", got)
	}
}

func TestQueueAction_QueuesTheSelectedArtistOfTheBrowser(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Album Artist", "Album Title", 3)
	})

	m.HandleQueueAction(QueueAdd)

	queue := m.Core.Playback().QueueTracks()
	if len(queue) != 3 || queue[0].Title != "Album Track 1" || queue[2].Path != "/lib/Album Title/3.mp3" {
		t.Errorf("queue = %+v, want the album's three tracks", queue)
	}
}

func TestCollectTrackIDsFromNode(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Artist", "First", 2)
		insertSingleAlbum(t, db, "Artist", "Second", 1)
	})
	source := librarynav.NewSource(m.Core.Library())
	children := func(parent librarynav.Node) []librarynav.Node {
		t.Helper()
		nodes, err := source.Children(parent)
		if err != nil || len(nodes) == 0 {
			t.Fatalf("children of %q = %v (err %v)", parent.ID(), nodes, err)
		}
		return nodes
	}
	root := source.Root()
	artist := children(root)[0]
	album := children(artist)[0]
	track := children(album)[1]

	if ids := m.collectTrackIDsFromNode(&root); ids != nil {
		t.Errorf("root: %v, want nothing", ids)
	}
	if ids := m.collectTrackIDsFromNode(&artist); len(ids) != 3 {
		t.Errorf("artist: %v, want its 3 tracks", ids)
	}
	if ids := m.collectTrackIDsFromNode(&album); len(ids) != 2 {
		t.Errorf("album: %v, want its 2 tracks", ids)
	}
	if ids := m.collectTrackIDsFromNode(&track); len(ids) != 1 || ids[0] != track.TrackID() {
		t.Errorf("track: %v, want [%d]", ids, track.TrackID())
	}
}

func TestPlaylistInput_CreatesThePlaylistAndOpensIt(t *testing.T) {
	m := newLibraryTestModel(t, nil)

	result, _ := m.processPlaylistInput(PlaylistInputContext{Mode: InputNewPlaylist}, "Mix")
	m, _ = result.(Model)

	if got := m.Navigation.PlaylistNav().Current().DisplayName(); got != "Mix" {
		t.Fatalf("the navigator is in %q, want in the new playlist Mix", got)
	}
	if lists, err := m.Core.Playlists().List(nil); err != nil || len(lists) != 2 {
		t.Errorf("playlists = %+v (err %v), want Favorites and Mix", lists, err)
	}
}

func TestToggleFavorite_GoesThroughTheCore(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Album Artist", "Album Title", 1)
	})
	track := m.Navigation.LibraryBrowser().SelectedTrack()
	if track == nil {
		t.Fatal("no track selected in the browser")
	}

	m.handleToggleFavorite([]int64{track.ID})

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Fatalf("toggle failed: %s", msg)
	}
	favorites, err := m.Core.Playlists().FavoriteTrackIDs()
	if err != nil || !favorites[track.ID] {
		t.Errorf("favorites = %v (err %v), want the track in", favorites, err)
	}
}

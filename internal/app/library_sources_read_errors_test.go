package app

import (
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

const emptyLibraryScreen = "No library sources configured"

// nullableSources rebuilds the library_sources table of db with a path that
// accepts NULL, holding sources: a row with no path, which Sources cannot
// read, can then be added. The schema's CREATE TABLE IF NOT EXISTS keeps it.
func nullableSources(t *testing.T, db *sql.DB, sources ...string) {
	t.Helper()
	if _, err := db.Exec(`
		DROP TABLE library_sources;
		CREATE TABLE library_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT UNIQUE,
			added_at INTEGER NOT NULL
		);
	`); err != nil {
		t.Fatalf("rebuild library_sources: %v", err)
	}
	for i, source := range sources {
		if _, err := db.Exec(`INSERT INTO library_sources (path, added_at) VALUES (?, ?)`, source, i); err != nil {
			t.Fatalf("insert source: %v", err)
		}
	}
}

// spoilSourcesAfter makes every event (INSERT or DELETE) on the sources of
// db, nullableSources built, add a row Sources cannot read: the write
// succeeds, the read that follows fails.
func spoilSourcesAfter(t *testing.T, db *sql.DB, event string) {
	t.Helper()
	if _, err := db.Exec(`
		CREATE TRIGGER spoil_sources AFTER ` + event + ` ON library_sources
		BEGIN
			INSERT INTO library_sources (path, added_at) VALUES (NULL, 9999);
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
}

// assertNoEmptyLibraryScreen checks that the panels render the library, not
// the screen of a library with no source.
func assertNoEmptyLibraryScreen(t *testing.T, m Model) string {
	t.Helper()
	panels := testutil.StripANSI(m.renderPanels())
	if strings.Contains(panels, emptyLibraryScreen) {
		t.Errorf("the panels render the empty-library screen:\n%s", panels)
	}
	return panels
}

// removeSourceAt opens the sources popup and removes the source at index
// (from 0), confirmed.
func removeSourceAt(t *testing.T, m Model, index int) Model {
	t.Helper()
	m, _ = updateModel(t, m, keyMsg("f"))
	m, _ = updateModel(t, m, keyMsg("p"))
	for range index {
		m, _ = updateModel(t, m, keyMsg("j"))
	}
	m, cmd := updateModel(t, m, keyMsg("d"))
	if cmd == nil {
		t.Fatal("removing a source asked for no track count")
	}
	m, _ = updateModel(t, m, cmd())
	m, cmd = updateModel(t, m, keyMsg("y"))
	if cmd == nil {
		t.Fatal("confirming removed no source")
	}
	m, _ = updateModel(t, m, cmd())
	return m
}

func TestLibrarySources_RemovingTheLastSourceShowsTheEmptyLibraryScreen(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Low", "Trust", 3)
		if _, err := db.Exec(`INSERT INTO library_sources (path, added_at) VALUES ('/lib', 0)`); err != nil {
			t.Fatalf("insert source: %v", err)
		}
	})
	m.Navigation.SetViewMode(navctl.ViewLibrary)

	m = removeSourceAt(t, m, 0)

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
	if got := listedSources(m); len(got) != 0 {
		t.Errorf("listed sources = %v, want none", got)
	}
	if panels := testutil.StripANSI(m.renderPanels()); !strings.Contains(panels, emptyLibraryScreen) {
		t.Errorf("a library with no source left does not render the empty-library screen:\n%s", panels)
	}
}

func TestLibrarySources_AFailedReadAfterAnAddKeepsTheListedSources(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Low", "Trust", 3)
		nullableSources(t, db, "/lib")
		spoilSourcesAfter(t, db, "INSERT")
	})
	m.Navigation.SetViewMode(navctl.ViewLibrary)

	m = addSourceTyped(t, m, t.TempDir())

	assertErrorTold(t, m, errmsg.OpSourceLoad)
	if got := listedSources(m); !slices.Equal(got, []string{"/lib"}) {
		t.Errorf("listed sources = %v, want [/lib]", got)
	}
	if view := testutil.StripANSI(m.Popups.LibrarySources().View()); strings.Contains(view, "No sources configured") {
		t.Errorf("the sources popup lists no source:\n%s", view)
	}
	assertNoEmptyLibraryScreen(t, m)
}

func TestLibrarySources_AFailedReadAfterAFirstSourceIsAddedShowsTheLibrary(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		nullableSources(t, db)
		spoilSourcesAfter(t, db, "INSERT")
	})
	m.Navigation.SetViewMode(navctl.ViewLibrary)
	if panels := testutil.StripANSI(m.renderPanels()); !strings.Contains(panels, emptyLibraryScreen) {
		t.Fatalf("a library with no source does not render the empty-library screen:\n%s", panels)
	}

	m = addSourceTyped(t, m, t.TempDir())

	assertErrorTold(t, m, errmsg.OpSourceLoad)
	assertNoEmptyLibraryScreen(t, m)
}

func TestLibrarySources_AFailedReadAfterARemovalKeepsTheListedSources(t *testing.T) {
	m := newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Low", "Trust", 3)
		insertSingleAlbum(t, db, "Other Artist", "Other", 2)
		nullableSources(t, db, "/lib/Trust", "/lib/Other")
		spoilSourcesAfter(t, db, "DELETE")
	})
	m.Navigation.SetViewMode(navctl.ViewLibrary)
	if panels := testutil.StripANSI(m.renderPanels()); !strings.Contains(panels, "Other Artist") {
		t.Fatalf("the Library view does not list Other Artist before the removal:\n%s", panels)
	}

	m = removeSourceAt(t, m, 1)

	assertErrorTold(t, m, errmsg.OpSourceLoad)
	want := []string{"/lib/Trust", "/lib/Other"}
	if got := listedSources(m); !slices.Equal(got, want) {
		t.Errorf("listed sources = %v, want %v", got, want)
	}
	panels := assertNoEmptyLibraryScreen(t, m)
	if strings.Contains(panels, "Other Artist") {
		t.Errorf("the Library view still lists Other Artist, whose source was removed:\n%s", panels)
	}
	if !strings.Contains(panels, "Low") {
		t.Errorf("the Library view does not list Low:\n%s", panels)
	}
}

func TestStartup_AFailedReadOfTheSourcesIsToldAndTheLibraryShown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		database := filepath.Join(t.TempDir(), "waves.db")
		seedDatabase(t, database, func(db *sql.DB) {
			insertSingleAlbum(t, db, "Low", "Trust", 3)
			nullableSources(t, db, "/lib")
		})
		seedNavigation(t, database, state.NavigationState{ViewMode: string(navctl.ViewLibrary)})
		s := startupOn(t, database, false)
		second, err := state.OpenAt(database)
		if err != nil {
			t.Fatalf("open a second connection: %v", err)
		}
		defer second.Close()
		if _, err := second.DB().Exec(`INSERT INTO library_sources (path, added_at) VALUES (NULL, 9999)`); err != nil {
			t.Fatalf("spoil sources: %v", err)
		}

		s.send(s.init)
		s.deliver(time.Second)

		if s.m.loadingState != loadingDone {
			t.Fatalf("loading state = %v, want done", s.m.loadingState)
		}
		assertErrorTold(t, s.m, errmsg.OpSourceLoad)
		panels := assertNoEmptyLibraryScreen(t, s.m)
		if !strings.Contains(panels, "Low") {
			t.Errorf("the Library view does not list Low:\n%s", panels)
		}
	})
}

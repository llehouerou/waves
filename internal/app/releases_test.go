package app

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// newReleasesTestModel builds a model whose library has an album by Low, and
// whose core has cached, a moment ago, a release of theirs out today: the
// core asks ListenBrainz nothing, which only the core's own tests replace.
func newReleasesTestModel(t *testing.T) Model {
	t.Helper()
	return newLibraryTestModel(t, func(db *sql.DB) {
		insertSingleAlbum(t, db, "Low", "Trust", 3)
		_, err := db.Exec(`
			INSERT INTO fresh_releases
				(release_group_mbid, artist_credit_name, norm_artist, release_name,
				 release_date, primary_type, listen_count, fetched_at)
			VALUES ('rg', 'Low', ?, 'HEY WHAT', date('now'), 'Album', 0, strftime('%s', 'now'))
		`, core.NormalizeTitle("Low"))
		if err != nil {
			t.Fatalf("cache a release: %v", err)
		}
	})
}

// openNewReleases opens the new releases list and lets it read what it shows.
func openNewReleases(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = updateModel(t, m, keyMsg("f"))
	m = press(t, m, "n")
	if m.Popups.Download() == nil {
		t.Fatal("the new releases list is not open")
	}
	return m
}

func TestNewReleases_TheListShowsWhatTheCoreLists(t *testing.T) {
	m := openNewReleases(t, newReleasesTestModel(t))

	if view := testutil.StripANSI(m.Popups.Download().View()); !strings.Contains(view, "HEY WHAT") {
		t.Errorf("the list does not show the release the core has cached:\n%s", view)
	}
}

// The refresh and the warm-up run in the core, whether the list is open or
// not: their end reaches the list when it is, as a discreet line for a
// failure, never as an error popup.
func TestNewReleases_AFailedRefreshIsToldInTheList(t *testing.T) {
	m := openNewReleases(t, newReleasesTestModel(t))

	m, cmd := updateModel(t, m, ServiceReleasesMsg{Event: core.ReleasesRefreshed{Err: errors.New("listenbrainz status 503")}})

	if view := testutil.StripANSI(m.Popups.Download().View()); !strings.Contains(view, "503") || !strings.Contains(view, "HEY WHAT") {
		t.Errorf("the list does not tell the failure next to what it showed:\n%s", view)
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want the failure left to the list", msg)
	}
	if cmd == nil {
		t.Error("expected the command that waits for the next core event")
	}
}

func TestNewReleases_TheirEventsAreDroppedWhenTheListIsClosed(t *testing.T) {
	m := newReleasesTestModel(t)

	m, cmd := updateModel(t, m, ServiceReleasesMsg{Event: core.ReleasesRefreshed{Err: errors.New("listenbrainz status 503")}})

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want nothing shown: nobody is looking at the list", msg)
	}
	if cmd == nil {
		t.Error("expected the command that waits for the next core event")
	}
}

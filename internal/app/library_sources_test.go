package app

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/ui/testutil"
)

// TestLibrarySources_TheConfirmationCountsTheTracksNoOtherSourceHolds asks
// to remove each of two nested sources, /lib and /lib/Inner: the inner one
// would delete no track, the outer one the tracks outside the inner one.
func TestLibrarySources_TheConfirmationCountsTheTracksNoOtherSourceHolds(t *testing.T) {
	cases := []struct {
		name  string
		moves int // cursor moves down to the source removed
		want  string
	}{
		{"the outer source", 0, "This will remove 2 tracks from the library."},
		{"the inner source", 1, "No tracks will be affected."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newLibraryTestModel(t, func(db *sql.DB) {
				insertSingleAlbum(t, db, "Outer Artist", "Outer", 2)
				insertSingleAlbum(t, db, "Inner Artist", "Inner", 3)
				for i, source := range []string{"/lib", "/lib/Inner"} {
					if _, err := db.Exec(`INSERT INTO library_sources (path, added_at) VALUES (?, ?)`, source, i); err != nil {
						t.Fatalf("insert source: %v", err)
					}
				}
			})

			m, _ = updateModel(t, m, keyMsg("f"))
			m, _ = updateModel(t, m, keyMsg("p"))
			for range tc.moves {
				m, _ = updateModel(t, m, keyMsg("j"))
			}
			m, cmd := updateModel(t, m, keyMsg("d"))
			if cmd == nil {
				t.Fatal("removing a source asked for no track count")
			}
			m, _ = updateModel(t, m, cmd())

			view := testutil.StripANSI(m.Popups.LibrarySources().View())
			if !strings.Contains(view, tc.want) {
				t.Errorf("confirmation does not say %q:\n%s", tc.want, view)
			}
		})
	}
}

package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

// addSourceTyped opens the sources popup, types path as the new source and
// runs the command the popup returns.
func addSourceTyped(t *testing.T, m Model, path string) Model {
	t.Helper()
	m, _ = updateModel(t, m, keyMsg("f"))
	m, _ = updateModel(t, m, keyMsg("p"))
	m, _ = updateModel(t, m, keyMsg("a"))
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(path)})
	m, cmd := updateModel(t, m, keyMsg(keyEnter))
	if cmd == nil {
		t.Fatalf("typing %s added no source", path)
	}
	m, _ = updateModel(t, m, cmd())
	return m
}

// listedSources returns the sources the sources popup lists.
func listedSources(m Model) []string {
	var sources []string
	for line := range strings.SplitSeq(testutil.StripANSI(m.Popups.LibrarySources().View()), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ">"))
		if strings.HasPrefix(line, string(filepath.Separator)) {
			sources = append(sources, line)
		}
	}
	return sources
}

func TestLibrarySources_ADirectoryTypedWithATrailingSlashIsAlreadyASource(t *testing.T) {
	music := t.TempDir()
	m := newLibraryTestModel(t, func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO library_sources (path, added_at) VALUES (?, 0)`, music); err != nil {
			t.Fatalf("insert source: %v", err)
		}
	})

	m = addSourceTyped(t, m, music+"/")

	if got := m.Popups.ErrorMsg(); got != "Source already exists" {
		t.Errorf("error = %q, want %q", got, "Source already exists")
	}
	if got := listedSources(m); !slices.Equal(got, []string{music}) {
		t.Errorf("listed sources = %v, want %v", got, []string{music})
	}
}

func TestLibrarySources_ANewDirectoryIsListedAtItsCleanPath(t *testing.T) {
	root := t.TempDir()
	music := filepath.Join(root, "music")
	if err := os.Mkdir(music, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m := newLibraryTestModel(t, nil)

	m = addSourceTyped(t, m, root+"//music/")

	if got := m.Popups.ErrorMsg(); got != "" {
		t.Errorf("error = %q, want none", got)
	}
	if got := listedSources(m); !slices.Equal(got, []string{music}) {
		t.Errorf("listed sources = %v, want %v", got, []string{music})
	}
}

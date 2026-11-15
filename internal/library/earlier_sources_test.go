package library_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/state"
)

// A source an earlier version stored with a trailing separator is the
// directory itself once the database is opened again: it cannot be added a
// second time, under either spelling.
func TestAddSource_RefusesTheDirectoryOfASourceAnEarlierVersionStoredUnclean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waves.db")
	st, err := state.OpenAt(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	st.Close()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, query := range []string{
		`DELETE FROM schema_version`,
		`INSERT INTO schema_version (version) VALUES (20)`,
		`INSERT INTO library_sources (path, added_at) VALUES ('/x/music/', 1000)`,
	} {
		if _, err := raw.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	raw.Close()

	st, err = state.OpenAt(path)
	if err != nil {
		t.Fatalf("open database again: %v", err)
	}
	defer st.Close()
	lib := library.New(st.DB())

	if sources, err := lib.Sources(); err != nil || !slices.Equal(sources, []string{"/x/music"}) {
		t.Errorf("sources = %q (err %v), want /x/music", sources, err)
	}
	for _, source := range []string{"/x/music", "/x/music/"} {
		if _, err := lib.AddSource(source); !errors.Is(err, library.ErrSourceExists) {
			t.Errorf("AddSource(%q) = %v, want ErrSourceExists", source, err)
		}
	}
}

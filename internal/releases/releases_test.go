package releases

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/llehouerou/waves/internal/listenbrainz"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE fresh_releases (
			release_group_mbid TEXT PRIMARY KEY,
			artist_credit_name TEXT NOT NULL,
			norm_artist        TEXT NOT NULL,
			release_name       TEXT NOT NULL,
			release_date       TEXT NOT NULL,
			primary_type       TEXT,
			secondary_type     TEXT,
			listen_count       INTEGER NOT NULL DEFAULT 0,
			fetched_at         INTEGER NOT NULL
		);
		CREATE TABLE library_tracks (album_artist TEXT NOT NULL);
		CREATE TABLE lastfm_similar_artists (artist TEXT NOT NULL, similar_artist TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func release(mbid, artist, name string) listenbrainz.Release {
	return listenbrainz.Release{
		ReleaseGroupMBID: mbid,
		ArtistCreditName: artist,
		ReleaseName:      name,
		ReleaseDate:      time.Now().Format(time.DateOnly),
		PrimaryType:      "Album",
	}
}

func count(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

func TestReplace_ReplacesPayload(t *testing.T) {
	db := setupTestDB(t)
	cache := NewCache(db)

	first := []listenbrainz.Release{
		release("rg-1", "Mogwai", "The Bad Fire"),
		release("rg-2", "Sigur Rós", "Átta"),
	}
	if err := cache.Replace(first); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM fresh_releases`); n != 2 {
		t.Fatalf("got %d rows, want 2", n)
	}

	// Second fetch: rg-2 vanished, rg-3 appeared.
	second := []listenbrainz.Release{
		release("rg-1", "Mogwai", "The Bad Fire"),
		release("rg-3", "Jane Weaver", "Love In Constant Spacetime"),
	}
	if err := cache.Replace(second); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM fresh_releases WHERE release_group_mbid = 'rg-2'`); n != 0 {
		t.Error("vanished release still cached")
	}
	if n := count(t, db, `SELECT COUNT(DISTINCT fetched_at) FROM fresh_releases`); n != 1 {
		t.Errorf("got %d distinct fetched_at, want 1", n)
	}
}

func TestReplace_EmptyPayloadKeepsCache(t *testing.T) {
	db := setupTestDB(t)
	cache := NewCache(db)

	if err := cache.Replace([]listenbrainz.Release{release("rg-1", "Mogwai", "The Bad Fire")}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if err := cache.Replace(nil); err != nil {
		t.Fatalf("Replace empty: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM fresh_releases`); n != 1 {
		t.Errorf("empty payload emptied the cache: %d rows", n)
	}
}

func TestLastFetchAndStaleness(t *testing.T) {
	db := setupTestDB(t)
	cache := NewCache(db)

	stale, err := cache.IsStale()
	if err != nil {
		t.Fatalf("IsStale: %v", err)
	}
	if !stale {
		t.Error("empty cache should be stale")
	}

	if err := cache.Replace([]listenbrainz.Release{release("rg-1", "Mogwai", "The Bad Fire")}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	last, err := cache.lastFetch()
	if err != nil {
		t.Fatalf("lastFetch: %v", err)
	}
	if last.IsZero() {
		t.Error("lastFetch zero after a write")
	}
	if stale, _ := cache.IsStale(); stale {
		t.Error("fresh cache reported stale")
	}
}

func TestMatched(t *testing.T) {
	db := setupTestDB(t)
	cache := NewCache(db)

	payload := []listenbrainz.Release{
		release("rg-lib", "Mogwai", "The Bad Fire"),
		release("rg-accent", "Sigur Ros", "Atta"),   // library has "Sigur Rós"
		release("rg-disco", "Jane Weaver", "Love"),  // similar artist, not in library
		release("rg-none", "Nobody At All", "Miss"), // no match
		release("rg-va", "Various Artists", "Comp"),
		{ // missing primary type: unknown, filtered out of the default view
			ReleaseGroupMBID: "rg-untyped",
			ArtistCreditName: "Mogwai",
			ReleaseName:      "Untyped",
			ReleaseDate:      time.Now().Format(time.DateOnly),
		},
		{ // out of the 90-day window: a stale cache can still hold it
			ReleaseGroupMBID: "rg-old",
			ArtistCreditName: "Mogwai",
			ReleaseName:      "Ancient",
			ReleaseDate:      time.Now().AddDate(0, 0, -120).Format(time.DateOnly),
			PrimaryType:      "Album",
		},
	}
	if err := cache.Replace(payload); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	// "Various Artists" is a real album_artist in any library with compilations.
	for _, artist := range []string{"Mogwai", "Sigur Rós", "Various Artists"} {
		if _, err := db.Exec(`INSERT INTO library_tracks (album_artist) VALUES (?)`, artist); err != nil {
			t.Fatalf("insert library artist: %v", err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO lastfm_similar_artists (artist, similar_artist) VALUES ('Mogwai', 'Jane Weaver')`,
	); err != nil {
		t.Fatalf("insert similar artist: %v", err)
	}

	matched, err := cache.Matched()
	if err != nil {
		t.Fatalf("Matched: %v", err)
	}

	got := map[string]bool{}
	for _, r := range matched {
		got[r.ReleaseGroupMBID] = r.InLibrary
	}
	if len(matched) != 3 {
		t.Fatalf("got %d matches (%v), want 3", len(matched), got)
	}
	if !got["rg-lib"] {
		t.Error("library release missing or not marked InLibrary")
	}
	if _, ok := got["rg-accent"]; !ok || !got["rg-accent"] {
		t.Error("accented library artist not matched through NormalizeTitle")
	}
	if inLib, ok := got["rg-disco"]; !ok || inLib {
		t.Error("discovery missing or wrongly marked InLibrary")
	}
	for _, unwanted := range []string{"rg-none", "rg-va", "rg-untyped", "rg-old"} {
		if _, ok := got[unwanted]; ok {
			t.Errorf("%s should not be matched", unwanted)
		}
	}
}

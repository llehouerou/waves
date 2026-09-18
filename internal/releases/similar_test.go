package releases

import (
	"database/sql"
	"fmt"
	"testing"
)

func insertSimilar(t *testing.T, db *sql.DB, seed, similar string, score float64) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO lastfm_similar_artists (artist, similar_artist, match_score, fetched_at) VALUES (?, ?, ?, ?)`,
		seed, similar, score, 0,
	)
	if err != nil {
		t.Fatalf("insert similar: %v", err)
	}
}

func TestDiscoveries(t *testing.T) {
	db := setupTestDB(t)
	libraryArtists := map[string]bool{"mogwai": true, "sigur ros": true, "portishead": true}

	// Recommended by 3 seeds: top of the list.
	for _, seed := range []string{"Mogwai", "Sigur Rós", "Portishead"} {
		insertSimilar(t, db, seed, "Jane Weaver", 0.9)
	}
	// Recommended by 2 seeds: kept, below.
	insertSimilar(t, db, "Mogwai", "Explosions In The Sky", 0.8)
	insertSimilar(t, db, "Portishead", "Explosions In The Sky", 0.8)
	// Recommended by a single seed: dropped.
	insertSimilar(t, db, "Mogwai", "Lonely Recommendation", 0.99)
	// Already in the library: never a discovery, however many seeds name it.
	insertSimilar(t, db, "Mogwai", "Sigur Ros", 0.7)
	insertSimilar(t, db, "Portishead", "Sigur Ros", 0.7)
	// Seed outside the library (a similar artist Last.fm was queried about): ignored.
	insertSimilar(t, db, "Jane Weaver", "Stranger One", 0.9)
	insertSimilar(t, db, "Jane Weaver", "Stranger Two", 0.9)

	got, err := discoveries(db, libraryArtists)
	if err != nil {
		t.Fatalf("discoveries: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d discoveries (%v), want 2", len(got), got)
	}
	if got[0].Name != "Jane Weaver" || len(got[0].Seeds) != 3 {
		t.Errorf("first discovery = %+v, want Jane Weaver with 3 seeds", got[0])
	}
	if got[1].Name != "Explosions In The Sky" || len(got[1].Seeds) != 2 {
		t.Errorf("second discovery = %+v, want Explosions In The Sky with 2 seeds", got[1])
	}
}

func TestDiscoveries_CapsSimilarPerSeed(t *testing.T) {
	db := setupTestDB(t)
	libraryArtists := map[string]bool{"mogwai": true, "portishead": true}

	// 40 similar per seed, ordered by descending match score: only the first 30 count.
	for i := range 40 {
		score := 1.0 - float64(i)/100
		for _, seed := range []string{"Mogwai", "Portishead"} {
			insertSimilar(t, db, seed, fmt.Sprintf("Artist %02d", i), score)
		}
	}

	got, err := discoveries(db, libraryArtists)
	if err != nil {
		t.Fatalf("discoveries: %v", err)
	}
	if len(got) != maxSimilarPerSeed {
		t.Fatalf("got %d discoveries, want %d", len(got), maxSimilarPerSeed)
	}
	for _, d := range got {
		if d.Name >= "Artist 30" {
			t.Errorf("%s is past the per-seed cap", d.Name)
		}
	}
}

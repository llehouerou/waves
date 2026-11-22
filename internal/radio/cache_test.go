package radio

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/llehouerou/waves/internal/lastfm"
)

// setupTestDB creates an in-memory SQLite database with the cache tables.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS lastfm_similar_artists (
			artist TEXT NOT NULL,
			similar_artist TEXT NOT NULL,
			match_score REAL NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, similar_artist)
		);

		CREATE TABLE IF NOT EXISTS lastfm_artist_top_tracks (
			artist TEXT NOT NULL,
			track_name TEXT NOT NULL,
			playcount INTEGER NOT NULL,
			rank INTEGER NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, track_name)
		);

		CREATE TABLE IF NOT EXISTS lastfm_no_similar_artists (
			artist TEXT PRIMARY KEY,
			fetched_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS lastfm_no_artist_top_tracks (
			artist TEXT PRIMARY KEY,
			fetched_at INTEGER NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	return db
}

func TestCache_SimilarArtists_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	result, ok, err := cache.GetSimilarArtists("Unknown Artist")
	if err != nil {
		t.Fatalf("GetSimilarArtists failed: %v", err)
	}
	if ok || result != nil {
		t.Errorf("unknown artist = %v, %v, want nothing", result, ok)
	}
}

func TestCache_SimilarArtists_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	similar := []lastfm.SimilarArtist{
		{Name: "Similar Artist 1", MatchScore: 0.95},
		{Name: "Similar Artist 2", MatchScore: 0.85},
		{Name: "Similar Artist 3", MatchScore: 0.75},
	}

	if err := cache.SetSimilarArtists("Test Artist", similar); err != nil {
		t.Fatalf("SetSimilarArtists failed: %v", err)
	}

	result, ok, err := cache.GetSimilarArtists("Test Artist")
	if err != nil {
		t.Fatalf("GetSimilarArtists failed: %v", err)
	}
	if !ok {
		t.Fatal("expected the answer stored")
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 similar artists, got %d", len(result))
	}

	// Should be sorted by match score descending
	if result[0].Name != "Similar Artist 1" {
		t.Errorf("first similar = %q, want %q", result[0].Name, "Similar Artist 1")
	}
	if result[0].MatchScore != 0.95 {
		t.Errorf("first match score = %f, want 0.95", result[0].MatchScore)
	}
}

func TestCache_SimilarArtists_Replace(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	// Set initial data
	_ = cache.SetSimilarArtists("Test Artist", []lastfm.SimilarArtist{
		{Name: "Old Similar", MatchScore: 0.9},
	})

	// Replace with new data
	_ = cache.SetSimilarArtists("Test Artist", []lastfm.SimilarArtist{
		{Name: "New Similar", MatchScore: 0.8},
	})

	result, _, _ := cache.GetSimilarArtists("Test Artist")
	if len(result) != 1 {
		t.Fatalf("expected 1 similar artist, got %d", len(result))
	}
	if result[0].Name != "New Similar" {
		t.Errorf("similar = %q, want %q", result[0].Name, "New Similar")
	}
}

func TestCache_SimilarArtists_Expired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7) // 7 day TTL

	// Set data
	_ = cache.SetSimilarArtists("Test Artist", []lastfm.SimilarArtist{
		{Name: "Similar", MatchScore: 0.9},
	})

	// Manually set old fetched_at
	oldTime := time.Now().AddDate(0, 0, -10).Unix() // 10 days ago
	_, _ = db.Exec(`UPDATE lastfm_similar_artists SET fetched_at = ?`, oldTime)

	// Should return nil for expired data
	result, ok, err := cache.GetSimilarArtists("Test Artist")
	if err != nil {
		t.Fatalf("GetSimilarArtists failed: %v", err)
	}
	if ok || result != nil {
		t.Errorf("expired data = %v, %v, want nothing", result, ok)
	}
}

func TestCache_ArtistTopTracks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	result, ok, err := cache.GetArtistTopTracks("Unknown Artist")
	if err != nil {
		t.Fatalf("GetArtistTopTracks failed: %v", err)
	}
	if ok || result != nil {
		t.Errorf("unknown artist = %v, %v, want nothing", result, ok)
	}
}

func TestCache_ArtistTopTracks_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	tracks := []lastfm.TopTrack{
		{Name: "Track 1", Playcount: 1000000, Rank: 1},
		{Name: "Track 2", Playcount: 800000, Rank: 2},
		{Name: "Track 3", Playcount: 600000, Rank: 3},
	}

	if err := cache.SetArtistTopTracks("Test Artist", tracks); err != nil {
		t.Fatalf("SetArtistTopTracks failed: %v", err)
	}

	result, ok, err := cache.GetArtistTopTracks("Test Artist")
	if err != nil {
		t.Fatalf("GetArtistTopTracks failed: %v", err)
	}
	if !ok {
		t.Fatal("expected the answer stored")
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 top tracks, got %d", len(result))
	}

	// Should be sorted by rank ascending
	if result[0].Name != "Track 1" {
		t.Errorf("first track = %q, want %q", result[0].Name, "Track 1")
	}
	if result[0].Playcount != 1000000 {
		t.Errorf("first playcount = %d, want 1000000", result[0].Playcount)
	}
	if result[0].Rank != 1 {
		t.Errorf("first rank = %d, want 1", result[0].Rank)
	}
}

func TestCache_ArtistTopTracks_Expired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	_ = cache.SetArtistTopTracks("Test Artist", []lastfm.TopTrack{
		{Name: "Track", Playcount: 1000, Rank: 1},
	})

	// Manually set old fetched_at
	oldTime := time.Now().AddDate(0, 0, -10).Unix()
	_, _ = db.Exec(`UPDATE lastfm_artist_top_tracks SET fetched_at = ?`, oldTime)

	result, ok, _ := cache.GetArtistTopTracks("Test Artist")
	if ok || result != nil {
		t.Errorf("expired data = %v, %v, want nothing", result, ok)
	}
}

func TestCache_CleanExpired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	// Add data
	_ = cache.SetSimilarArtists("Artist 1", []lastfm.SimilarArtist{{Name: "Similar", MatchScore: 0.9}})
	_ = cache.SetArtistTopTracks("Artist 1", []lastfm.TopTrack{{Name: "Track", Playcount: 1000, Rank: 1}})
	_ = cache.SetSimilarArtists("Artist 2", nil)
	_ = cache.SetArtistTopTracks("Artist 2", nil)

	// Set old timestamps
	oldTime := time.Now().AddDate(0, 0, -10).Unix()
	_, _ = db.Exec(`UPDATE lastfm_similar_artists SET fetched_at = ?`, oldTime)
	_, _ = db.Exec(`UPDATE lastfm_artist_top_tracks SET fetched_at = ?`, oldTime)
	_, _ = db.Exec(`UPDATE lastfm_no_similar_artists SET fetched_at = ?`, oldTime)
	_, _ = db.Exec(`UPDATE lastfm_no_artist_top_tracks SET fetched_at = ?`, oldTime)

	// Clean expired
	if err := cache.CleanExpired(); err != nil {
		t.Fatalf("CleanExpired failed: %v", err)
	}

	// Verify all tables are empty
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM lastfm_similar_artists`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 similar artists after clean, got %d", count)
	}

	_ = db.QueryRow(`SELECT COUNT(*) FROM lastfm_artist_top_tracks`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 top tracks after clean, got %d", count)
	}

	_ = db.QueryRow(`SELECT COUNT(*) FROM lastfm_no_similar_artists`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 empty similar-artists answers after clean, got %d", count)
	}

	_ = db.QueryRow(`SELECT COUNT(*) FROM lastfm_no_artist_top_tracks`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 empty top-tracks answers after clean, got %d", count)
	}
}

func TestCache_CleanExpired_KeepsRecent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	// Add recent data
	_ = cache.SetSimilarArtists("Recent Artist", []lastfm.SimilarArtist{{Name: "Similar", MatchScore: 0.9}})

	// Add old data
	_ = cache.SetSimilarArtists("Old Artist", []lastfm.SimilarArtist{{Name: "Similar", MatchScore: 0.8}})
	oldTime := time.Now().AddDate(0, 0, -10).Unix()
	_, _ = db.Exec(`UPDATE lastfm_similar_artists SET fetched_at = ? WHERE artist = 'Old Artist'`, oldTime)

	// Clean expired
	_ = cache.CleanExpired()

	// Recent data should remain
	result, ok, _ := cache.GetSimilarArtists("Recent Artist")
	if !ok || result == nil {
		t.Error("recent data should be kept")
	}

	// Old data should be gone
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM lastfm_similar_artists WHERE artist = 'Old Artist'`).Scan(&count)
	if count != 0 {
		t.Error("old data should be cleaned")
	}
}

func TestCache_DifferentTTL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// 1 day TTL
	cache := NewCache(db, 1)

	_ = cache.SetSimilarArtists("Test Artist", []lastfm.SimilarArtist{{Name: "Similar", MatchScore: 0.9}})

	// Set 2 days ago
	oldTime := time.Now().AddDate(0, 0, -2).Unix()
	_, _ = db.Exec(`UPDATE lastfm_similar_artists SET fetched_at = ?`, oldTime)

	// Should be expired with 1 day TTL
	result, ok, _ := cache.GetSimilarArtists("Test Artist")
	if ok || result != nil {
		t.Error("data should be expired with 1 day TTL")
	}
}

func TestCache_MultipleArtists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	// Set data for multiple artists
	_ = cache.SetSimilarArtists("Artist 1", []lastfm.SimilarArtist{{Name: "Similar 1", MatchScore: 0.9}})
	_ = cache.SetSimilarArtists("Artist 2", []lastfm.SimilarArtist{{Name: "Similar 2", MatchScore: 0.8}})

	// Verify data is separate
	result1, _, _ := cache.GetSimilarArtists("Artist 1")
	result2, _, _ := cache.GetSimilarArtists("Artist 2")

	if result1[0].Name != "Similar 1" {
		t.Errorf("Artist 1 similar = %q, want %q", result1[0].Name, "Similar 1")
	}
	if result2[0].Name != "Similar 2" {
		t.Errorf("Artist 2 similar = %q, want %q", result2[0].Name, "Similar 2")
	}
}

func TestCache_IsExpired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	// Recent timestamp should not be expired
	recentTime := time.Now().Unix()
	if cache.isExpired(recentTime) {
		t.Error("recent timestamp should not be expired")
	}

	// Old timestamp should be expired
	oldTime := time.Now().AddDate(0, 0, -10).Unix()
	if !cache.isExpired(oldTime) {
		t.Error("10 days old timestamp should be expired with 7 day TTL")
	}

	// Exactly at TTL boundary - uses strict < comparison
	boundaryTime := time.Now().AddDate(0, 0, -7).Unix()
	if cache.isExpired(boundaryTime) {
		t.Error("timestamp exactly at TTL boundary should not be expired (uses strict < comparison)")
	}

	// Just past TTL boundary should be expired
	pastBoundaryTime := time.Now().AddDate(0, 0, -8).Unix()
	if !cache.isExpired(pastBoundaryTime) {
		t.Error("timestamp past TTL boundary should be expired")
	}
}

// Issue #242: the cache tells an artist never asked about, one whose answer
// was empty and one whose answer expired apart.

// answerKind is one of the two answers the cache remembers per artist.
type answerKind struct {
	name string
	age  string // makes an empty answer about an artist fetched at a time
	set  func(c *Cache, artist string, names ...string) error
	get  func(c *Cache, artist string) (names []string, ok bool, err error)
}

var answerKinds = []answerKind{
	{
		name: "similar artists",
		age:  `UPDATE lastfm_no_similar_artists SET fetched_at = ? WHERE artist = ?`,
		set: func(c *Cache, artist string, names ...string) error {
			similar := make([]lastfm.SimilarArtist, 0, len(names))
			for _, n := range names {
				similar = append(similar, lastfm.SimilarArtist{Name: n, MatchScore: 0.9})
			}
			return c.SetSimilarArtists(artist, similar)
		},
		get: func(c *Cache, artist string) ([]string, bool, error) {
			similar, ok, err := c.GetSimilarArtists(artist)
			names := make([]string, 0, len(similar))
			for _, s := range similar {
				names = append(names, s.Name)
			}
			return names, ok, err
		},
	},
	{
		name: "top tracks",
		age:  `UPDATE lastfm_no_artist_top_tracks SET fetched_at = ? WHERE artist = ?`,
		set: func(c *Cache, artist string, names ...string) error {
			tracks := make([]lastfm.TopTrack, 0, len(names))
			for i, n := range names {
				tracks = append(tracks, lastfm.TopTrack{Name: n, Playcount: 100, Rank: i + 1})
			}
			return c.SetArtistTopTracks(artist, tracks)
		},
		get: func(c *Cache, artist string) ([]string, bool, error) {
			tracks, ok, err := c.GetArtistTopTracks(artist)
			names := make([]string, 0, len(tracks))
			for _, t := range tracks {
				names = append(names, t.Name)
			}
			return names, ok, err
		},
	},
}

// expectAnswer fails the test unless the cache answers want about artist,
// or nothing when ok is false.
func expectAnswer(t *testing.T, kind answerKind, cache *Cache, artist string, wantOK bool, want ...string) {
	t.Helper()
	got, ok, err := kind.get(cache, artist)
	if err != nil {
		t.Fatalf("get %s of %s: %v", kind.name, artist, err)
	}
	if ok != wantOK || len(got) != len(want) {
		t.Fatalf("%s of %s = %v, %v, want %v, %v", kind.name, artist, got, ok, want, wantOK)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s of %s = %v, want %v", kind.name, artist, got, want)
		}
	}
}

func TestCache_TellsNeverAskedEmptyAndExpiredApart(t *testing.T) {
	for _, kind := range answerKinds {
		t.Run(kind.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			cache := NewCache(db, 7)

			if err := kind.set(cache, "Empty"); err != nil {
				t.Fatalf("set an empty answer: %v", err)
			}
			if err := kind.set(cache, "Expired"); err != nil {
				t.Fatalf("set an empty answer: %v", err)
			}
			aged := time.Now().AddDate(0, 0, -10).Unix()
			if _, err := db.Exec(kind.age, aged, "Expired"); err != nil {
				t.Fatalf("age the answer: %v", err)
			}

			expectAnswer(t, kind, cache, "Never asked", false)
			expectAnswer(t, kind, cache, "Empty", true)
			expectAnswer(t, kind, cache, "Expired", false)
		})
	}
}

func TestCache_KeepsOneAnswerPerArtist(t *testing.T) {
	for _, kind := range answerKinds {
		t.Run(kind.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			cache := NewCache(db, 7)

			if err := kind.set(cache, "Low"); err != nil {
				t.Fatalf("set an empty answer: %v", err)
			}
			if err := kind.set(cache, "Low", "One", "Two"); err != nil {
				t.Fatalf("set a list: %v", err)
			}
			expectAnswer(t, kind, cache, "Low", true, "One", "Two")

			if err := kind.set(cache, "Low"); err != nil {
				t.Fatalf("set an empty answer: %v", err)
			}
			expectAnswer(t, kind, cache, "Low", true)
		})
	}
}

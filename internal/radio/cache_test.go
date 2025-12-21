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

		CREATE TABLE IF NOT EXISTS lastfm_user_artist_tracks (
			artist TEXT NOT NULL,
			track_name TEXT NOT NULL,
			user_playcount INTEGER NOT NULL,
			fetched_at INTEGER NOT NULL,
			PRIMARY KEY (artist, track_name)
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

	result, err := cache.GetSimilarArtists("Unknown Artist")
	if err != nil {
		t.Fatalf("GetSimilarArtists failed: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for unknown artist, got %v", result)
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

	result, err := cache.GetSimilarArtists("Test Artist")
	if err != nil {
		t.Fatalf("GetSimilarArtists failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
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

	result, _ := cache.GetSimilarArtists("Test Artist")
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
	result, err := cache.GetSimilarArtists("Test Artist")
	if err != nil {
		t.Fatalf("GetSimilarArtists failed: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for expired data, got %v", result)
	}
}

func TestCache_ArtistTopTracks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	result, err := cache.GetArtistTopTracks("Unknown Artist")
	if err != nil {
		t.Fatalf("GetArtistTopTracks failed: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for unknown artist, got %v", result)
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

	result, err := cache.GetArtistTopTracks("Test Artist")
	if err != nil {
		t.Fatalf("GetArtistTopTracks failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
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

	result, _ := cache.GetArtistTopTracks("Test Artist")
	if result != nil {
		t.Errorf("expected nil for expired data, got %v", result)
	}
}

func TestCache_UserArtistTracks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	result, err := cache.GetUserArtistTracks("Unknown Artist")
	if err != nil {
		t.Fatalf("GetUserArtistTracks failed: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for unknown artist, got %v", result)
	}
}

func TestCache_UserArtistTracks_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	tracks := []lastfm.UserTrack{
		{Name: "Track 1", Playcount: 50},
		{Name: "Track 2", Playcount: 30},
		{Name: "Track 3", Playcount: 10},
	}

	if err := cache.SetUserArtistTracks("Test Artist", tracks); err != nil {
		t.Fatalf("SetUserArtistTracks failed: %v", err)
	}

	result, err := cache.GetUserArtistTracks("Test Artist")
	if err != nil {
		t.Fatalf("GetUserArtistTracks failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 user tracks, got %d", len(result))
	}

	// Should be sorted by user playcount descending
	if result[0].Name != "Track 1" {
		t.Errorf("first track = %q, want %q", result[0].Name, "Track 1")
	}
	if result[0].Playcount != 50 {
		t.Errorf("first playcount = %d, want 50", result[0].Playcount)
	}
}

func TestCache_UserArtistTracks_Expired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	_ = cache.SetUserArtistTracks("Test Artist", []lastfm.UserTrack{
		{Name: "Track", Playcount: 10},
	})

	// Manually set old fetched_at
	oldTime := time.Now().AddDate(0, 0, -10).Unix()
	_, _ = db.Exec(`UPDATE lastfm_user_artist_tracks SET fetched_at = ?`, oldTime)

	result, _ := cache.GetUserArtistTracks("Test Artist")
	if result != nil {
		t.Errorf("expected nil for expired data, got %v", result)
	}
}

func TestCache_CleanExpired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cache := NewCache(db, 7)

	// Add data
	_ = cache.SetSimilarArtists("Artist 1", []lastfm.SimilarArtist{{Name: "Similar", MatchScore: 0.9}})
	_ = cache.SetArtistTopTracks("Artist 1", []lastfm.TopTrack{{Name: "Track", Playcount: 1000, Rank: 1}})
	_ = cache.SetUserArtistTracks("Artist 1", []lastfm.UserTrack{{Name: "Track", Playcount: 10}})

	// Set old timestamps
	oldTime := time.Now().AddDate(0, 0, -10).Unix()
	_, _ = db.Exec(`UPDATE lastfm_similar_artists SET fetched_at = ?`, oldTime)
	_, _ = db.Exec(`UPDATE lastfm_artist_top_tracks SET fetched_at = ?`, oldTime)
	_, _ = db.Exec(`UPDATE lastfm_user_artist_tracks SET fetched_at = ?`, oldTime)

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

	_ = db.QueryRow(`SELECT COUNT(*) FROM lastfm_user_artist_tracks`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 user tracks after clean, got %d", count)
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
	result, _ := cache.GetSimilarArtists("Recent Artist")
	if result == nil {
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
	result, _ := cache.GetSimilarArtists("Test Artist")
	if result != nil {
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
	result1, _ := cache.GetSimilarArtists("Artist 1")
	result2, _ := cache.GetSimilarArtists("Artist 2")

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

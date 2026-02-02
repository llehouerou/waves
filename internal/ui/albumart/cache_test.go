package albumart

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCache_DefaultDir(t *testing.T) {
	// NewCache with empty string should use default directory
	cache, err := NewCache("")
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}

	// Verify cache directory was created
	userCache, _ := os.UserCacheDir()
	expectedDir := filepath.Join(userCache, "waves", "albumart")

	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("cache directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("cache path is not a directory")
	}

	// Clean up
	if cache.dir == expectedDir {
		// Don't delete user's actual cache
		t.Log("using default cache dir, skipping cleanup")
	}
}

func TestNewCache_CustomDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "custom", "cache")

	cache, err := NewCache(cacheDir)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}

	// Verify custom directory was created
	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("cache directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("cache path is not a directory")
	}

	if cache.dir != cacheDir {
		t.Errorf("cache.dir = %q, want %q", cache.dir, cacheDir)
	}
}

func TestCache_PutAndGet(t *testing.T) {
	cache := newTestCache(t)

	trackPath := "/music/artist/album/track.mp3"
	width, height := 8, 4
	pngData := []byte("fake png data for testing")

	// Put
	err := cache.Put(trackPath, width, height, pngData)
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Get
	retrieved := cache.Get(trackPath, width, height)
	if retrieved == nil {
		t.Fatal("Get() returned nil, expected data")
	}
	if !bytes.Equal(retrieved, pngData) {
		t.Errorf("Get() = %q, want %q", retrieved, pngData)
	}
}

func TestCache_Get_NotFound(t *testing.T) {
	cache := newTestCache(t)

	// Get non-existent entry
	retrieved := cache.Get("/nonexistent/track.mp3", 8, 4)
	if retrieved != nil {
		t.Errorf("Get() for nonexistent entry = %v, want nil", retrieved)
	}
}

func TestCache_Get_DifferentDimensions(t *testing.T) {
	cache := newTestCache(t)

	trackPath := "/music/track.mp3"
	pngData := []byte("test data")

	// Store at 8x4
	_ = cache.Put(trackPath, 8, 4, pngData)

	// Should not find at different dimensions
	if cache.Get(trackPath, 10, 5) != nil {
		t.Error("Get() should return nil for different width")
	}
	if cache.Get(trackPath, 8, 5) != nil {
		t.Error("Get() should return nil for different height")
	}

	// Should find at original dimensions
	if cache.Get(trackPath, 8, 4) == nil {
		t.Error("Get() should return data for original dimensions")
	}
}

func TestCache_Get_DifferentPaths(t *testing.T) {
	cache := newTestCache(t)

	pngData1 := []byte("data for track 1")
	pngData2 := []byte("data for track 2")

	_ = cache.Put("/track1.mp3", 8, 4, pngData1)
	_ = cache.Put("/track2.mp3", 8, 4, pngData2)

	// Each path should return its own data
	retrieved1 := cache.Get("/track1.mp3", 8, 4)
	retrieved2 := cache.Get("/track2.mp3", 8, 4)

	if !bytes.Equal(retrieved1, pngData1) {
		t.Errorf("track1 data = %q, want %q", retrieved1, pngData1)
	}
	if !bytes.Equal(retrieved2, pngData2) {
		t.Errorf("track2 data = %q, want %q", retrieved2, pngData2)
	}
}

func TestCache_Put_EmptyData(t *testing.T) {
	cache := newTestCache(t)

	// Put empty data should be a no-op
	err := cache.Put(testTrackPath, 8, 4, []byte{})
	if err != nil {
		t.Fatalf("Put() with empty data error: %v", err)
	}

	// Should not be cached
	retrieved := cache.Get(testTrackPath, 8, 4)
	if retrieved != nil {
		t.Error("empty data should not be cached")
	}

	// Put nil data should also be a no-op
	err = cache.Put("/track2.mp3", 8, 4, nil)
	if err != nil {
		t.Fatalf("Put() with nil data error: %v", err)
	}
}

func TestCache_Put_Overwrite(t *testing.T) {
	cache := newTestCache(t)

	oldData := []byte("old data")
	newData := []byte("new data")

	_ = cache.Put(testTrackPath, 8, 4, oldData)
	_ = cache.Put(testTrackPath, 8, 4, newData)

	retrieved := cache.Get(testTrackPath, 8, 4)
	if !bytes.Equal(retrieved, newData) {
		t.Errorf("Get() = %q, want %q (should be overwritten)", retrieved, newData)
	}
}

func TestCache_Clear(t *testing.T) {
	cache := newTestCache(t)

	// Add multiple entries
	_ = cache.Put("/track1.mp3", 8, 4, []byte("data1"))
	_ = cache.Put("/track2.mp3", 8, 4, []byte("data2"))
	_ = cache.Put("/track3.mp3", 8, 4, []byte("data3"))

	// Verify they exist
	if cache.Get("/track1.mp3", 8, 4) == nil {
		t.Fatal("track1 should exist before clear")
	}

	// Clear
	err := cache.Clear()
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	// All should be gone
	if cache.Get("/track1.mp3", 8, 4) != nil {
		t.Error("track1 should not exist after clear")
	}
	if cache.Get("/track2.mp3", 8, 4) != nil {
		t.Error("track2 should not exist after clear")
	}
	if cache.Get("/track3.mp3", 8, 4) != nil {
		t.Error("track3 should not exist after clear")
	}
}

func TestCache_Prune_RemovesOldEntries(t *testing.T) {
	cache := newTestCache(t)

	_ = cache.Put(testTrackPath, 8, 4, []byte("data"))

	// Set old modification time
	key := cache.cacheKey(testTrackPath, 8, 4)
	path := filepath.Join(cache.dir, key+".png")
	oldTime := time.Now().Add(-10 * 24 * time.Hour) // 10 days ago
	_ = os.Chtimes(path, oldTime, oldTime)

	// Prune with 7 day max age
	err := cache.Prune(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}

	// Should be pruned
	if cache.Get(testTrackPath, 8, 4) != nil {
		t.Error("old entry should be pruned")
	}
}

func TestCache_Prune_KeepsRecentEntries(t *testing.T) {
	cache := newTestCache(t)

	data := []byte("data")
	_ = cache.Put(testTrackPath, 8, 4, data)

	// Prune with 7 day max age (entry is fresh)
	err := cache.Prune(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}

	// Should still exist
	retrieved := cache.Get(testTrackPath, 8, 4)
	if retrieved == nil {
		t.Error("recent entry should not be pruned")
	}
}

func TestCache_Prune_MixedEntries(t *testing.T) {
	cache := newTestCache(t)

	// Add recent entry
	_ = cache.Put("/recent.mp3", 8, 4, []byte("recent"))

	// Add old entry
	_ = cache.Put("/old.mp3", 8, 4, []byte("old"))
	key := cache.cacheKey("/old.mp3", 8, 4)
	path := filepath.Join(cache.dir, key+".png")
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	_ = os.Chtimes(path, oldTime, oldTime)

	// Prune
	_ = cache.Prune(7 * 24 * time.Hour)

	// Recent should remain, old should be gone
	if cache.Get("/recent.mp3", 8, 4) == nil {
		t.Error("recent entry should remain")
	}
	if cache.Get("/old.mp3", 8, 4) != nil {
		t.Error("old entry should be pruned")
	}
}

func TestCache_Get_UpdatesMtime(t *testing.T) {
	cache := newTestCache(t)

	_ = cache.Put(testTrackPath, 8, 4, []byte("data"))

	// Set old modification time
	key := cache.cacheKey(testTrackPath, 8, 4)
	path := filepath.Join(cache.dir, key+".png")
	oldTime := time.Now().Add(-5 * 24 * time.Hour)
	_ = os.Chtimes(path, oldTime, oldTime)

	// Get should touch the file
	_ = cache.Get(testTrackPath, 8, 4)

	// Check mtime is updated
	info, _ := os.Stat(path)
	mtime := info.ModTime()

	// Mtime should be within last second
	if time.Since(mtime) > time.Second {
		t.Error("Get() should update mtime to current time")
	}
}

func TestCache_cacheKey_Deterministic(t *testing.T) {
	cache := newTestCache(t)

	trackPath := "/music/artist/album/track.mp3"

	key1 := cache.cacheKey(trackPath, 8, 4)
	key2 := cache.cacheKey(trackPath, 8, 4)

	if key1 != key2 {
		t.Errorf("cacheKey should be deterministic: %q != %q", key1, key2)
	}
}

func TestCache_cacheKey_IncludesDimensions(t *testing.T) {
	cache := newTestCache(t)

	key1 := cache.cacheKey(testTrackPath, 8, 4)
	key2 := cache.cacheKey(testTrackPath, 10, 5)

	if key1 == key2 {
		t.Error("cacheKey should differ for different dimensions")
	}
}

func TestCache_cacheKey_DifferentPaths(t *testing.T) {
	cache := newTestCache(t)

	key1 := cache.cacheKey("/track1.mp3", 8, 4)
	key2 := cache.cacheKey("/track2.mp3", 8, 4)

	if key1 == key2 {
		t.Error("cacheKey should differ for different paths")
	}
}

func TestCache_cacheKey_LongPath(t *testing.T) {
	cache := newTestCache(t)

	// Very long path
	longPath := "/music/" + string(make([]byte, 1000)) + "/track.mp3"
	key := cache.cacheKey(longPath, 8, 4)

	// Key should be a fixed-length hash
	if len(key) != 64 { // SHA256 hex = 64 chars
		t.Errorf("cacheKey length = %d, want 64 (SHA256 hex)", len(key))
	}
}

func TestCache_cacheKey_SpecialChars(t *testing.T) {
	cache := newTestCache(t)

	// Path with special characters
	specialPath := "/music/Artist (feat. Other)/Album [Deluxe]/01 - Track #1.mp3"
	key := cache.cacheKey(specialPath, 8, 4)

	// Key should be valid filename (hex only)
	for _, c := range key {
		if !isHexChar(c) {
			t.Errorf("cacheKey contains invalid char %q", string(c))
		}
	}
}

// isHexChar returns true if c is a valid hex digit (0-9, a-f).
func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}

func TestDefaultMaxAge(t *testing.T) {
	expected := 30 * 24 * time.Hour
	if DefaultMaxAge != expected {
		t.Errorf("DefaultMaxAge = %v, want %v", DefaultMaxAge, expected)
	}
}

// Helper functions

func newTestCache(t *testing.T) *Cache {
	t.Helper()

	dir := t.TempDir()
	cache, err := NewCache(dir)
	if err != nil {
		t.Fatalf("failed to create test cache: %v", err)
	}
	return cache
}

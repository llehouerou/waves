package albumart

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

// DefaultMaxAge is the default maximum age for cached album art (30 days).
const DefaultMaxAge = 30 * 24 * time.Hour

// Cache handles disk caching of resized album art.
type Cache struct {
	dir string
}

// NewCache creates a new album art cache.
// If cacheDir is empty, uses ~/.cache/waves/albumart/
// Automatically prunes entries older than 30 days on startup.
func NewCache(cacheDir string) (*Cache, error) {
	if cacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		cacheDir = filepath.Join(userCache, "waves", "albumart")
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}

	c := &Cache{dir: cacheDir}

	// Prune old entries in background on startup
	go c.Prune(DefaultMaxAge) //nolint:errcheck // best-effort cleanup

	return c, nil
}

// cacheKey generates a cache key from a track path and dimensions.
// Uses SHA256 hash to avoid filesystem issues with long paths or special chars.
func (c *Cache) cacheKey(trackPath string, width, height int) string {
	// Include dimensions in key so different sizes don't collide
	data := []byte(trackPath)
	data = append(data, byte(width), byte(height))
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Get retrieves cached PNG data for a track.
// Returns nil if not cached.
// Updates the file's modification time on hit to keep frequently used entries fresh.
func (c *Cache) Get(trackPath string, width, height int) []byte {
	key := c.cacheKey(trackPath, width, height)
	path := filepath.Join(c.dir, key+".png")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Touch the file to update mtime, keeping frequently accessed entries from expiring
	now := time.Now()
	os.Chtimes(path, now, now) //nolint:errcheck // best-effort touch

	return data
}

// Put stores PNG data in the cache.
func (c *Cache) Put(trackPath string, width, height int, data []byte) error {
	if len(data) == 0 {
		return nil // Don't cache empty results
	}

	key := c.cacheKey(trackPath, width, height)
	path := filepath.Join(c.dir, key+".png")

	return os.WriteFile(path, data, 0o600)
}

// Clear removes all cached album art.
func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			os.Remove(filepath.Join(c.dir, entry.Name()))
		}
	}
	return nil
}

// Prune removes cached entries older than maxAge.
func (c *Cache) Prune(maxAge time.Duration) error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(c.dir, entry.Name()))
		}
	}
	return nil
}

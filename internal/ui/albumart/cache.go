// Package albumart provides terminal-based album cover rendering using Kitty graphics protocol.
package albumart

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheDirName  = "waves/albumart"
	cacheMaxAge   = 30 * 24 * time.Hour // 30 days
	pruneInterval = 24 * time.Hour
)

// Cache provides disk-based caching for resized album art images.
type Cache struct {
	dir        string
	lastPruned time.Time
}

// NewCache creates a new disk cache in the user's cache directory.
// Returns nil if the cache directory cannot be created.
func NewCache(baseDir string) (*Cache, error) {
	if baseDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		baseDir = userCache
	}

	dir := filepath.Join(baseDir, cacheDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	c := &Cache{dir: dir}

	// Prune old entries in background
	go c.pruneOldEntries()

	return c, nil
}

// cacheKey generates a unique key for a track at specific dimensions.
func cacheKey(trackPath string, width, height int) string {
	data := fmt.Sprintf("%s:%d:%d", trackPath, width, height)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Get retrieves cached PNG data for a track at specific dimensions.
// Returns nil if not cached.
func (c *Cache) Get(trackPath string, width, height int) []byte {
	if c == nil {
		return nil
	}

	key := cacheKey(trackPath, width, height)
	path := filepath.Join(c.dir, key+".png")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Touch the file to update mtime (keeps frequently used entries fresh)
	now := time.Now()
	_ = os.Chtimes(path, now, now) //nolint:errcheck // best-effort

	return data
}

// Put stores PNG data for a track at specific dimensions.
func (c *Cache) Put(trackPath string, width, height int, data []byte) error {
	if c == nil {
		return nil
	}

	key := cacheKey(trackPath, width, height)
	path := filepath.Join(c.dir, key+".png")

	return os.WriteFile(path, data, 0o600)
}

// pruneOldEntries removes cache entries older than cacheMaxAge.
func (c *Cache) pruneOldEntries() {
	if c == nil {
		return
	}

	// Don't prune too frequently
	if time.Since(c.lastPruned) < pruneInterval {
		return
	}
	c.lastPruned = time.Now()

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-cacheMaxAge)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(c.dir, entry.Name())) //nolint:errcheck // best-effort cleanup
		}
	}
}

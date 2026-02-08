//go:build linux

package notify

import (
	"crypto/md5" //nolint:gosec // MD5 used for cache key, not security
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/llehouerou/waves/internal/mpris"
	"github.com/llehouerou/waves/internal/tags"
)

// FindAlbumArtPath returns the path to album art for a track, if found.
// It first looks for external cover files (cover.jpg, folder.jpg, etc.),
// then falls back to extracting embedded art from the audio file.
// Embedded art is cached to a temporary file for use by notification daemons.
func FindAlbumArtPath(trackPath string) string {
	// First try external cover files
	if path := mpris.FindAlbumArt(trackPath); path != "" {
		return path
	}

	// Try to extract embedded art
	data, mimeType, err := tags.ExtractCoverArt(trackPath)
	if err != nil || data == nil {
		return ""
	}

	// Save to temp file for notification daemon
	return saveArtToCache(trackPath, data, mimeType)
}

// saveArtToCache saves cover art data to a cache file and returns its path.
func saveArtToCache(trackPath string, data []byte, mimeType string) string {
	// Use hash of track path as cache key
	hash := fmt.Sprintf("%x", md5.Sum([]byte(trackPath))) //nolint:gosec // MD5 for cache key

	// Determine extension from MIME type
	ext := ".jpg"
	if strings.Contains(mimeType, "png") {
		ext = ".png"
	}

	// Create cache directory
	cacheDir := filepath.Join(os.TempDir(), "waves-albumart")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return ""
	}

	// Write to cache file
	cachePath := filepath.Join(cacheDir, hash+ext)

	// Skip if already cached
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath
	}

	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		return ""
	}

	return cachePath
}

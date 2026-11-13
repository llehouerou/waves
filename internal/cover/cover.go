// Package cover finds the cover of a track as a file: what MPRIS and a
// desktop notification are given, since both take a path rather than an image.
package cover

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/llehouerou/waves/internal/tags"
)

// coverNames lists common album art filenames in priority order.
var coverNames = []string{
	"cover.jpg", "cover.png", "cover.jpeg",
	"folder.jpg", "folder.png", "folder.jpeg",
	"album.jpg", "album.png", "album.jpeg",
	"front.jpg", "front.png", "front.jpeg",
	"artwork.jpg", "artwork.png", "artwork.jpeg",
}

// Find looks for album art for the given track.
// It first checks for common image files in the track's directory,
// whatever the case of their name, then falls back to extracting embedded
// cover art from the audio file.
// Returns the path to the art file, or empty string if not found.
//
// It reads the disk: never call it from a loop that must not wait.
func Find(trackPath string) string {
	dir := filepath.Dir(trackPath)
	// A folder that can't be read has no cover file to give either.
	entries, _ := os.ReadDir(dir) //nolint:errcheck // see above
	for _, name := range coverNames {
		for _, e := range entries {
			if strings.EqualFold(e.Name(), name) {
				return filepath.Join(dir, e.Name())
			}
		}
	}

	return extractEmbeddedToFile(trackPath)
}

// extractEmbeddedToFile extracts embedded cover art from an audio file
// and writes it to a cache file. Returns the cache file path, or empty
// string if no embedded art is found.
// Note: cached files are keyed by track path hash. If embedded art is
// re-tagged with a different format, the stale cache entry persists.
func extractEmbeddedToFile(trackPath string) string {
	data, mimeType, err := tags.ExtractEmbeddedArt(trackPath)
	if err != nil || data == nil {
		return ""
	}

	ext := ".jpg"
	if mimeType == "image/png" {
		ext = ".png"
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}

	dir := filepath.Join(cacheDir, "waves", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}

	hash := sha256.Sum256([]byte(trackPath))
	path := filepath.Join(dir, hex.EncodeToString(hash[:])+ext)

	// Skip write if already cached
	if _, err := os.Stat(path); err == nil {
		return path
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return ""
	}

	return path
}

//go:build linux || freebsd

package mpris

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/llehouerou/waves/internal/tags"
)

// coverNames lists common album art filenames in priority order.
var coverNames = []string{
	"cover.jpg", "cover.png", "cover.jpeg",
	"folder.jpg", "folder.png", "folder.jpeg",
	"album.jpg", "album.png", "album.jpeg",
	"front.jpg", "front.png", "front.jpeg",
}

// FindAlbumArt looks for album art for the given track.
// It first checks for common image files in the track's directory,
// then falls back to extracting embedded cover art from the audio file.
// Returns the path to the art file, or empty string if not found.
func FindAlbumArt(trackPath string) string {
	dir := filepath.Dir(trackPath)
	for _, name := range coverNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return extractEmbeddedToFile(trackPath)
}

// extractEmbeddedToFile extracts embedded cover art from an audio file
// and writes it to a cache file. Returns the cache file path, or empty
// string if no embedded art is found.
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

	dir := filepath.Join(cacheDir, "waves", "mpris-covers")
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

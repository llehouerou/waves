//go:build linux

package mpris

import (
	"os"
	"path/filepath"
)

// coverNames lists common album art filenames in priority order.
var coverNames = []string{
	"cover.jpg", "cover.png", "cover.jpeg",
	"folder.jpg", "folder.png", "folder.jpeg",
	"album.jpg", "album.png", "album.jpeg",
	"front.jpg", "front.png", "front.jpeg",
}

// FindAlbumArt looks for album art in the same directory as the track.
// Returns the path to the art file, or empty string if not found.
func FindAlbumArt(trackPath string) string {
	dir := filepath.Dir(trackPath)
	for _, name := range coverNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

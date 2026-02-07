//go:build linux

package notify

import "github.com/llehouerou/waves/internal/mpris"

// FindAlbumArtPath returns the path to album art for a track, if found.
// This is a convenience wrapper around mpris.FindAlbumArt.
func FindAlbumArtPath(trackPath string) string {
	return mpris.FindAlbumArt(trackPath)
}

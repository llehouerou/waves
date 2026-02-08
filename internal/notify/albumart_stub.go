//go:build !linux

package notify

// FindAlbumArtPath returns empty on non-Linux platforms.
// Desktop notifications are only supported on Linux via D-Bus.
func FindAlbumArtPath(_ string) string {
	return ""
}

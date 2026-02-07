//go:build linux

package mpris

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindAlbumArt(t *testing.T) {
	// Create temp directory with a cover file
	dir := t.TempDir()
	coverPath := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(coverPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	trackPath := filepath.Join(dir, "track.mp3")

	got := FindAlbumArt(trackPath)
	if got != coverPath {
		t.Errorf("FindAlbumArt() = %q, want %q", got, coverPath)
	}
}

func TestFindAlbumArt_NotFound(t *testing.T) {
	dir := t.TempDir()
	trackPath := filepath.Join(dir, "track.mp3")

	got := FindAlbumArt(trackPath)
	if got != "" {
		t.Errorf("FindAlbumArt() = %q, want empty string", got)
	}
}

func TestFindAlbumArt_Priority(t *testing.T) {
	dir := t.TempDir()

	// Create folder.jpg (lower priority)
	folderPath := filepath.Join(dir, "folder.jpg")
	if err := os.WriteFile(folderPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create cover.jpg (higher priority)
	coverPath := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(coverPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	trackPath := filepath.Join(dir, "track.mp3")

	got := FindAlbumArt(trackPath)
	if got != coverPath {
		t.Errorf("FindAlbumArt() = %q, want %q (higher priority)", got, coverPath)
	}
}

//go:build linux

package notify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindAlbumArtPath(t *testing.T) {
	dir := t.TempDir()

	// Create a fake track file
	trackPath := filepath.Join(dir, "01-song.mp3")
	if err := os.WriteFile(trackPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// No cover yet
	got := FindAlbumArtPath(trackPath)
	if got != "" {
		t.Errorf("FindAlbumArtPath() = %q, want empty", got)
	}

	// Create cover.jpg
	coverPath := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(coverPath, []byte{0xFF, 0xD8, 0xFF}, 0o600); err != nil {
		t.Fatal(err)
	}

	got = FindAlbumArtPath(trackPath)
	if got != coverPath {
		t.Errorf("FindAlbumArtPath() = %q, want %q", got, coverPath)
	}
}

func TestFindAlbumArtPathPriority(t *testing.T) {
	dir := t.TempDir()
	trackPath := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(trackPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Create folder.png first
	folderPath := filepath.Join(dir, "folder.png")
	if err := os.WriteFile(folderPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Create cover.jpg (higher priority)
	coverPath := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(coverPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	got := FindAlbumArtPath(trackPath)
	if got != coverPath {
		t.Errorf("FindAlbumArtPath() = %q, want %q (higher priority)", got, coverPath)
	}
}

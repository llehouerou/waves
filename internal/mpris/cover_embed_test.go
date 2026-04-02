//go:build linux

package mpris

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/tags"
)

func createTestMP3WithCover(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.mp3")

	// Create minimal MP3 frame (MPEG1 Layer3, 128kbps, 44100Hz, stereo)
	mp3Frame := make([]byte, 417)
	mp3Frame[0] = 0xff
	mp3Frame[1] = 0xfb
	mp3Frame[2] = 0x90
	mp3Frame[3] = 0x00

	if err := os.WriteFile(path, mp3Frame, 0o600); err != nil {
		t.Fatalf("create test MP3: %v", err)
	}

	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	if err := tags.Write(path, &tags.Tag{
		Title:    "Test",
		Artist:   "Artist",
		CoverArt: jpegData,
	}); err != nil {
		t.Fatalf("write tags: %v", err)
	}

	return path
}

// useTempCacheDir sets XDG_CACHE_HOME to a temp directory so tests
// work in sandboxed environments (e.g., nix build).
func useTempCacheDir(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

func TestFindAlbumArt_EmbeddedFallback(t *testing.T) {
	useTempCacheDir(t)
	dir := t.TempDir()
	trackPath := createTestMP3WithCover(t, dir)

	// No folder art exists, should fall back to embedded extraction
	got := FindAlbumArt(trackPath)
	if got == "" {
		t.Fatal("FindAlbumArt() returned empty, want cached embedded art path")
	}
	if !strings.Contains(got, "mpris-covers") {
		t.Errorf("expected path in mpris-covers cache dir, got %q", got)
	}
	if !strings.HasSuffix(got, ".jpg") {
		t.Errorf("expected .jpg extension, got %q", got)
	}

	// Verify the file was written
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read cached art: %v", err)
	}
	if len(data) == 0 {
		t.Error("cached art file is empty")
	}
}

func TestFindAlbumArt_FolderArtPreferredOverEmbedded(t *testing.T) {
	dir := t.TempDir()
	trackPath := createTestMP3WithCover(t, dir)

	// Create folder art — should be preferred over embedded
	coverPath := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(coverPath, []byte("folder art"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := FindAlbumArt(trackPath)
	if got != coverPath {
		t.Errorf("FindAlbumArt() = %q, want folder art %q", got, coverPath)
	}
}

func TestExtractEmbeddedToFile_NoArt(t *testing.T) {
	useTempCacheDir(t)
	dir := t.TempDir()

	// MP3 without cover art
	path := filepath.Join(dir, "bare.mp3")
	mp3Frame := make([]byte, 417)
	mp3Frame[0] = 0xff
	mp3Frame[1] = 0xfb
	mp3Frame[2] = 0x90
	mp3Frame[3] = 0x00
	if err := os.WriteFile(path, mp3Frame, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tags.Write(path, &tags.Tag{Title: "Bare"}); err != nil {
		t.Fatal(err)
	}

	got := extractEmbeddedToFile(path)
	if got != "" {
		t.Errorf("extractEmbeddedToFile() = %q, want empty for track without art", got)
	}
}

func TestExtractEmbeddedToFile_Caching(t *testing.T) {
	useTempCacheDir(t)
	dir := t.TempDir()
	trackPath := createTestMP3WithCover(t, dir)

	// First call extracts and caches
	path1 := extractEmbeddedToFile(trackPath)
	if path1 == "" {
		t.Fatal("first extraction returned empty")
	}

	// Second call should return same path (cache hit)
	path2 := extractEmbeddedToFile(trackPath)
	if path2 != path1 {
		t.Errorf("cache miss: got %q, want %q", path2, path1)
	}
}

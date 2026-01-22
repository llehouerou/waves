package tags

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractCoverArt_EmbeddedMP3 tests extracting embedded cover art from MP3.
func TestExtractCoverArt_EmbeddedMP3(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal JPEG (just header bytes for testing)
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}

	tags := &Tag{
		Title:    "Test",
		Artist:   "Test Artist",
		CoverArt: jpegData,
	}
	path := createTestMP3(t, dir, tags)

	data, mimeType, err := ExtractCoverArt(path)
	if err != nil {
		t.Fatalf("ExtractCoverArt() error: %v", err)
	}

	if data == nil {
		t.Error("expected cover art data, got nil")
	}
	if mimeType != mimeJPEG {
		t.Errorf("mimeType = %q, want %q", mimeType, mimeJPEG)
	}
}

// TestExtractCoverArt_FolderArt tests finding cover art in the album folder.
func TestExtractCoverArt_FolderArt(t *testing.T) {
	dir := t.TempDir()

	// Create MP3 without embedded art
	path := createTestMP3(t, dir, &Tag{Title: "Test"})

	// Create cover.jpg in the same folder
	coverPath := filepath.Join(dir, "cover.jpg")
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	if err := os.WriteFile(coverPath, jpegData, 0o600); err != nil {
		t.Fatalf("create cover.jpg: %v", err)
	}

	data, mimeType, err := ExtractCoverArt(path)
	if err != nil {
		t.Fatalf("ExtractCoverArt() error: %v", err)
	}

	if data == nil {
		t.Error("expected cover art data from folder, got nil")
	}
	if mimeType != mimeJPEG {
		t.Errorf("mimeType = %q, want %q", mimeType, mimeJPEG)
	}
}

// TestExtractCoverArt_FolderArtPNG tests finding PNG cover art.
func TestExtractCoverArt_FolderArtPNG(t *testing.T) {
	dir := t.TempDir()

	// Create MP3 without embedded art
	path := createTestMP3(t, dir, &Tag{Title: "Test"})

	// Create album.png in the same folder
	coverPath := filepath.Join(dir, "album.png")
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if err := os.WriteFile(coverPath, pngData, 0o600); err != nil {
		t.Fatalf("create album.png: %v", err)
	}

	data, mimeType, err := ExtractCoverArt(path)
	if err != nil {
		t.Fatalf("ExtractCoverArt() error: %v", err)
	}

	if data == nil {
		t.Error("expected cover art data from folder, got nil")
	}
	if mimeType != mimePNG {
		t.Errorf("mimeType = %q, want %q", mimeType, mimePNG)
	}
}

// TestExtractCoverArt_FolderArtPriority tests that common filenames are checked in order.
func TestExtractCoverArt_FolderArtPriority(t *testing.T) {
	dir := t.TempDir()

	// Create MP3 without embedded art
	path := createTestMP3(t, dir, &Tag{Title: "Test"})

	// Create both cover.jpg and folder.jpg
	// cover.jpg should be found first (it's in coverArtFilenames before folder.jpg)
	coverPath := filepath.Join(dir, "cover.jpg")
	coverData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'c', 'o', 'v', 'e', 'r'}
	if err := os.WriteFile(coverPath, coverData, 0o600); err != nil {
		t.Fatalf("create cover.jpg: %v", err)
	}

	folderPath := filepath.Join(dir, "folder.jpg")
	folderData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'f', 'o', 'l', 'd', 'r'}
	if err := os.WriteFile(folderPath, folderData, 0o600); err != nil {
		t.Fatalf("create folder.jpg: %v", err)
	}

	data, _, err := ExtractCoverArt(path)
	if err != nil {
		t.Fatalf("ExtractCoverArt() error: %v", err)
	}

	// Should get cover.jpg content (first in priority)
	if string(data[4:]) != "cover" {
		t.Errorf("expected cover.jpg data, got folder.jpg data")
	}
}

// TestExtractCoverArt_NoCoverArt tests when no cover art is available.
func TestExtractCoverArt_NoCoverArt(t *testing.T) {
	dir := t.TempDir()

	// Create MP3 without embedded art and no folder art
	path := createTestMP3(t, dir, &Tag{Title: "Test"})

	data, mimeType, err := ExtractCoverArt(path)
	if err != nil {
		t.Fatalf("ExtractCoverArt() error: %v", err)
	}

	// Should return nil data without error
	if data != nil {
		t.Errorf("expected nil data, got %d bytes", len(data))
	}
	if mimeType != "" {
		t.Errorf("expected empty mimeType, got %q", mimeType)
	}
}

// TestExtractCoverArt_NonexistentFile tests error handling for missing files.
func TestExtractCoverArt_NonexistentFile(t *testing.T) {
	_, _, err := ExtractCoverArt("/nonexistent/file.mp3")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestExtractCoverArt_UppercaseFolderArt tests case-insensitive folder art search.
func TestExtractCoverArt_UppercaseFolderArt(t *testing.T) {
	dir := t.TempDir()

	// Create MP3 without embedded art
	path := createTestMP3(t, dir, &Tag{Title: "Test"})

	// Create COVER.JPG (uppercase)
	coverPath := filepath.Join(dir, "COVER.JPG")
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	if err := os.WriteFile(coverPath, jpegData, 0o600); err != nil {
		t.Fatalf("create COVER.JPG: %v", err)
	}

	data, _, err := ExtractCoverArt(path)
	if err != nil {
		t.Fatalf("ExtractCoverArt() error: %v", err)
	}

	if data == nil {
		t.Error("expected cover art data from COVER.JPG, got nil")
	}
}

// TestFindFolderArt tests the findFolderArt function directly.
func TestFindFolderArt(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantMime string
	}{
		{"cover.jpg", "cover.jpg", mimeJPEG},
		{"cover.jpeg", "cover.jpeg", mimeJPEG},
		{"cover.png", "cover.png", mimePNG},
		{"folder.jpg", "folder.jpg", mimeJPEG},
		{"album.png", "album.png", mimePNG},
		{"front.jpg", "front.jpg", mimeJPEG},
		{"artwork.jpeg", "artwork.jpeg", mimeJPEG},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Create the cover file
			coverPath := filepath.Join(dir, tt.filename)
			testData := []byte("test image data")
			if err := os.WriteFile(coverPath, testData, 0o600); err != nil {
				t.Fatalf("create %s: %v", tt.filename, err)
			}

			data, mimeType, err := findFolderArt(dir)
			if err != nil {
				t.Fatalf("findFolderArt() error: %v", err)
			}

			if data == nil {
				t.Error("expected data, got nil")
			}
			if mimeType != tt.wantMime {
				t.Errorf("mimeType = %q, want %q", mimeType, tt.wantMime)
			}
		})
	}
}

// TestFindFolderArt_EmptyDir tests findFolderArt with no cover files.
func TestFindFolderArt_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	data, mimeType, err := findFolderArt(dir)
	if err != nil {
		t.Fatalf("findFolderArt() error: %v", err)
	}

	if data != nil {
		t.Error("expected nil data for empty dir")
	}
	if mimeType != "" {
		t.Error("expected empty mimeType for empty dir")
	}
}

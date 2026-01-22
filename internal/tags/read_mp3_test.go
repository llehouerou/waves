package tags

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
)

// createMinimalMP3 creates a minimal valid MP3 file for testing.
// Returns MP3 frame header + padding (417 bytes total for 128kbps frame).
func createMinimalMP3(t *testing.T, path string) {
	t.Helper()
	// MP3 frame header (MPEG1 Layer3, 128kbps, 44100Hz, stereo) + padding
	mp3Frame := make([]byte, 417)
	mp3Frame[0] = 0xff
	mp3Frame[1] = 0xfb
	mp3Frame[2] = 0x90
	mp3Frame[3] = 0x00

	if err := os.WriteFile(path, mp3Frame, 0o600); err != nil {
		t.Fatalf("failed to create test MP3: %v", err)
	}
}

func TestParseTrackNumber(t *testing.T) {
	tests := []struct {
		input     string
		wantNum   int
		wantTotal int
	}{
		{"", 0, 0},
		{"5", 5, 0},
		{"5/10", 5, 10},
		{"1/1", 1, 1},
		{"12/24", 12, 24},
		{"invalid", 0, 0},
		{"5/invalid", 5, 0},
		{"invalid/10", 0, 10},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			num, total := parseTrackNumber(tt.input)
			if num != tt.wantNum {
				t.Errorf("parseTrackNumber(%q) num = %d, want %d", tt.input, num, tt.wantNum)
			}
			if total != tt.wantTotal {
				t.Errorf("parseTrackNumber(%q) total = %d, want %d", tt.input, total, tt.wantTotal)
			}
		})
	}
}

func TestReadMP3WithID3v2Fallback(t *testing.T) {
	// Create a temporary MP3 file with ID3v2 tags
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")
	createMinimalMP3(t, mp3Path)

	// Add ID3v2 tags using the bogem/id3v2 library
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("failed to open MP3 for tagging: %v", err)
	}

	tag.SetTitle("Test Title")
	tag.SetArtist("Test Artist")
	tag.SetAlbum("Test Album")
	tag.SetYear("2024")
	tag.SetGenre("Rock")
	tag.AddTextFrame("TRCK", id3v2.EncodingUTF8, "3/12")
	tag.AddTextFrame("TPOS", id3v2.EncodingUTF8, "1/2")
	tag.AddTextFrame("TPE2", id3v2.EncodingUTF8, "Test Album Artist")

	if err := tag.Save(); err != nil {
		t.Fatalf("failed to save ID3 tags: %v", err)
	}
	tag.Close()

	// Test the fallback function
	info, err := readMP3WithID3v2Fallback(mp3Path)
	if err != nil {
		t.Fatalf("readMP3WithID3v2Fallback failed: %v", err)
	}

	// Verify metadata
	if info.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", info.Title, "Test Title")
	}
	if info.Artist != "Test Artist" {
		t.Errorf("Artist = %q, want %q", info.Artist, "Test Artist")
	}
	if info.Album != "Test Album" {
		t.Errorf("Album = %q, want %q", info.Album, "Test Album")
	}
	if info.AlbumArtist != "Test Album Artist" {
		t.Errorf("AlbumArtist = %q, want %q", info.AlbumArtist, "Test Album Artist")
	}
	if info.Year() != 2024 {
		t.Errorf("Year() = %d, want %d", info.Year(), 2024)
	}
	if info.Genre != "Rock" {
		t.Errorf("Genre = %q, want %q", info.Genre, "Rock")
	}
	if info.TrackNumber != 3 {
		t.Errorf("TrackNumber = %d, want %d", info.TrackNumber, 3)
	}
	if info.TotalTracks != 12 {
		t.Errorf("TotalTracks = %d, want %d", info.TotalTracks, 12)
	}
	if info.DiscNumber != 1 {
		t.Errorf("DiscNumber = %d, want %d", info.DiscNumber, 1)
	}
	if info.TotalDiscs != 2 {
		t.Errorf("TotalDiscs = %d, want %d", info.TotalDiscs, 2)
	}
}

func TestReadMP3WithID3v2Fallback_AlbumArtistFallsBackToArtist(t *testing.T) {
	// Create a temporary MP3 file without TPE2 (album artist) frame
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")
	createMinimalMP3(t, mp3Path)

	// Add tags without album artist
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("failed to open MP3 for tagging: %v", err)
	}
	tag.SetArtist("Solo Artist")
	tag.SetAlbum("Album")
	if err := tag.Save(); err != nil {
		t.Fatalf("failed to save ID3 tags: %v", err)
	}
	tag.Close()

	// Test that album artist falls back to artist
	info, err := readMP3WithID3v2Fallback(mp3Path)
	if err != nil {
		t.Fatalf("readMP3WithID3v2Fallback failed: %v", err)
	}

	if info.AlbumArtist != "Solo Artist" {
		t.Errorf("AlbumArtist = %q, want %q (should fall back to Artist)", info.AlbumArtist, "Solo Artist")
	}
}

func TestReadMP3WithID3v2Fallback_TitleFallsBackToFilename(t *testing.T) {
	// Create a temporary MP3 file without title
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "my-song.mp3")
	createMinimalMP3(t, mp3Path)

	// Add tags without title
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("failed to open MP3 for tagging: %v", err)
	}
	tag.SetArtist("Artist")
	tag.SetAlbum("Album")
	if err := tag.Save(); err != nil {
		t.Fatalf("failed to save ID3 tags: %v", err)
	}
	tag.Close()

	// Test that title falls back to filename
	info, err := readMP3WithID3v2Fallback(mp3Path)
	if err != nil {
		t.Fatalf("readMP3WithID3v2Fallback failed: %v", err)
	}

	if info.Title != "my-song.mp3" {
		t.Errorf("Title = %q, want %q (should fall back to filename)", info.Title, "my-song.mp3")
	}
}

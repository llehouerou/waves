package player

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"

	"github.com/llehouerou/waves/internal/tags"
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

func TestIsMusicFile_Opus(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"song.opus", true},
		{"song.OPUS", true},
		{"song.ogg", true},
		{"song.OGG", true},
		{"song.mp3", true},
		{"song.flac", true},
		{"song.wav", false},
		{"song.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := tags.IsMusicFile(tt.path); got != tt.want {
				t.Errorf("IsMusicFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestReadTrackInfo_FallbackOnMalformedUTF16(t *testing.T) {
	// Create a temporary MP3 file with UTF-16 encoded tags that trigger
	// the dhowden/tag UTF-16 parsing bug
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")
	createMinimalMP3(t, mp3Path)

	// Add tags with UTF-16 encoding which triggers the bug in dhowden/tag
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("failed to open MP3 for tagging: %v", err)
	}

	tag.AddTextFrame("TIT2", id3v2.EncodingUTF16, "Test Title UTF16")
	tag.AddTextFrame("TPE1", id3v2.EncodingUTF16, "Test Artist UTF16")
	tag.AddTextFrame("TALB", id3v2.EncodingUTF16, "Test Album UTF16")
	tag.AddTextFrame("TYER", id3v2.EncodingUTF16, "2024")
	tag.AddTextFrame("TCON", id3v2.EncodingUTF16, "Rock")

	if err := tag.Save(); err != nil {
		t.Fatalf("failed to save ID3 tags: %v", err)
	}
	tag.Close()

	// Test tags.Read - should succeed via fallback when dhowden/tag fails
	info, err := tags.Read(mp3Path)
	if err != nil {
		t.Fatalf("tags.Read failed: %v", err)
	}

	// Verify we got the metadata via the fallback path
	if info.Artist == "" {
		t.Error("Artist should not be empty")
	}
	if info.Album == "" {
		t.Error("Album should not be empty")
	}
	if info.Title == "" {
		t.Error("Title should not be empty")
	}
}

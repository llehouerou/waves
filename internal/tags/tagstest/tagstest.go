// Package tagstest makes audio files for the tests of the packages that read,
// tag and move them.
package tagstest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/llehouerou/waves/internal/tags"
)

// WriteMP3 creates at path, and its folder with it, the smallest MP3 the tags
// package reads and writes: one silent frame, tagged with tag. It needs no
// ffmpeg, so a test using it never skips.
func WriteMP3(t *testing.T, path string, tag tags.Tag) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	// MPEG1 Layer 3, 128 kbps, 44100 Hz, stereo.
	frame := make([]byte, 417)
	frame[0], frame[1], frame[2] = 0xff, 0xfb, 0x90
	if err := os.WriteFile(path, frame, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := tags.Write(path, &tag); err != nil {
		t.Fatalf("tag %s: %v", path, err)
	}
}

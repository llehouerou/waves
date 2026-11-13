//go:build linux

package cover

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates an image file of dir, and returns its path.
func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	coverPath := writeFile(t, dir, "cover.jpg")

	if got := Find(filepath.Join(dir, "track.mp3")); got != coverPath {
		t.Errorf("Find() = %q, want %q", got, coverPath)
	}
}

func TestFind_NotFound(t *testing.T) {
	if got := Find(filepath.Join(t.TempDir(), "track.mp3")); got != "" {
		t.Errorf("Find() = %q, want empty string", got)
	}
}

func TestFind_Priority(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "folder.jpg")
	coverPath := writeFile(t, dir, "cover.jpg")

	if got := Find(filepath.Join(dir, "track.mp3")); got != coverPath {
		t.Errorf("Find() = %q, want %q (higher priority)", got, coverPath)
	}
}

// Rippers and taggers do not agree on the name of the file, nor on its case.
func TestFind_TakesTheCoverOfTheFolderWhateverItsCase(t *testing.T) {
	for _, name := range []string{"Cover.jpg", "FOLDER.JPG", "artwork.png", "Front.jpeg"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			coverPath := writeFile(t, dir, name)

			if got := Find(filepath.Join(dir, "track.mp3")); got != coverPath {
				t.Errorf("Find() = %q, want %q", got, coverPath)
			}
		})
	}
}

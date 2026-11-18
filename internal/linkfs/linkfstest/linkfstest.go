// Package linkfstest builds folders holding symbolic links, for the tests of
// the code that lists or walks folders.
package linkfstest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// writeTrack writes at path a tagged MP3 titled title.
func writeTrack(t *testing.T, path, title string) {
	t.Helper()
	tagstest.WriteMP3(t, path, tags.Tag{Artist: "Low", AlbumArtist: "Low", Album: "Trust", Title: title})
}

// Link makes path a symbolic link to target and returns path.
func Link(t *testing.T, target, path string) string {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return path
}

// Links returns a new folder holding:
//
//	real/a/one.mp3  a tagged MP3 titled "One"
//	link            a symbolic link to real
//	linked.mp3      a symbolic link to real/a/one.mp3
//	dangling        a symbolic link to nothing
//	dangling.mp3    a symbolic link to nothing
//	notes.txt       a file
//	linked.txt      a symbolic link to notes.txt
func Links(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	one := filepath.Join(dir, "real", "a", "one.mp3")
	writeTrack(t, one, "One")
	Link(t, filepath.Join(dir, "real"), filepath.Join(dir, "link"))
	Link(t, one, filepath.Join(dir, "linked.mp3"))
	Link(t, filepath.Join(dir, "missing"), filepath.Join(dir, "dangling"))
	Link(t, filepath.Join(dir, "missing.mp3"), filepath.Join(dir, "dangling.mp3"))
	notes := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notes, []byte("notes"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	Link(t, notes, filepath.Join(dir, "linked.txt"))
	return dir
}

// Loops returns a new folder, top, holding:
//
//	a/one.mp3  a tagged MP3 titled "One"
//	inner      a symbolic link to a folder outside top holding two.mp3
//	loop       a symbolic link to top itself
func Loops(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	top := filepath.Join(root, "top")
	writeTrack(t, filepath.Join(top, "a", "one.mp3"), "One")
	outside := filepath.Join(root, "outside")
	writeTrack(t, filepath.Join(outside, "two.mp3"), "Two")
	Link(t, outside, filepath.Join(top, "inner"))
	Link(t, top, filepath.Join(top, "loop"))
	return top
}

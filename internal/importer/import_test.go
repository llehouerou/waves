package importer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
)

// A track already in the library is never overwritten: re-importing an album,
// or a wrong match, would silently replace a good file. The source is left
// untouched too, tags included.
func TestImport_RefusesAnExistingDestination(t *testing.T) {
	src := filepath.Join(t.TempDir(), "01.flac")
	if err := os.WriteFile(src, []byte("downloaded"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dest := filepath.Join(root, "Album", "Title.flac")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("in the library"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Import(ImportParams{
		SourcePath:   src,
		DestRoot:     root,
		ReleaseGroup: &musicbrainz.ReleaseGroup{PrimaryType: "Album"},
		Release: &musicbrainz.ReleaseDetails{
			Release: musicbrainz.Release{Title: "Album", Artist: "Artist"},
			Tracks:  []musicbrainz.Track{{Position: 1, Title: "Title"}},
		},
		RenameConfig: rename.Config{Folder: "{album}", Filename: "{title}"},
	})

	if !errors.Is(err, ErrAlreadyInLibrary) {
		t.Errorf("Import = %v, want ErrAlreadyInLibrary", err)
	}
	for path, want := range map[string]string{dest: "in the library", src: "downloaded"} {
		if got, _ := os.ReadFile(path); string(got) != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
}

// A copy that fails half-way must not leave a partial file in the library:
// since Import refuses existing destinations, it would block every re-import.
func TestCopyFile_FailureLeavesNothing(t *testing.T) {
	src := t.TempDir() // a directory: opens, but can't be read as a file
	dst := filepath.Join(t.TempDir(), "Title.flac")

	if err := copyFile(src, dst); err == nil {
		t.Fatal("copying a directory succeeded")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("partial destination left behind: %v", err)
	}
}

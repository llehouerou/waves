package importer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
)

// ownArtistParams imports srcPath as a track with its own artist, as on a
// compilation or a collaboration.
func ownArtistParams(srcPath, root string) ImportParams {
	return ImportParams{
		SourcePath:   srcPath,
		DestRoot:     root,
		ReleaseGroup: &musicbrainz.ReleaseGroup{PrimaryType: "Album"},
		Release: &musicbrainz.ReleaseDetails{
			Release: musicbrainz.Release{Title: "Album", Artist: "Artist"},
			Tracks:  []musicbrainz.Track{{Position: 1, Title: "Title", Artist: "Guest"}},
		},
		DiscNumber:   1,
		TotalDiscs:   1,
		RenameConfig: rename.Config{Folder: "{albumartist}/{album}", Filename: "{artist} - {title}"},
	}
}

func TestDestPath_TrackArtistAndReleaseArtist(t *testing.T) {
	got := DestPath(ownArtistParams("/downloads/01.FLAC", "/music"))

	want := "/music/Artist/Album/Guest - Title.flac"
	if got != want {
		t.Errorf("DestPath = %q, want %q", got, want)
	}
}

// What the import popup previews with DestPath is where Import puts the file.
func TestImport_WritesToDestPath(t *testing.T) {
	src := filepath.Join(t.TempDir(), "01.flac")
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "flac", src)
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
	p := ownArtistParams(src, t.TempDir())

	result, err := Import(p)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if want := DestPath(p); result.DestPath != want {
		t.Errorf("Import wrote to %q, DestPath = %q", result.DestPath, want)
	}
	if _, err := os.Stat(result.DestPath); err != nil {
		t.Errorf("imported file: %v", err)
	}
}

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

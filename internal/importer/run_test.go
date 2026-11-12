package importer

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// planned is the plan of downloaded's three files, with cover as the cover of
// release A.
func planned(t *testing.T, completed string, cover []byte) Plan {
	t.Helper()
	mb := &fakeMB{covers: map[string][]byte{"A": cover}}
	plan, err := Prepare(downloaded(t, completed, tags.Tag{}), completed, mb, rename.DefaultConfig())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return plan
}

func states(files []FileProgress) []FileState {
	out := make([]FileState, len(files))
	for i, f := range files {
		out[i] = f.State
	}
	return out
}

func TestRun_TagsAndMovesEachFileToWhereThePlanSaid(t *testing.T) {
	completed, library := t.TempDir(), t.TempDir()
	plan := planned(t, completed, nil)
	var reports [][]FileState

	done := Run(plan, library, rename.DefaultConfig(), func(files []FileProgress) {
		reports = append(reports, states(files))
	}, nil)

	if want := []FileState{FileDone, FileDone, FileDone}; !slices.Equal(states(done), want) {
		t.Fatalf("files ended %v (%+v), want all done", states(done), done)
	}
	for i, title := range []string{"One", "Two", "Three"} {
		dest := filepath.Join(library, plan.Files[i].Dest)
		if done[i].Path != plan.Files[i].Source || done[i].Dest != dest {
			t.Errorf("file %d went from %s to %s, want from %s to %s", i, done[i].Path, done[i].Dest, plan.Files[i].Source, dest)
		}
		tag, err := tags.Read(dest)
		if err != nil {
			t.Fatalf("file %d is not at its destination: %v", i, err)
		}
		if tag.Title != title || tag.Album != "Album A" || tag.MBReleaseID != "A" || tag.TrackNumber != i+1 {
			t.Errorf("file %d tagged %q on %q (release %q, track %d), want %q on Album A (release A, track %d)",
				i, tag.Title, tag.Album, tag.MBReleaseID, tag.TrackNumber, title, i+1)
		}
		if _, err := os.Stat(plan.Files[i].Source); !os.IsNotExist(err) {
			t.Errorf("file %d is still in the download folder: %v", i, err)
		}
	}

	// One report before each file: it is being worked on, those before it are settled.
	want := [][]FileState{
		{FileWorking, FilePending, FilePending},
		{FileDone, FileWorking, FilePending},
		{FileDone, FileDone, FileWorking},
	}
	if !slices.EqualFunc(reports, want, slices.Equal[[]FileState]) {
		t.Errorf("reports = %v, want %v", reports, want)
	}
}

// A download with more files than the release has tracks used to map every
// extra file onto the last track, and the move overwrote it. An extra file
// has no destination: the plan says so, and the import fails it untouched.
func TestRun_AFileWithNoTrackIsLeftWhereItIs(t *testing.T) {
	completed, library := t.TempDir(), t.TempDir()
	d := downloaded(t, completed, tags.Tag{})
	d.Files = append(d.Files, files(`@@bob\Album\04 bonus.mp3`)...)
	bonus := downloads.ExpectedDiskPath(completed, d.Files[3].Filename)
	tagstest.WriteMP3(t, bonus, tags.Tag{})
	plan, err := Prepare(d, completed, &fakeMB{}, rename.DefaultConfig())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if extra := plan.Files[3]; extra.Track != -1 || extra.Dest != "" {
		t.Fatalf("the extra file is planned as track %d to %q, want no track and no destination", extra.Track, extra.Dest)
	}

	done := Run(plan, library, rename.DefaultConfig(), nil, nil)

	if want := []FileState{FileDone, FileDone, FileDone, FileFailed}; !slices.Equal(states(done), want) {
		t.Fatalf("files ended %v, want the three tracks done and the extra file failed", states(done))
	}
	if err := done[3].Err; err == nil || !strings.Contains(err.Error(), "no MusicBrainz track") {
		t.Errorf("the extra file failed with %v, want it to say it has no MusicBrainz track", err)
	}
	if _, err := os.Stat(bonus); err != nil {
		t.Errorf("the extra file left the download folder: %v", err)
	}
}

// Waves only downloads audio, so an image in the (possibly shared) download
// folder is never this download's: the album gets the cover fetched from the
// Cover Art Archive, and the download folder is left alone.
func TestRun_TheAlbumGetsTheFetchedCover(t *testing.T) {
	completed, library := t.TempDir(), t.TempDir()
	fetched := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 16)...) // a JPEG header
	plan := planned(t, completed, fetched)
	foreign := filepath.Join(completed, "Album", "cover.jpg")
	if err := os.WriteFile(foreign, []byte("another download's cover"), 0o600); err != nil {
		t.Fatal(err)
	}

	done := Run(plan, library, rename.DefaultConfig(), nil, nil)

	cover := filepath.Join(filepath.Dir(done[0].Dest), "cover.jpg")
	if got, err := os.ReadFile(cover); err != nil || !bytes.Equal(got, fetched) {
		t.Errorf("album cover = %q, %v; want the fetched cover", got, err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("the download folder's image was taken: %v", err)
	}
}

// partlyImported runs the plan of downloaded's three files with the third
// one missing, as an import that failed on it leaves a download, then puts
// the third file back: the plan is ready to be run again.
func partlyImported(t *testing.T, completed, library string) Plan {
	t.Helper()
	plan := planned(t, completed, nil)
	third := plan.Files[2].Source
	if err := os.Rename(third, third+".away"); err != nil {
		t.Fatal(err)
	}
	first := Run(plan, library, rename.DefaultConfig(), nil, nil)
	if want := []FileState{FileDone, FileDone, FileFailed}; !slices.Equal(states(first), want) {
		t.Fatalf("first import ended %v, want the third file failed", states(first))
	}
	if err := os.Rename(third+".away", third); err != nil {
		t.Fatal(err)
	}
	return plan
}

// The files a first import moved are gone from the download folder, so
// importing again failed on each of them and the download could never be
// finished. A file whose track is in the library counts as already imported:
// the second import brings the rest and is a full success.
func TestRun_AgainFinishesAPartlyImportedDownload(t *testing.T) {
	completed, library := t.TempDir(), t.TempDir()
	plan := partlyImported(t, completed, library)

	done := Run(plan, library, rename.DefaultConfig(), nil, nil)

	if want := []FileState{FileAlreadyImported, FileAlreadyImported, FileDone}; !slices.Equal(states(done), want) {
		t.Fatalf("second import ended %v (%+v), want two files already imported and the third done", states(done), done)
	}
	for i := range done {
		if _, err := os.Stat(done[i].Dest); err != nil {
			t.Errorf("file %d is not in the library: %v", i, err)
		}
	}
}

// A file gone from the download folder without being in the library was not
// imported: it still fails.
func TestRun_AgainFailsAFileGoneEverywhere(t *testing.T) {
	completed, library := t.TempDir(), t.TempDir()
	plan := partlyImported(t, completed, library)
	if err := os.Remove(filepath.Join(library, plan.Files[1].Dest)); err != nil {
		t.Fatal(err)
	}

	done := Run(plan, library, rename.DefaultConfig(), nil, nil)

	if want := []FileState{FileAlreadyImported, FileFailed, FileDone}; !slices.Equal(states(done), want) {
		t.Errorf("second import ended %v, want the second file failed", states(done))
	}
}

func TestRun_StopsBeforeTheNextFileWhenAsked(t *testing.T) {
	completed, library := t.TempDir(), t.TempDir()
	plan := planned(t, completed, nil)
	asked := 0

	done := Run(plan, library, rename.DefaultConfig(), nil, func() bool {
		asked++
		return asked > 1 // after the first file
	})

	if want := []FileState{FileDone, FilePending, FilePending}; !slices.Equal(states(done), want) {
		t.Fatalf("files ended %v, want the run stopped after the first", states(done))
	}
	if _, err := os.Stat(plan.Files[1].Source); err != nil {
		t.Errorf("a file not reached left the download folder: %v", err)
	}
}

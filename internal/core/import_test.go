package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// fakeCatalogue is the MusicBrainz of these tests: it knows the covers it is
// given and no release. Any other request panics, on the nil API it embeds.
type fakeCatalogue struct {
	musicbrainzAPI
	covers map[string][]byte
}

func (f fakeCatalogue) GetRelease(string) (*MBReleaseDetails, error) {
	return nil, errors.New("unavailable")
}

func (f fakeCatalogue) GetCoverArt(id string) ([]byte, error) {
	return f.covers[id], nil
}

// lowTrust is the release bob's "Album" folder is downloaded as.
func lowTrust() *MBReleaseDetails {
	return &MBReleaseDetails{
		Release: MBRelease{ID: "release", Title: "Trust", Artist: "Low", ArtistID: "low", Status: "Official", Date: "2002"},
		Tracks:  []MBTrack{{Position: 1, Title: "Amazing Grace"}, {Position: 2, Title: "Canada"}, {Position: 3, Title: "Candy Girl"}},
	}
}

// newImportCore builds a core with one library source, whose fake slskd has
// finished downloading the three tracks of lowTrust: the files are in the
// completed folder, untagged. It returns the download's ID.
func newImportCore(t *testing.T, opts Options) (c *Core, id int64, completed, source string) {
	t.Helper()
	return newImportCoreOf(t, opts, lowTrust())
}

// newImportCoreOf is newImportCore for the tracks of release, downloaded as
// 01.mp3, 02.mp3 and so on.
func newImportCoreOf(t *testing.T, opts Options, release *MBReleaseDetails) (c *Core, id int64, completed, source string) {
	t.Helper()
	source = t.TempDir()
	opts.LibrarySources = []string{source}
	c, _, completed = newDownloadsCore(t, opts)

	names := make([]string, len(release.Tracks))
	for i := range names {
		names[i] = fmt.Sprintf("%02d.mp3", i+1)
	}
	d := album(names...)
	d.MBReleaseDetails = release
	d.MBReleaseGroup = &MBReleaseGroup{ID: "rg", PrimaryType: "Album", FirstRelease: "2002"}
	id, err := c.Downloads().Queue(d)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	for _, f := range d.Files {
		tagstest.WriteMP3(t, ExpectedDiskPath(completed, f.Filename), FileTags{})
	}
	return c, id, completed, source
}

func TestImport_RunsToItsEndWithNoClientDrivingIt(t *testing.T) {
	c, id, completed, source := newImportCore(t, Options{})
	dl := c.Downloads()

	plan, err := dl.PrepareImport(id)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := dl.Import(plan, source); err != nil {
		t.Fatalf("import: %v", err)
	}

	// Not an event is read, nothing more is asked: the core imports on its own.
	eventually(t, "the import is finished: the download is gone", func() bool {
		all, err := dl.List()
		return err == nil && len(all) == 0
	})

	// Tags written and tracks in the library: it reads them from the files.
	tracks, err := c.Library().Tracks("Low", "Trust")
	if err != nil || len(tracks) != 3 {
		t.Fatalf("library tracks = %+v (err %v), want the three tracks of Trust", tracks, err)
	}
	for i, title := range []string{"Amazing Grace", "Canada", "Candy Girl"} {
		if tracks[i].Title != title {
			t.Errorf("track %d = %q, want %q", i+1, tracks[i].Title, title)
		}
		// Files moved into the library source, where the plan said.
		if want := filepath.Join(source, plan.Files[i].Dest); tracks[i].Path != want {
			t.Errorf("track %d is at %s, want %s", i+1, tracks[i].Path, want)
		}
		if _, err := os.Stat(tracks[i].Path); err != nil {
			t.Errorf("track %d: %v", i+1, err)
		}
	}
	// Finish import: the download folder, now empty, is gone with the download.
	if _, err := os.Stat(filepath.Join(completed, "Album")); !os.IsNotExist(err) {
		t.Errorf("the download folder is still there: %v", err)
	}
}

// startImport prepares and starts the import of download id.
func startImport(t *testing.T, c *Core, id int64, source string) ImportPlan {
	t.Helper()
	plan, err := c.Downloads().PrepareImport(id)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := c.Downloads().Import(plan, source); err != nil {
		t.Fatalf("import: %v", err)
	}
	return plan
}

func fileStates(files []FileProgress) []FileState {
	out := make([]FileState, len(files))
	for i := range files {
		out[i] = files[i].State
	}
	return out
}

func TestImport_ReportsWhereItIsThenHowItEnded(t *testing.T) {
	cover := []byte("\xff\xd8\xff\xe0 a cover")
	c, id, _, source := newImportCore(t, Options{musicbrainz: fakeCatalogue{covers: map[string][]byte{"release": cover}}})

	plan := startImport(t, c, id, source)

	progress := nextEvent[ImportProgressed](t, c)
	if want := []FileState{FileWorking, FilePending, FilePending}; progress.DownloadID != id || !slices.Equal(fileStates(progress.Files), want) {
		t.Errorf("first ImportProgressed = %+v, want download %d with its first file being imported", progress, id)
	}

	finished := nextEvent[ImportFinished](t, c)
	if finished.DownloadID != id || finished.Artist != "Low" || finished.Album != "Trust" || finished.Err != nil {
		t.Errorf("ImportFinished = %+v, want download %d imported as Low - Trust", finished, id)
	}
	if !AllDone(finished.Files) || len(finished.Files) != 3 {
		t.Fatalf("files ended %v, want the three done", fileStates(finished.Files))
	}
	for i, f := range finished.Files {
		if want := filepath.Join(source, plan.Files[i].Dest); f.Path != plan.Files[i].Source || f.Dest != want {
			t.Errorf("file %d went from %s to %s, want from %s to %s", i, f.Path, f.Dest, plan.Files[i].Source, want)
		}
	}
	// The cover PrepareImport fetched is the album's.
	if got, err := os.ReadFile(filepath.Join(filepath.Dir(finished.Files[0].Dest), "cover.jpg")); err != nil || !bytes.Equal(got, cover) {
		t.Errorf("album cover = %q (err %v), want the one of the release", got, err)
	}
	// A client told the import is over can start another.
	if err := c.Downloads().Import(plan, source); errors.Is(err, ErrImportRunning) {
		t.Error("an import is still said to run after ImportFinished")
	}
}

// A file that fails does not stop the others, which are in the library; the
// download is kept, to be imported again once the file is back.
func TestImport_AFileThatFailsKeepsTheDownload(t *testing.T) {
	c, id, completed, source := newImportCore(t, Options{})
	plan, err := c.Downloads().PrepareImport(id)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// Gone between the preview and the import.
	if err := os.Remove(filepath.Join(completed, "Album", "02.mp3")); err != nil {
		t.Fatal(err)
	}

	if err := c.Downloads().Import(plan, source); err != nil {
		t.Fatalf("import: %v", err)
	}

	finished := nextEvent[ImportFinished](t, c)
	if want := []FileState{FileDone, FileFailed, FileDone}; !slices.Equal(fileStates(finished.Files), want) || finished.Err != nil {
		t.Fatalf("ImportFinished = %+v, want the second file failed and no other failure", finished)
	}
	if finished.Files[1].Err == nil {
		t.Error("the failed file carries no error")
	}
	if tracks, err := c.Library().Tracks("Low", "Trust"); err != nil || len(tracks) != 2 {
		t.Errorf("library tracks = %+v (err %v), want the two imported", tracks, err)
	}
	if all, err := c.Downloads().List(); err != nil || len(all) != 1 || all[0].ID != id {
		t.Errorf("downloads = %+v (err %v), want the download kept", all, err)
	}
}

func TestImport_OneRunsAtATime(t *testing.T) {
	c, id, _, source := newImportCore(t, Options{})

	plan := startImport(t, c, id, source)
	// The first one has three files to tag and move: it is still running.
	if err := c.Downloads().Import(plan, source); !errors.Is(err, ErrImportRunning) {
		t.Errorf("a second Import = %v, want ErrImportRunning", err)
	}

	if finished := nextEvent[ImportFinished](t, c); !AllDone(finished.Files) {
		t.Errorf("the first import ended %v, want it untouched by the second", fileStates(finished.Files))
	}
}

func TestImport_RefusesAFolderThatIsNotALibrarySource(t *testing.T) {
	c, id, completed, _ := newImportCore(t, Options{})
	plan, err := c.Downloads().PrepareImport(id)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if err := c.Downloads().Import(plan, t.TempDir()); err == nil {
		t.Fatal("Import into a folder that is not a library source was accepted")
	}

	expectNoEvent[ImportProgressed](t, c)
	if _, err := os.Stat(filepath.Join(completed, "Album", "01.mp3")); err != nil {
		t.Errorf("a file left the download folder: %v", err)
	}
}

// The tracks are in the library when Finish import fails: the import is a
// success, and the failure to clean up is reported apart.
func TestImport_AFailedFinishImportIsNotAFailedImport(t *testing.T) {
	mgr := openTestState(t)
	c, id, _, source := newImportCore(t, Options{state: mgr})
	plan, err := c.Downloads().PrepareImport(id)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// Finish import reads the download first: it is gone.
	if _, err := mgr.DB().Exec(`DELETE FROM downloads`); err != nil {
		t.Fatalf("delete download: %v", err)
	}

	if err := c.Downloads().Import(plan, source); err != nil {
		t.Fatalf("import: %v", err)
	}

	if e := waitFailed(t, c, errmsg.OpDownloadCleanup); e.Err == nil {
		t.Error("Failed carries no error")
	}
	if finished := nextEvent[ImportFinished](t, c); !AllDone(finished.Files) || finished.Err != nil {
		t.Errorf("ImportFinished = %+v, want every file done and no failure", finished)
	}
}

// Close ends an import after the file it is on, and waits for it: what was
// moved is in the library, the rest is where it was, nothing is reported.
func TestClose_EndsAnImportAfterTheFileItIsOn(t *testing.T) {
	// Long enough to still run when Close comes.
	boxSet := lowTrust()
	boxSet.Tracks = nil
	for i := range 80 {
		boxSet.Tracks = append(boxSet.Tracks, MBTrack{Position: i + 1, Title: fmt.Sprintf("Track %d", i+1)})
	}
	// On a database the core opens and closes itself, as a client's does:
	// the import must find it open until its last write.
	database := filepath.Join(t.TempDir(), "waves.db")
	c, id, _, source := newImportCoreOf(t, Options{Database: database}, boxSet)
	plan := startImport(t, c, id, source)
	nextEvent[ImportProgressed](t, c) // the first file is being imported

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	mgr, err := state.OpenAt(database)
	if err != nil {
		t.Fatalf("open the database again: %v", err)
	}
	defer mgr.Close()
	// The import is over: nothing moves any more.
	inLibrary := 0
	for _, f := range plan.Files {
		dest := filepath.Join(source, f.Dest)
		_, moved := os.Stat(dest)
		_, left := os.Stat(f.Source)
		if (moved == nil) == (left == nil) {
			t.Errorf("%s: in the library %v, in the download folder %v; want exactly one", f.Filename, moved == nil, left == nil)
		}
		var known int
		if err := mgr.DB().QueryRow(`SELECT COUNT(*) FROM library_tracks WHERE path = ?`, dest).Scan(&known); err != nil {
			t.Fatalf("read library: %v", err)
		}
		if (known == 1) != (moved == nil) {
			t.Errorf("%s: moved %v but the library knows %d track at its destination", f.Filename, moved == nil, known)
		}
		inLibrary += known
	}
	if inLibrary == 0 || inLibrary == len(plan.Files) {
		t.Errorf("%d of %d files imported: want the file Close found it on, and not those after", inLibrary, len(plan.Files))
	}
	for e := range c.Events() {
		switch e.(type) {
		case Failed, ImportFinished:
			t.Errorf("Close reported %+v", e)
		}
	}
}

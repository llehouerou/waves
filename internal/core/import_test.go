package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/slskd/slskdtest"
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
	f := newImportCoreOf(t, opts, lowTrust())
	return f.c, f.id, f.completed, f.source
}

// importCore is a core built by newImportCoreOf, and what it imports from.
type importCore struct {
	c         *Core
	fake      *slskdtest.Server
	release   *MBReleaseDetails
	id        int64  // the download of release
	completed string // slskd's completed folder
	source    string // the library source
}

// newImportCoreOf is newImportCore for the tracks of release, downloaded as
// 01.mp3, 02.mp3 and so on.
func newImportCoreOf(t *testing.T, opts Options, release *MBReleaseDetails) importCore {
	t.Helper()
	f := importCore{release: release, source: t.TempDir()}
	opts.LibrarySources = []string{f.source}
	f.c, f.fake, f.completed = newDownloadsCore(t, opts)
	f.id = f.download(t)
	return f
}

// download queues f.release, finished downloading in the completed folder,
// and returns the download's ID.
func (f importCore) download(t *testing.T) int64 {
	t.Helper()
	names := make([]string, len(f.release.Tracks))
	for i := range names {
		names[i] = fmt.Sprintf("%02d.mp3", i+1)
	}
	d := album(names...)
	d.MBReleaseDetails = f.release
	d.MBReleaseGroup = &MBReleaseGroup{ID: "rg", PrimaryType: "Album", FirstRelease: "2002"}
	id, err := f.c.Downloads().Queue(d)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	for _, file := range d.Files {
		tagstest.WriteMP3(t, ExpectedDiskPath(f.completed, file.Filename), FileTags{})
	}
	return id
}

// boxSet is a release of 80 tracks: its import still runs long after its
// first ImportProgressed.
func boxSet() *MBReleaseDetails {
	release := lowTrust()
	release.Tracks = nil
	for i := range 80 {
		release.Tracks = append(release.Tracks, MBTrack{Position: i + 1, Title: fmt.Sprintf("Track %d", i+1)})
	}
	return release
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

// Removed from the library, its files kept on disk, an album imported again
// still fails: the files at the destinations are never replaced. Each failed
// file says where the file is and that it must be deleted from disk, not
// from the library, which no longer has it.
func TestImport_AFileAtTheDestinationFailsEvenOnceRemovedFromTheLibrary(t *testing.T) {
	f := newImportCoreOf(t, Options{}, lowTrust())
	startImport(t, f.c, f.id, f.source)
	if finished := nextEvent[ImportFinished](t, f.c); !AllDone(finished.Files) {
		t.Fatalf("the first import ended %v, want all done", fileStates(finished.Files))
	}
	tracks, err := f.c.Library().Tracks("Low", "Trust")
	if err != nil || len(tracks) != 3 {
		t.Fatalf("library tracks = %+v (err %v), want the three imported", tracks, err)
	}
	for _, track := range tracks {
		if err := f.c.Library().DeleteTrack(track.ID); err != nil {
			t.Fatalf("remove from library: %v", err)
		}
	}

	plan := startImport(t, f.c, f.download(t), f.source)

	finished := nextEvent[ImportFinished](t, f.c)
	if want := []FileState{FileFailed, FileFailed, FileFailed}; !slices.Equal(fileStates(finished.Files), want) {
		t.Fatalf("the second import ended %v, want every file failed", fileStates(finished.Files))
	}
	for i, file := range finished.Files {
		dest := filepath.Join(f.source, plan.Files[i].Dest)
		want := "a file is already at " + dest + ": delete it from disk to replace it"
		if file.Err == nil || file.Err.Error() != want {
			t.Errorf("file %d failed with %v, want %q", i+1, file.Err, want)
		}
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("file %d: the file at the destination: %v", i+1, err)
		}
	}
	if tracks, err := f.c.Library().Tracks("Low", "Trust"); err != nil || len(tracks) != 0 {
		t.Errorf("library tracks = %+v (err %v), want none", tracks, err)
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
	c, id, _, source := newImportCore(t, Options{})
	plan, err := c.Downloads().PrepareImport(id)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// Finish import reads the download first: it is gone.
	if _, err := c.state.DB().Exec(`DELETE FROM downloads`); err != nil {
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
	// The import must find the database open until its last write; the box
	// set is long enough to still run when Close comes.
	database := filepath.Join(t.TempDir(), "waves.db")
	f := newImportCoreOf(t, Options{Database: database}, boxSet())
	c, id, source := f.c, f.id, f.source
	plan := startImport(t, c, id, source)
	nextEvent[ImportProgressed](t, c) // the first file is being imported

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	mgr := reopen(t, database)
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

// queueOther queues alice's "Other" folder next to the download of f, and
// writes its file where slskd would. It returns the download's ID.
func (f importCore) queueOther(t *testing.T) int64 {
	t.Helper()
	id, err := f.c.Downloads().Queue(Download{
		MBReleaseGroupID: "other", MBArtistName: "Other", MBAlbumTitle: "Other",
		SlskdUsername: "alice", SlskdDirectory: `@@alice\Music\Other`,
		Files: []DownloadFile{{Filename: otherFile, Size: 3}},
	})
	if err != nil {
		t.Fatalf("queue the other download: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(f.completed, "Other"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.completed, "Other", "01.flac"), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}

// otherFile is the slskd path of the file of alice's "Other".
const otherFile = `@@alice\Music\Other\01.flac`

// downloaded has the fake slskd report every file of f's release and of
// alice's "Other" as downloaded.
func (f importCore) downloaded() {
	transfers := make([]slskd.DownloadFile, 0, 1+len(f.release.Tracks))
	transfers = append(transfers, slskd.DownloadFile{
		ID: "t-alice", Username: "alice", Filename: otherFile, State: "Completed, Succeeded", BytesTransferred: 3,
	})
	for i := range f.release.Tracks {
		transfers = append(transfers, transfer(fmt.Sprintf("%02d.mp3", i+1), "Completed, Succeeded"))
	}
	f.fake.SetTransfers(transfers...)
}

// completeAll has the fake slskd report every download as downloaded, and
// waits for a Sync to mark them completed.
func (f importCore) completeAll(t *testing.T) {
	t.Helper()
	f.downloaded()
	f.c.Downloads().Sync()
	eventually(t, "a Sync marks the downloads completed", func() bool {
		all, err := f.c.Downloads().List()
		if err != nil || len(all) == 0 {
			return false
		}
		for i := range all {
			if all[i].Status != DownloadCompleted {
				return false
			}
		}
		return true
	})
}

// importEndsUntouched waits for the import of f's download to end, and checks
// it ended as if nothing had been asked of the download meanwhile: every file
// in the library, Finish import done with no failure, the emptied download
// folder removed.
func (f importCore) importEndsUntouched(t *testing.T) {
	t.Helper()
	files := len(f.release.Tracks)
	var finished ImportFinished
	deadline := time.After(5 * time.Second)
	for finished.Files == nil {
		select {
		case e := <-f.c.Events():
			switch e := e.(type) {
			case Failed:
				if e.Op == errmsg.OpDownloadCleanup {
					t.Errorf("Finish import failed: %v", e.Err)
				}
			case ImportFinished:
				finished = e
			}
		case <-deadline:
			t.Fatal("no ImportFinished")
		}
	}
	if !AllDone(finished.Files) || len(finished.Files) != files || finished.Err != nil {
		t.Errorf("ImportFinished = %v (err %v), want the %d files done", fileStates(finished.Files), finished.Err, files)
	}
	if tracks, err := f.c.Library().Tracks("Low", "Trust"); err != nil || len(tracks) != files {
		t.Errorf("%d tracks in the library (err %v), want %d", len(tracks), err, files)
	}
	all, err := f.c.Downloads().List()
	if err != nil {
		t.Fatal(err)
	}
	for i := range all {
		if all[i].ID == f.id {
			t.Errorf("download %d kept after its import", f.id)
		}
	}
	if _, err := os.Stat(filepath.Join(f.completed, "Album")); !os.IsNotExist(err) {
		t.Errorf("the download folder is still there: %v", err)
	}
}

func TestDelete_RefusesTheDownloadBeingImported(t *testing.T) {
	f := newImportCoreOf(t, Options{}, boxSet())
	f.downloaded() // transfers a Delete would cancel
	startImport(t, f.c, f.id, f.source)
	nextEvent[ImportProgressed](t, f.c) // the first file is being imported

	err := f.c.Downloads().Delete(f.id)
	if !errors.Is(err, ErrDownloadImporting) || !strings.Contains(err.Error(), "being imported") {
		t.Errorf("Delete = %v, want ErrDownloadImporting", err)
	}
	if dropped := f.fake.Dropped(); len(dropped) != 0 {
		t.Errorf("slskd dropped %v, want nothing", dropped)
	}
	if all, err := f.c.Downloads().List(); err != nil || len(all) != 1 || all[0].ID != f.id {
		t.Errorf("downloads = %+v (err %v), want the one being imported kept", all, err)
	}

	f.importEndsUntouched(t)
}

func TestDelete_DeletesAnotherDownloadWhileOneIsImported(t *testing.T) {
	f := newImportCoreOf(t, Options{}, boxSet())
	other := f.queueOther(t)
	startImport(t, f.c, f.id, f.source)
	nextEvent[ImportProgressed](t, f.c)

	if err := f.c.Downloads().Delete(other); err != nil {
		t.Fatalf("Delete of the other download: %v", err)
	}
	if all, err := f.c.Downloads().List(); err != nil || len(all) != 1 || all[0].ID != f.id {
		t.Errorf("downloads = %+v (err %v), want only the one being imported", all, err)
	}
	if _, err := os.Stat(filepath.Join(f.completed, "Other")); !os.IsNotExist(err) {
		t.Errorf("the other download folder is still there: %v", err)
	}

	f.importEndsUntouched(t)
}

func TestClearCompleted_HoldsBackTheDownloadBeingImported(t *testing.T) {
	f := newImportCoreOf(t, Options{}, boxSet())
	f.queueOther(t)
	f.completeAll(t)
	startImport(t, f.c, f.id, f.source)
	nextEvent[ImportProgressed](t, f.c)

	err := f.c.Downloads().ClearCompleted()
	if !errors.Is(err, ErrDownloadImporting) || !strings.Contains(err.Error(), "being imported") {
		t.Errorf("ClearCompleted = %v, want ErrDownloadImporting", err)
	}
	if all, err := f.c.Downloads().List(); err != nil || len(all) != 1 || all[0].ID != f.id {
		t.Errorf("downloads = %+v (err %v), want only the one being imported", all, err)
	}
	if want := []string{"alice/t-alice?remove=true"}; !slices.Equal(f.fake.Dropped(), want) {
		t.Errorf("slskd dropped %v, want the other download's transfer only", f.fake.Dropped())
	}

	f.importEndsUntouched(t)
}

// A download being imported that is not completed is not one ClearCompleted
// would drop: nothing is held back.
func TestClearCompleted_DropsAsBeforeWhenTheDownloadImportedIsNotCompleted(t *testing.T) {
	f := newImportCoreOf(t, Options{}, boxSet())
	startImport(t, f.c, f.id, f.source)
	nextEvent[ImportProgressed](t, f.c)

	if err := f.c.Downloads().ClearCompleted(); err != nil {
		t.Errorf("ClearCompleted = %v, want no error", err)
	}
	nextEvent[ImportFinished](t, f.c)
}

// An import that left failed files is over: its download, still there, is
// deleted or cleared as any other.
func TestDeleteAndClearCompleted_ActOnADownloadOnceItsImportIsOver(t *testing.T) {
	commands := map[string]func(dl *Downloads, id int64) error{
		"Delete":         (*Downloads).Delete,
		"ClearCompleted": func(dl *Downloads, _ int64) error { return dl.ClearCompleted() },
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			f := newImportCoreOf(t, Options{}, lowTrust())
			f.completeAll(t)
			plan, err := f.c.Downloads().PrepareImport(f.id)
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			if err := os.Remove(filepath.Join(f.completed, "Album", "02.mp3")); err != nil {
				t.Fatal(err)
			}
			if err := f.c.Downloads().Import(plan, f.source); err != nil {
				t.Fatalf("import: %v", err)
			}
			if finished := nextEvent[ImportFinished](t, f.c); AllDone(finished.Files) {
				t.Fatal("every file was imported: the download is gone already")
			}

			if err := command(f.c.Downloads(), f.id); err != nil {
				t.Errorf("%s = %v, want no error", name, err)
			}
			if all, err := f.c.Downloads().List(); err != nil || len(all) != 0 {
				t.Errorf("downloads = %+v (err %v), want none", all, err)
			}
		})
	}
}

// inFlight runs command until it reaches slskd to drop a transfer, and holds
// it there: done receives its error once the test releases slskd.
func (f importCore) inFlight(t *testing.T, command func() error) (done <-chan error, release func()) {
	t.Helper()
	held, release := f.fake.HoldDrops(t)
	result := make(chan error, 1)
	go func() { result <- command() }()
	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("the command never reached slskd")
	}
	return result, release
}

// atOnce calls Import, and fails the test unless it returns at once.
func (f importCore) atOnce(t *testing.T, plan ImportPlan) error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- f.c.Downloads().Import(plan, f.source) }()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatal("Import waits for the command in flight")
		return nil
	}
}

// A Delete waiting for slskd does not hold up the import of another
// download: Import returns at once.
func TestImport_DoesNotWaitForADeleteOfAnotherDownload(t *testing.T) {
	f := newImportCoreOf(t, Options{}, lowTrust())
	other := f.queueOther(t)
	f.downloaded() // a transfer the Delete cancels
	plan, err := f.c.Downloads().PrepareImport(f.id)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	deleted, release := f.inFlight(t, func() error { return f.c.Downloads().Delete(other) })

	if err := f.atOnce(t, plan); err != nil {
		t.Fatalf("import: %v", err)
	}
	release()
	if err := <-deleted; err != nil {
		t.Errorf("Delete of the other download: %v", err)
	}
	f.importEndsUntouched(t)
}

// An import never starts on a download a Delete or a ClearCompleted is
// working on: it is refused at once, and the command runs to its end.
func TestImport_RefusesADownloadBeingDeletedOrCleared(t *testing.T) {
	commands := map[string]func(dl *Downloads, id int64) error{
		"Delete":         (*Downloads).Delete,
		"ClearCompleted": func(dl *Downloads, _ int64) error { return dl.ClearCompleted() },
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			f := newImportCoreOf(t, Options{}, lowTrust())
			f.completeAll(t)
			plan, err := f.c.Downloads().PrepareImport(f.id)
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			done, release := f.inFlight(t, func() error { return command(f.c.Downloads(), f.id) })

			if err := f.atOnce(t, plan); err == nil {
				t.Error("Import of a download being removed was accepted")
			}
			release()
			if err := <-done; err != nil {
				t.Errorf("%s = %v, want no error", name, err)
			}
			if all, err := f.c.Downloads().List(); err != nil || len(all) != 0 {
				t.Errorf("downloads = %+v (err %v), want none", all, err)
			}
			expectNoEvent[ImportProgressed](t, f.c)
		})
	}
}

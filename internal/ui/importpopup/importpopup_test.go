package importpopup

import (
	"errors"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeImporter is the core's import as the popup sees it: it hands out the
// plan it holds and records what it is asked.
type fakeImporter struct {
	plan       core.ImportPlan
	prepareErr error
	importErr  error
	prepared   []int64  // the downloads PrepareImport was asked for
	imported   []string // the library sources Import was asked for
}

func (f *fakeImporter) PrepareImport(id int64) (core.ImportPlan, error) {
	f.prepared = append(f.prepared, id)
	return f.plan, f.prepareErr
}

func (f *fakeImporter) Import(_ core.ImportPlan, source string) error {
	if f.importErr != nil {
		return f.importErr
	}
	f.imported = append(f.imported, source)
	return nil
}

func sampleDownload() *core.Download {
	return &core.Download{ID: 1, MBArtistName: "Test Artist", MBAlbumTitle: "Test Album"}
}

// samplePlan is the plan of sampleDownload: three files, one per track.
func samplePlan() core.ImportPlan {
	plan := core.ImportPlan{
		DownloadID: 1,
		Release: &core.MBReleaseDetails{
			Release: core.MBRelease{ID: "abc123", Title: "Test Album", Artist: "Test Artist", Date: "2024-01-15"},
			Tracks:  []core.MBTrack{{Position: 1, Title: "Track One"}, {Position: 2, Title: "Track Two"}, {Position: 3, Title: "Track Three"}},
		},
		ReleaseGroup: &core.MBReleaseGroup{ID: "rg123", Title: "Test Album", PrimaryType: "Album", FirstRelease: "2024-01-15"},
	}
	for i, name := range []string{"01 - Track One.flac", "02 - Track Two.flac", "03 - Track Three.flac"} {
		plan.Files = append(plan.Files, core.ImportFile{
			Filename: `@@bob\Test Album\` + name,
			Source:   "/downloads/Test Album/" + name,
			Track:    i,
			Dest:     "Test Artist/2024 • Test Album/" + name,
		})
	}
	return plan
}

func sampleLibrarySources() []string {
	return []string{"/music/library1", "/music/library2"}
}

// openPopup opens the import popup of sampleDownload on an importer holding
// plan. Its Init is in the harness, not run yet.
func openPopup(plan core.ImportPlan) (*testutil.PopupHarness, *Model, *fakeImporter) {
	imp := &fakeImporter{plan: plan}
	m := New(sampleDownload(), sampleLibrarySources(), imp)
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m), m, imp
}

// preparedPopup is openPopup once the core has answered with the plan: the
// popup shows the tag preview.
func preparedPopup(t *testing.T, plan core.ImportPlan) (*testutil.PopupHarness, *Model, *fakeImporter) {
	t.Helper()
	h, m, imp := openPopup(plan)
	h.ExecuteAndSend(h.LastCommand())
	h.ClearCommands()
	return h, m, imp
}

// importingPopup is preparedPopup taken to the import of the first library
// source, which the core has started.
func importingPopup(t *testing.T) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m, _ := preparedPopup(t, samplePlan())
	h.SendEnter() // path preview
	h.SendEnter() // start import
	if m.state != StateImporting {
		t.Fatalf("import not started: state %d", m.state)
	}
	h.ClearCommands()
	return h, m
}

// progress is where the import of samplePlan is: one state per file.
func progress(states ...core.FileState) []core.FileProgress {
	plan := samplePlan()
	files := make([]core.FileProgress, len(states))
	for i, state := range states {
		files[i] = core.FileProgress{Path: plan.Files[i].Source, State: state}
		switch state { //nolint:exhaustive // the other states carry nothing more
		case core.FileDone, core.FileAlreadyImported:
			files[i].Dest = "/music/library1/" + plan.Files[i].Dest
		case core.FileFailed:
			files[i].Err = errors.New("disk full")
		}
	}
	return files
}

func closes(t *testing.T, h *testutil.PopupHarness) bool {
	t.Helper()
	msg, ok := testutil.ExecuteCmd(h.LastCommand()).(action.Msg)
	if !ok {
		return false
	}
	_, ok = msg.Action.(Close)
	return ok
}

func assertView(t *testing.T, h *testutil.PopupHarness, contains ...string) {
	t.Helper()
	for _, s := range contains {
		if err := h.AssertViewContains(s); err != "" {
			t.Error(err)
		}
	}
}

// === Preparing ===

func TestImport_AsksTheCoreForThePlanAndWaitsForIt(t *testing.T) {
	h, m, imp := openPopup(samplePlan())

	assertView(t, h, "Import: Test Artist - Test Album", "Preparing import...")
	h.SendEnter() // nothing to continue to yet
	if m.state != StateTagPreview {
		t.Errorf("state = %d before the plan arrived, want the tag preview", m.state)
	}
	if len(imp.prepared) != 0 {
		t.Error("the plan was asked for on the UI goroutine")
	}

	h.ExecuteAndSend(h.Commands()[0]) // Init, off the UI goroutine

	if !slices.Equal(imp.prepared, []int64{1}) {
		t.Errorf("PrepareImport asked for %v, want download 1", imp.prepared)
	}
	assertView(t, h, "Tag Changes Preview", "3 files will be retagged", "[Enter] Continue   [Esc] Cancel")
}

func TestImport_APlanThatCannotBeMadeIsSaid(t *testing.T) {
	h, m, imp := openPopup(samplePlan())
	imp.prepareErr = errors.New("the download has no MusicBrainz release")

	h.ExecuteAndSend(h.LastCommand())

	assertView(t, h, "the download has no MusicBrainz release")
	h.SendEnter()
	if m.state != StateTagPreview {
		t.Errorf("state = %d, want to stay on the failure", m.state)
	}
	h.SendEscape()
	if !closes(t, h) {
		t.Error("Esc does not close the popup")
	}
}

// A popup closed while its plan was being made leaves the answer in flight:
// it must not land in the import popup of another download.
func TestImport_IgnoresWhatIsAboutAnotherDownload(t *testing.T) {
	h, m, _ := openPopup(samplePlan())
	other := samplePlan()
	other.DownloadID = 2
	other.Release.ID = "elsewhere"

	h.SendMsg(PreparedMsg{DownloadID: 2, Plan: other})
	if !m.preparing || m.plan.Release != nil {
		t.Fatalf("took another download's plan: %+v", m.plan)
	}

	h.SendMsg(PreparedMsg{DownloadID: 1, Plan: samplePlan()})
	h.SendEnter() // path preview
	h.SendEnter() // start import
	h.SendMsg(core.ImportProgressed{DownloadID: 2, Files: progress(core.FileFailed, core.FileFailed, core.FileFailed)})
	h.SendMsg(core.ImportFinished{DownloadID: 2, Files: progress(core.FileFailed, core.FileFailed, core.FileFailed)})
	if m.state != StateImporting {
		t.Errorf("state = %d after another download's import finished, want still importing", m.state)
	}
	if err := h.AssertViewNotContains("disk full"); err != "" {
		t.Errorf("shows another download's files: %s", err)
	}
}

// === Tag preview ===

func TestImport_TagPreviewShowsWhatTheImportWrites(t *testing.T) {
	h, _, _ := preparedPopup(t, samplePlan())

	assertView(t, h, "Tags", "Paths", "Album Artist", "Test Artist", "2024-01-15", "Cover Art:", "Not available")
}

func TestImport_TagPreviewTellsTheCoverTheCoreFetched(t *testing.T) {
	plan := samplePlan()
	plan.Cover = make([]byte, 2048)
	h, _, _ := preparedPopup(t, plan)

	assertView(t, h, "Found (2.0 KB, will be embedded)")
}

// The tag preview compares each file's title with its own track's: disc 2's
// files already titled as disc 2's tracks have nothing to change.
func TestImport_TagPreviewComparesEachFileWithItsTrack(t *testing.T) {
	titlesChanged := func(secondTitle string) bool {
		plan := samplePlan()
		plan.Release.DiscCount = 2
		plan.Release.Tracks = []core.MBTrack{
			{DiscNumber: 1, Position: 1, Title: "One A"}, {DiscNumber: 1, Position: 2, Title: "One B"},
			{DiscNumber: 2, Position: 1, Title: "Two A"}, {DiscNumber: 2, Position: 2, Title: "Two B"},
		}
		plan.Files = plan.Files[:2] // the files of disc 2
		plan.Files[0].Track, plan.Files[0].Tags.Title = 2, "Two A"
		plan.Files[1].Track, plan.Files[1].Tags.Title = 3, secondTitle
		_, m, _ := preparedPopup(t, plan)
		for _, d := range m.tagDiffs {
			if d.Field == "Track Titles" {
				return d.Changed
			}
		}
		t.Fatal("no Track Titles line in the preview")
		return false
	}

	if titlesChanged("Two B") {
		t.Error("titles reported changed though each file has its track's title")
	}
	if !titlesChanged("One B") {
		t.Error("a wrong title not reported")
	}
}

func TestImport_EscapeFromTagPreviewCloses(t *testing.T) {
	h, _, _ := preparedPopup(t, samplePlan())

	h.SendEscape()

	if !closes(t, h) {
		t.Error("Esc does not close the popup")
	}
}

// === Path preview ===

func TestImport_PathPreviewShowsWhereEachFileGoes(t *testing.T) {
	h, m, _ := preparedPopup(t, samplePlan())

	h.SendEnter()

	if m.state != StatePathPreview {
		t.Fatalf("state = %d, want the path preview", m.state)
	}
	assertView(t, h, "Destination Library:", "> /music/library1", "File Paths",
		"01 - Track One.flac", "Test Artist/2024 • Test Album/02 - Track Two", // the column is cut there
		"[Enter] Start Import   [j/k] Select Library   [Esc] Back")
}

// An extra file has no destination: the preview says so.
func TestImport_PathPreviewSaysAFileHasNoTrack(t *testing.T) {
	plan := samplePlan()
	plan.Files = append(plan.Files, core.ImportFile{Filename: `@@bob\Test Album\04 - Bonus.flac`, Track: -1})
	h, _, _ := preparedPopup(t, plan)

	h.SendEnter()

	assertView(t, h, "04 - Bonus.flac", "no MusicBrainz track: not imported")
}

func TestImport_PathPreviewWithOneLibrarySourceShowsItInline(t *testing.T) {
	m := New(sampleDownload(), []string{"/music/only"}, &fakeImporter{plan: samplePlan()})
	m.SetSize(100, 40)
	h := testutil.NewPopupHarness(m)
	h.ExecuteAndSend(h.LastCommand())

	h.SendEnter()

	assertView(t, h, "Destination: /music/only", "[Enter] Start Import   [Esc] Back")
}

func TestImport_PathPreviewSelectsTheLibrarySource(t *testing.T) {
	h, m, _ := preparedPopup(t, samplePlan())
	h.SendKey("j")
	if m.selectedSource != 0 {
		t.Error("j moved the library source in the tag preview")
	}
	h.SendEnter()

	h.SendKey("j")
	assertView(t, h, "> /music/library2")
	h.SendKey("j") // wraps around
	assertView(t, h, "> /music/library1")
	h.SendKey("k") // and back
	assertView(t, h, "> /music/library2")
}

func TestImport_EscapeFromPathPreviewGoesBack(t *testing.T) {
	h, m, _ := preparedPopup(t, samplePlan())
	h.SendEnter()

	h.SendEscape()

	if m.state != StateTagPreview || h.LastCommand() != nil {
		t.Errorf("state = %d, want back to the tag preview with the popup open", m.state)
	}
}

// === Importing ===

func TestImport_EnterInPathPreviewStartsTheImportIntoTheSelectedSource(t *testing.T) {
	h, m, imp := preparedPopup(t, samplePlan())
	h.SendEnter()
	h.SendKey("j") // the second library source

	h.SendEnter()

	if !slices.Equal(imp.imported, []string{"/music/library2"}) {
		t.Fatalf("Import asked for %v, want one import into /music/library2", imp.imported)
	}
	if m.state != StateImporting {
		t.Errorf("state = %d, want importing", m.state)
	}
	assertView(t, h, "Importing...", "01 - Track One.flac", "Pending", "Progress: 0/3 files")
}

func TestImport_AnImportTheCoreRefusesIsSaid(t *testing.T) {
	h, m, imp := preparedPopup(t, samplePlan())
	imp.importErr = core.ErrImportRunning
	h.SendEnter()

	h.SendEnter()

	if m.state != StatePathPreview {
		t.Errorf("state = %d, want to stay on the path preview", m.state)
	}
	assertView(t, h, "another import is running")
}

func TestImport_ShowsWhereTheCoreSaysTheImportIs(t *testing.T) {
	h, _ := importingPopup(t)

	h.SendMsg(core.ImportProgressed{DownloadID: 1, Files: progress(core.FileDone, core.FileWorking, core.FilePending)})
	assertView(t, h, "Done", "Importing...", "Pending", "Progress: 1/3 files")

	h.SendMsg(core.ImportProgressed{DownloadID: 1, Files: progress(core.FileAlreadyImported, core.FileFailed, core.FileWorking)})
	assertView(t, h, "Already imported", "disk full", "Progress: 2/3 files")
}

// The core runs the import: closing the popup no longer abandons it half-way.
func TestImport_EscapeWhileImportingClosesAndSaysTheImportGoesOn(t *testing.T) {
	h, _ := importingPopup(t)
	assertView(t, h, "[Esc] Close (the import continues)")

	h.SendEscape()

	if !closes(t, h) {
		t.Error("Esc does not close the popup while importing")
	}
}

// === Complete ===

func TestImport_AFinishedImportShowsItsSuccess(t *testing.T) {
	h, m := importingPopup(t)

	h.SendMsg(core.ImportFinished{DownloadID: 1, Files: progress(core.FileDone, core.FileAlreadyImported, core.FileDone)})

	if m.state != StateComplete {
		t.Fatalf("state = %d, want complete", m.state)
	}
	assertView(t, h, "Import Complete", "3 files imported successfully", "Library index updated",
		"Destination: /music/library1/Test Artist/2024 • Test Album", "[Enter] Close")
	h.SendEnter()
	if !closes(t, h) {
		t.Error("Enter does not close the popup")
	}
}

// Files import in track order; a failure names the file that failed.
func TestImport_AFinishedImportNamesTheFilesThatFailed(t *testing.T) {
	h, _ := importingPopup(t)

	h.SendMsg(core.ImportFinished{DownloadID: 1, Files: progress(core.FileFailed, core.FileDone, core.FileDone)})

	assertView(t, h, "Import Completed with Errors", "2 files imported successfully", "1 files failed",
		"01 - Track One.flac: disk full")
	if err := h.AssertViewNotContains("02 - Track Two.flac"); err != "" {
		t.Errorf("a file that imported is reported as failed: %s", err)
	}
}

// An import where every file failed still reaches the complete screen.
func TestImport_AnImportWhereEveryFileFailedStillCompletes(t *testing.T) {
	h, m := importingPopup(t)

	h.SendMsg(core.ImportFinished{DownloadID: 1, Files: progress(core.FileFailed, core.FileFailed, core.FileFailed)})

	if m.state != StateComplete {
		t.Fatalf("state = %d, want complete", m.state)
	}
	assertView(t, h, "3 files failed")
	h.SendEscape()
	if !closes(t, h) {
		t.Error("Esc does not close the popup")
	}
}

// The files are in place but the library doesn't know them: not a success.
func TestImport_AFinishedImportSaysTheLibraryWasNotUpdated(t *testing.T) {
	h, _ := importingPopup(t)

	h.SendMsg(core.ImportFinished{
		DownloadID: 1,
		Files:      progress(core.FileDone, core.FileDone, core.FileDone),
		Err:        errors.New("database is locked"),
	})

	assertView(t, h, "Import Completed with Errors", "Library not updated: database is locked")
	if err := h.AssertViewNotContains("Library index updated"); err != "" {
		t.Error(err)
	}
}

// === Edge cases ===

func TestImport_EmptyViewWhenZeroSize(t *testing.T) {
	m := New(sampleDownload(), sampleLibrarySources(), &fakeImporter{})

	if view := m.View(); view != "" {
		t.Errorf("View() with zero size = %q, want it empty", view)
	}
}

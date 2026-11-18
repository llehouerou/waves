package importpopup

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
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

// closedPopupsPlan is the plan of download id, or why the core could make
// none, prepared for a popup closed before it came.
func closedPopupsPlan(id int64, err error) tea.Msg {
	plan := samplePlan()
	plan.DownloadID = id
	plan.Release.ID = "elsewhere"
	d := sampleDownload()
	d.ID = id
	h := testutil.NewPopupHarness(New(d, sampleLibrarySources(), &fakeImporter{plan: plan, prepareErr: err}))
	msg := testutil.ExecuteCmd(h.LastCommand())
	h.SendEscape()
	return msg
}

// closedPopupsPlans are the plans, and the failures, prepared for popups
// closed since: of another download, and of the same one.
func closedPopupsPlans() map[string]tea.Msg {
	failed := errors.New("the download has no MusicBrainz release")
	return map[string]tea.Msg{
		"another download":          closedPopupsPlan(2, nil),
		"another download, failed":  closedPopupsPlan(2, failed),
		"the same download":         closedPopupsPlan(1, nil),
		"the same download, failed": closedPopupsPlan(1, failed),
	}
}

// A popup closed while its plan was being made leaves the answer in flight:
// it must not land in the import popup opened since, of another download or
// of the same one.
func TestImport_APreparingPopupDropsThePlansOfClosedPopups(t *testing.T) {
	for name, msg := range closedPopupsPlans() {
		t.Run(name, func(t *testing.T) {
			h, m, _ := openPopup(samplePlan())
			own := h.LastCommand()

			h.AssertDrops(t, msg)
			if !m.preparing || m.plan.Release != nil || m.err != "" {
				t.Fatalf("took the plan of a closed popup: %+v, error %q", m.plan, m.err)
			}

			h.ExecuteAndSend(own)
			if m.preparing || m.plan.Release == nil || m.plan.Release.ID != "abc123" {
				t.Errorf("its own plan is not taken: %+v", m.plan)
			}
		})
	}
}

// Past its plan, the popup drops the plans of closed popups, and its own
// delivered again, in every state.
func TestImport_APopupPastItsPlanDropsEveryPlan(t *testing.T) {
	popups := map[string]func(t *testing.T) (*testutil.PopupHarness, *Model, tea.Msg){
		"tag preview": func(t *testing.T) (*testutil.PopupHarness, *Model, tea.Msg) {
			t.Helper()
			h, m, _ := openPopup(samplePlan())
			own, _ := h.ExecuteAndSend(h.LastCommand())
			return h, m, own
		},
		"path preview": func(t *testing.T) (*testutil.PopupHarness, *Model, tea.Msg) {
			t.Helper()
			h, m, _ := openPopup(samplePlan())
			own, _ := h.ExecuteAndSend(h.LastCommand())
			h.SendEnter()
			return h, m, own
		},
		"importing": func(t *testing.T) (*testutil.PopupHarness, *Model, tea.Msg) {
			t.Helper()
			h, m, _ := openPopup(samplePlan())
			own, _ := h.ExecuteAndSend(h.LastCommand())
			h.SendEnter()
			h.SendEnter()
			return h, m, own
		},
		"complete": func(t *testing.T) (*testutil.PopupHarness, *Model, tea.Msg) {
			t.Helper()
			h, m, _ := openPopup(samplePlan())
			own, _ := h.ExecuteAndSend(h.LastCommand())
			h.SendEnter()
			h.SendEnter()
			h.SendMsg(core.ImportFinished{DownloadID: 1, Files: progress(core.FileDone, core.FileFailed, core.FileDone)})
			return h, m, own
		},
	}
	for state, open := range popups {
		plans := closedPopupsPlans()
		plans["its own, again"] = nil
		for name, msg := range plans {
			t.Run(state+"/"+name, func(t *testing.T) {
				h, m, own := open(t)
				if msg == nil {
					msg = own
				}
				before := m.state
				h.ClearCommands()
				h.AssertDrops(t, msg)
				if m.state != before || m.plan.Release.ID != "abc123" {
					t.Errorf("state %d, release %q; want it left at %d with its own plan", m.state, m.plan.Release.ID, before)
				}
			})
		}
	}
}

// The import of another download is not the popup's own.
func TestImport_IgnoresTheImportOfAnotherDownload(t *testing.T) {
	h, m, _ := preparedPopup(t, samplePlan())
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

// The tag preview shows the release type the import writes, in lower case,
// and compares the files' release type with it exactly.
func TestImport_TagPreviewShowsTheReleaseTypeInLowerCase(t *testing.T) {
	// previewedReleaseType is the Release Type line of the preview of files
	// tagged tagged, once checked to be in the view.
	previewedReleaseType := func(tagged string) TagDiff {
		plan := samplePlan()
		plan.ReleaseGroup.SecondaryTypes = []string{"Live"}
		for i := range plan.Files {
			plan.Files[i].Tags.ReleaseType = tagged
		}
		h, m, _ := preparedPopup(t, plan)
		for _, d := range m.tagDiffs {
			if d.Field == "Release Type" {
				assertView(t, h, d.NewValue)
				return d
			}
		}
		t.Fatal("no Release Type line in the preview")
		return TagDiff{}
	}

	if d := previewedReleaseType(""); d.NewValue != "album; live" || !d.Changed {
		t.Errorf("untagged files: release type becomes %q (changed %v), want album; live", d.NewValue, d.Changed)
	}
	if d := previewedReleaseType("album; live"); d.Changed {
		t.Error("files tagged album; live reported changed")
	}
	if d := previewedReleaseType("album; Live"); !d.Changed {
		t.Error("files tagged album; Live not reported changed")
	}
}

// A release group with no primary type has the types it has: the preview
// shows its release type with no leading separator.
func TestImport_TagPreviewShowsTheReleaseTypeOfAGroupWithNoPrimaryType(t *testing.T) {
	plan := samplePlan()
	plan.ReleaseGroup.PrimaryType = ""
	plan.ReleaseGroup.SecondaryTypes = []string{"Live"}
	h, m, _ := preparedPopup(t, plan)

	for _, d := range m.tagDiffs {
		if d.Field == "Release Type" {
			if d.NewValue != "live" {
				t.Errorf("release type becomes %q, want live", d.NewValue)
			}
			if line := testutil.FindLine(testutil.StripANSI(h.View()), "Release Type"); strings.Contains(line, ";") {
				t.Errorf("Release Type line = %q, want no separator", line)
			}
			return
		}
	}
	t.Fatal("no Release Type line in the preview")
}

// The preview compares the genre and the artist IDs of the files with the
// ones written, joined as tags.Read joins the values of a tag: files whose
// tags read "Rock; Pop" and "idA; idB" have nothing to change there.
func TestImport_TagPreviewComparesTheJoinedValuesWithTheOnesWritten(t *testing.T) {
	plan := samplePlan()
	plan.Release.Genres = []string{"rock", "pop"}
	for i := range plan.Release.Tracks {
		plan.Release.Tracks[i].ArtistIDs = []string{"idA", "idB"}
	}
	for i := range plan.Files {
		plan.Files[i].Tags.Genre, plan.Files[i].Tags.MBArtistID = "Rock; Pop", "idA; idB"
	}
	_, m, _ := preparedPopup(t, plan)

	want := map[string]string{"Genre": "Rock; Pop", "MB Artist ID": "idA; idB"}
	for _, d := range m.tagDiffs {
		if v, ok := want[d.Field]; ok {
			if d.NewValue != v || d.Changed {
				t.Errorf("%s becomes %q (changed %v), want %q unchanged", d.Field, d.NewValue, d.Changed, v)
			}
			delete(want, d.Field)
		}
	}
	if len(want) > 0 {
		t.Errorf("no preview line for %v", want)
	}
}

// A plan settled on the release the files are tagged as carries that
// release's group: the tag preview shows what the import writes from it.
func TestImport_TagPreviewShowsTheGroupOfTheSettledRelease(t *testing.T) {
	plan := samplePlan()
	plan.Release.ID, plan.Release.Date = "B", "2001-05-05"
	plan.ReleaseGroup = &core.MBReleaseGroup{
		ID: "rgB", PrimaryType: "EP", SecondaryTypes: []string{"Live"},
		FirstRelease: "2001-05-05", Genres: []string{"slowcore"},
	}
	h, m, _ := preparedPopup(t, plan)

	want := map[string]string{"Original Date": "2001-05-05", "Release Type": "ep; live", "Genre": "Slowcore"}
	for _, d := range m.tagDiffs {
		if v, ok := want[d.Field]; ok {
			if d.NewValue != v {
				t.Errorf("%s becomes %q, want %q", d.Field, d.NewValue, v)
			}
			assertView(t, h, v)
			delete(want, d.Field)
		}
	}
	if len(want) > 0 {
		t.Errorf("no preview line for %v", want)
	}
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

// The Media line compares each file with the format of its own track's
// medium, which is what the import writes: files Picard tagged CD on a double
// CD, or CD and DVD-Video on a CD + DVD-Video, have nothing to change; files
// waves tagged with the line of every medium ("CD, CD") do, as does a file
// with no Media whose track has a format. A file with no track is not
// imported and does not count.
func TestImport_TagPreviewComparesTheMediaOfEachFileWithItsTracksMedium(t *testing.T) {
	type file struct {
		track int
		media string
	}
	// previewedMedia is the Media line of the preview of files on a release
	// of two tracks, one per medium, of the formats given; once checked to be
	// in the view.
	previewedMedia := func(formats [2]string, files ...file) TagDiff {
		plan := samplePlan()
		plan.Release.DiscCount = 2
		plan.Release.Formats = formats[0] + ", " + formats[1]
		plan.Release.Tracks = []core.MBTrack{
			{DiscNumber: 1, Position: 1, Title: "One", MediumFormat: formats[0]},
			{DiscNumber: 2, Position: 1, Title: "Two", MediumFormat: formats[1]},
		}
		plan.Files = plan.Files[:len(files)]
		for i, f := range files {
			plan.Files[i].Track, plan.Files[i].Tags.Media = f.track, f.media
		}
		h, m, _ := preparedPopup(t, plan)
		for _, d := range m.tagDiffs {
			if d.Field == "Media" {
				assertView(t, h, d.NewValue)
				return d
			}
		}
		t.Fatal("no Media line in the preview")
		return TagDiff{}
	}
	doubleCD, cdAndDVD := [2]string{"CD", "CD"}, [2]string{"CD", "DVD-Video"}

	if d := previewedMedia(doubleCD, file{0, "CD"}, file{1, "CD"}); d.OldValue != "CD" || d.NewValue != "CD" || d.Changed {
		t.Errorf("files tagged CD on a double CD: Media %q -> %q (changed %v), want CD -> CD unchanged", d.OldValue, d.NewValue, d.Changed)
	}
	if d := previewedMedia(cdAndDVD, file{0, "CD"}, file{1, "DVD-Video"}); d.NewValue != "(2 different)" || d.Changed {
		t.Errorf("files tagged as their medium on a CD + DVD-Video: Media becomes %q (changed %v), want (2 different) unchanged", d.NewValue, d.Changed)
	}
	if d := previewedMedia(cdAndDVD, file{0, "DVD-Video"}, file{1, "CD"}); !d.Changed {
		t.Error("files tagged with each other's medium format not reported changed")
	}
	if d := previewedMedia(doubleCD, file{0, "CD, CD"}, file{1, "CD, CD"}); d.NewValue != "CD" || !d.Changed {
		t.Errorf("files tagged CD, CD: Media becomes %q (changed %v), want CD, changed", d.NewValue, d.Changed)
	}
	if d := previewedMedia(doubleCD, file{0, "CD"}, file{1, ""}); !d.Changed {
		t.Error("a file with no Media whose track has a format not reported changed")
	}
	if d := previewedMedia(doubleCD, file{0, "CD"}, file{-1, "Vinyl"}); d.OldValue != "CD" || d.NewValue != "CD" || d.Changed {
		t.Errorf("a file with no track: Media %q -> %q (changed %v), want CD -> CD unchanged", d.OldValue, d.NewValue, d.Changed)
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

// With no library source there is nowhere to import into: the path preview
// says how to add one and offers no Enter.
func TestImport_PathPreviewWithNoLibrarySourceSaysHowToAddOne(t *testing.T) {
	imp := &fakeImporter{plan: samplePlan()}
	m := New(sampleDownload(), nil, imp)
	m.SetSize(100, 40)
	h := testutil.NewPopupHarness(m)
	h.ExecuteAndSend(h.LastCommand())
	bound := keymap.NewResolver(keymap.Bindings)
	libraryView := bound.KeysFor(keymap.ActionViewLibrary)[0]
	sourcesManager := bound.KeysFor(keymap.ActionLibrarySources)[0]

	h.SendEnter()

	if m.state != StatePathPreview {
		t.Fatalf("state = %d, want the path preview", m.state)
	}
	message := []string{
		"No library source to import into.",
		"Press " + libraryView + " for the library view, then " + sourcesManager,
		"to open the library sources manager and add a music folder.",
	}
	assertView(t, h, message...)
	assertView(t, h, "File Paths", "01 - Track One.flac", "[Esc] Back")
	if err := h.AssertViewNotContains("[Enter] Start Import"); err != "" {
		t.Error(err)
	}

	cmd := h.SendEnter()

	if len(imp.imported) != 0 || m.state != StatePathPreview || cmd != nil {
		t.Errorf("Enter started something: imported %v, state %d", imp.imported, m.state)
	}
	assertView(t, h, message...)

	h.SendEscape()

	if m.state != StateTagPreview {
		t.Errorf("state = %d, want back to the tag preview", m.state)
	}
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

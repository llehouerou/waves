package app

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/retag"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// === The import popup fits the screen (issue #316) ===

// planImporter is the core's import as the popup sees it: the plan it holds,
// and an import that starts.
type planImporter struct{ plan core.ImportPlan }

func (p planImporter) PrepareImport(int64) (core.ImportPlan, error) { return p.plan, nil }

func (planImporter) Import(core.ImportPlan, string) error { return nil }

// trackFile is the name of the file of track n (1-based) of a plan.
func trackFile(n int) string {
	return fmt.Sprintf("%02d - Track %02d.flac", n, n)
}

// planOf is the plan of download 1 for files files, one per track.
func planOf(files int) core.ImportPlan {
	plan := core.ImportPlan{
		DownloadID: 1,
		Release: &core.MBReleaseDetails{
			Release: core.MBRelease{ID: "trust-id", Title: "Trust", Artist: "Low", Date: "2002-09-24"},
		},
		ReleaseGroup: &core.MBReleaseGroup{ID: "rg", Title: "Trust", PrimaryType: "Album"},
	}
	for i := range files {
		plan.Release.Tracks = append(plan.Release.Tracks, core.MBTrack{Position: i + 1, Title: fmt.Sprintf("Track %02d", i+1)})
		plan.Files = append(plan.Files, core.ImportFile{
			Filename: `@@bob\Trust\` + trackFile(i+1),
			Source:   "/downloads/Trust/" + trackFile(i+1),
			Track:    i,
			Dest:     "Low/2002 • Trust/" + trackFile(i+1),
		})
	}
	return plan
}

// importOnScreen is m resized to width×height, with the import popup of
// planOf(files) shown on its tag preview, one library source to import into.
func importOnScreen(t *testing.T, m Model, width, height, files int) Model {
	t.Helper()
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	cmd := m.Popups.ShowImport(&core.Download{ID: 1, MBArtistName: "Low", MBAlbumTitle: "Trust"},
		[]string{"/music"}, planImporter{plan: planOf(files)})
	m, _ = updateModel(t, m, cmd())
	if imp := m.Popups.Import(); imp == nil {
		t.Fatal("the import popup is not shown")
	}
	return m
}

func screen(m Model) string {
	return testutil.StripANSI(m.View())
}

// keys sends each key to m.
func keys(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, key := range keys {
		var msg tea.Msg
		switch key {
		case "ctrl+d":
			msg = tea.KeyMsg{Type: tea.KeyCtrlD}
		case "ctrl+u":
			msg = tea.KeyMsg{Type: tea.KeyCtrlU}
		default:
			msg = keyMsg(key)
		}
		m, _ = updateModel(t, m, msg)
	}
	return m
}

var screenRange = regexp.MustCompile(`\((\d+)-(\d+) of (\d+)\)`)

// rangeOnScreen is the line of the screen that holds the range of a list,
// and the range; "" when there is none.
func rangeOnScreen(m Model) (line, r string) {
	s := screen(m)
	r = screenRange.FindString(s)
	if r == "" {
		return "", ""
	}
	return testutil.FindLine(s, r), r
}

// assertBoxEndsAbove checks that the screen shows help, and below it the
// bottom border of the popup's box, of a popup width columns wide, on a row
// before row.
func assertBoxEndsAbove(t *testing.T, m Model, help string, width, row int) {
	t.Helper()
	lines := strings.Split(screen(m), "\n")
	bottom := "╰" + strings.Repeat("─", width-2) + "╯"
	helpRow := -1
	for i, line := range lines {
		if helpRow < 0 && strings.Contains(line, help) {
			helpRow = i
		}
		if helpRow >= 0 && strings.Contains(line, bottom) {
			if i >= row {
				t.Errorf("the box ends on row %d, want before row %d", i, row)
			}
			return
		}
	}
	t.Errorf("%q then the bottom of the box are not on screen:\n%s", help, screen(m))
}

// showsTagLine reports whether the screen lists the tag named field.
func showsTagLine(m Model, field string) bool {
	return regexp.MustCompile(`│\s+` + regexp.QuoteMeta(field) + `\s{2}`).MatchString(screen(m))
}

func TestImportPopup_TagPreviewScrollsOnAShortTerminal(t *testing.T) {
	m := importOnScreen(t, newLibraryTestModel(t, nil), 80, 24, 3)
	assertBoxEndsAbove(t, m, "[Enter] Continue", 64, 24)
	line, r := rangeOnScreen(m)
	if !strings.Contains(line, "[j/k ctrl+d/u g/G] Scroll") || !strings.HasPrefix(r, "(1-") {
		t.Fatalf("range line %q, want the first tags and the scroll keys:\n%s", line, screen(m))
	}
	if !showsTagLine(m, "Artist") {
		t.Errorf("no tag listed:\n%s", screen(m))
	}

	for _, step := range []struct{ key, want string }{
		{"j", "(2-"}, {"k", "(1-"}, {"ctrl+d", ""}, {"ctrl+u", "(1-"},
	} {
		m = keys(t, m, step.key)
		_, got := rangeOnScreen(m)
		if step.want != "" && !strings.HasPrefix(got, step.want) || step.want == "" && got == r {
			t.Errorf("%s: range %q", step.key, got)
		}
	}

	m = keys(t, m, "G")
	if !showsTagLine(m, "MB Release ID") {
		t.Errorf("G: MB Release ID is not on screen:\n%s", screen(m))
	}
	m = keys(t, m, "g")
	if !showsTagLine(m, "Artist") {
		t.Errorf("g: Artist is not on screen:\n%s", screen(m))
	}
}

func TestImportPopup_TagPreviewShowsEveryTagOnATallTerminal(t *testing.T) {
	m := importOnScreen(t, newLibraryTestModel(t, nil), 120, 40, 3)
	if _, r := rangeOnScreen(m); r != "" {
		t.Errorf("range %q, want every tag", r)
	}
	for _, field := range []string{"Artist", "Genre", "MB Release ID"} {
		if !showsTagLine(m, field) {
			t.Errorf("%s is not on screen", field)
		}
	}
	assertBoxEndsAbove(t, m, "[Enter] Continue", 96, 40)
}

func TestImportPopup_PathPreviewListsAFileOnAShortTerminal(t *testing.T) {
	m := importOnScreen(t, newLibraryTestModel(t, nil), 80, 24, 3)
	m = keys(t, m, keyEnter)
	s := screen(m)
	if !strings.Contains(s, trackFile(1)) || strings.Contains(s, "(1-0 of 3)") {
		t.Errorf("no file listed:\n%s", s)
	}
	assertBoxEndsAbove(t, m, "[Enter] Start Import", 64, 24)
}

func TestImportPopup_PathPreviewListsEveryFileOnALargeTerminal(t *testing.T) {
	m := importOnScreen(t, newLibraryTestModel(t, nil), 200, 60, 30)
	m = keys(t, m, keyEnter)
	for n := 1; n <= 30; n++ {
		if !strings.Contains(screen(m), trackFile(n)) {
			t.Errorf("%s is not listed", trackFile(n))
		}
	}
	if _, r := rangeOnScreen(m); r != "" {
		t.Errorf("range %q, want every file", r)
	}
	assertBoxEndsAbove(t, m, "[Enter] Start Import", 160, 60)
}

// progressOf is where the import of planOf(30) is: the files before working
// done, the one at working (1-based; none at 0) being imported, the others
// pending.
func progressOf(working int) []core.FileProgress {
	files := make([]core.FileProgress, 30)
	for i := range files {
		files[i] = core.FileProgress{Path: "/downloads/Trust/" + trackFile(i+1), State: core.FilePending}
		switch {
		case working == 0 || i+1 < working:
			files[i].State = core.FileDone
		case i+1 == working:
			files[i].State = core.FileWorking
		}
	}
	return files
}

func TestImportPopup_ProgressFollowsTheImportOnScreen(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := importOnScreen(t, newLibraryTestModel(t, nil), size[0], size[1], 30)
			m = keys(t, m, keyEnter, keyEnter)
			assertBoxEndsAbove(t, m, "[Esc] Close", size[0]*80/100, size[1])
			line, r := rangeOnScreen(m)
			if r == "" || strings.Contains(line, "Scroll") || !strings.Contains(screen(m), "Progress:") {
				t.Errorf("range line %q, want one that names no key, and the progress:\n%s", line, screen(m))
			}

			m, _ = updateModel(t, m, ServiceImportMsg{Event: core.ImportProgressed{DownloadID: 1, Files: progressOf(20)}})
			if !strings.Contains(screen(m), trackFile(20)) {
				t.Errorf("the file being imported is not on screen:\n%s", screen(m))
			}
			m, _ = updateModel(t, m, ServiceImportMsg{Event: core.ImportProgressed{DownloadID: 1, Files: progressOf(0)}})
			if !strings.Contains(screen(m), trackFile(30)) {
				t.Errorf("the last file is not on screen:\n%s", screen(m))
			}
			assertBoxEndsAbove(t, m, "[Esc] Close", size[0]*80/100, size[1])
		})
	}
}

// failedOnScreen is importOnScreen once the import of its 30 files failed.
func failedOnScreen(t *testing.T, m Model, width, height int) Model {
	t.Helper()
	m = importOnScreen(t, m, width, height, 30)
	m = keys(t, m, keyEnter, keyEnter)
	files := progressOf(0)
	for i := range files {
		files[i].State, files[i].Err = core.FileFailed, errors.New("disk full")
	}
	m, _ = updateModel(t, m, ServiceImportMsg{Event: core.ImportFinished{DownloadID: 1, Files: files}})
	return m
}

func TestImportPopup_FailedFilesScrollOnScreen(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := failedOnScreen(t, newLibraryTestModel(t, nil), size[0], size[1])
			assertBoxEndsAbove(t, m, "[Enter] Close", size[0]*80/100, size[1])
			if line, _ := rangeOnScreen(m); !strings.Contains(line, "[j/k ctrl+d/u g/G] Scroll") {
				t.Errorf("range line %q, want the scroll keys", line)
			}
			first := "- " + trackFile(1) + ": disk full"
			last := "- " + trackFile(30) + ": disk full"
			m = keys(t, m, "G")
			if !strings.Contains(screen(m), last) {
				t.Errorf("G: the last failure is not on screen:\n%s", screen(m))
			}
			m = keys(t, m, "g")
			if !strings.Contains(screen(m), first) {
				t.Errorf("g: the first failure is not on screen:\n%s", screen(m))
			}
		})
	}
}

// With the player bar on screen, the box of each view ends above it.
func TestImportPopup_EndsAboveThePlayerBar(t *testing.T) {
	views := []struct {
		name string
		help string
		open func(t *testing.T, m Model, width, height int) Model
	}{
		{"tag preview", "[Enter] Continue", func(t *testing.T, m Model, width, height int) Model {
			return importOnScreen(t, m, width, height, 30)
		}},
		{"path preview", "[Enter] Start Import", func(t *testing.T, m Model, width, height int) Model {
			return keys(t, importOnScreen(t, m, width, height, 30), keyEnter)
		}},
		{"import", "[Esc] Close", func(t *testing.T, m Model, width, height int) Model {
			return keys(t, importOnScreen(t, m, width, height, 30), keyEnter, keyEnter)
		}},
		{"failed", "[Enter] Close", failedOnScreen},
	}
	for _, v := range views {
		for _, size := range [][2]int{{120, 40}, {80, 24}} {
			t.Run(fmt.Sprintf("%s/%dx%d", v.name, size[0], size[1]), func(t *testing.T) {
				m := playing(t, newLibraryTestModel(t, nil), playerbar.ModeExpanded)
				m = v.open(t, m, size[0], size[1])
				row := m.PlayerBarRow()
				if row == 0 {
					t.Fatal("no player bar")
				}
				assertBoxEndsAbove(t, m, v.help, size[0]*80/100, row-1)
			})
		}
	}
}

// === The retag tag preview fits the screen (issue #316) ===

// coveredLowMB is lowMB with a release that has no cover.
type coveredLowMB struct{ lowMB }

func (coveredLowMB) GetCoverArt(string) ([]byte, error) { return nil, nil }

// retagOnScreen is m resized to width×height, with the retag popup of Low's
// Trust on its tag preview: it returns the fetch of the cover, its result not
// delivered.
func retagOnScreen(t *testing.T, width, height int) (Model, tea.Cmd) {
	t.Helper()
	m := newLibraryTestModel(t, nil)
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	tags := m.Popups.Show(popupctl.Retag, retag.New("low", "trust", coveredLowMB{}, trustTags{}))
	m, details := updateModel(t, m, tags())
	m, cover := updateModel(t, m, details())
	if rt := m.Popups.Retag(); rt == nil || cover == nil {
		t.Fatal("the retag popup is not fetching the cover of its release")
	}
	return m, cover
}

func TestRetagPopup_TagPreviewScrollsOnAShortTerminal(t *testing.T) {
	m, cover := retagOnScreen(t, 80, 24)
	assertBoxEndsAbove(t, m, "Waiting for cover art...", 64, 24)
	m, _ = updateModel(t, m, cover())
	assertBoxEndsAbove(t, m, "[Enter] Retag", 64, 24)
	line, r := rangeOnScreen(m)
	if !strings.Contains(line, "[j/k ctrl+d/u g/G] Scroll") || !strings.HasPrefix(r, "(1-") {
		t.Fatalf("range line %q, want the first tags and the scroll keys:\n%s", line, screen(m))
	}
	m = keys(t, m, "G")
	if !showsTagLine(m, "MB Group ID") {
		t.Errorf("G: the last tag is not on screen:\n%s", screen(m))
	}
	m = keys(t, m, "g")
	if !showsTagLine(m, "Artist") {
		t.Errorf("g: the first tag is not on screen:\n%s", screen(m))
	}
}

func TestRetagPopup_TagPreviewShowsEveryTagOnATallTerminal(t *testing.T) {
	m, cover := retagOnScreen(t, 120, 40)
	m, _ = updateModel(t, m, cover())
	if _, r := rangeOnScreen(m); r != "" {
		t.Errorf("range %q, want every tag", r)
	}
	for _, field := range []string{"Artist", "MB Group ID"} {
		if !showsTagLine(m, field) {
			t.Errorf("%s is not on screen", field)
		}
	}
	assertBoxEndsAbove(t, m, "[Enter] Retag", 96, 40)
}

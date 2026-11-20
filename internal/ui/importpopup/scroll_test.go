package importpopup

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// longPlan is samplePlan with n files, the last of which has no MusicBrainz
// track.
func longPlan(n int) core.ImportPlan {
	plan := samplePlan()
	plan.Files, plan.Release.Tracks = nil, nil
	for i := range n {
		name := fmt.Sprintf("%02d - Track %02d.flac", i+1, i+1)
		if i < n-1 {
			plan.Release.Tracks = append(plan.Release.Tracks, core.MBTrack{Position: i + 1, Title: fmt.Sprintf("Track %02d", i+1)})
		}
		plan.Files = append(plan.Files, core.ImportFile{
			Filename: `@@bob\Test Album\` + name,
			Source:   "/downloads/Test Album/" + name,
			Track:    i,
			Dest:     "Test Artist/2024 • Test Album/" + name,
		})
	}
	plan.Files[n-1].Track, plan.Files[n-1].Dest = -1, ""
	return plan
}

// pathPreview opens the import popup of plan, sized width×height, with
// sources, and takes it to the path preview.
func pathPreview(t *testing.T, plan core.ImportPlan, sources []string, width, height int) (*testutil.PopupHarness, *Model) {
	t.Helper()
	m := New(sampleDownload(), sources, &fakeImporter{plan: plan})
	m.SetSize(width, height)
	h := testutil.NewPopupHarness(m)
	h.ExecuteAndSend(h.LastCommand())
	h.SendEnter()
	if m.state != StatePathPreview {
		t.Fatalf("state = %d, want the path preview", m.state)
	}
	h.ClearCommands()
	return h, m
}

// file is how the path preview names the plan's file n (1-based).
func file(n int) string {
	return fmt.Sprintf("%02d - Track %02d.flac", n, n)
}

var rangePattern = regexp.MustCompile(`\(\d+-\d+ of \d+\)`)

// listed is the range of files the path preview says it lists, "" when it
// says none.
func listed(h *testutil.PopupHarness) string {
	return rangePattern.FindString(testutil.StripANSI(h.View()))
}

// assertListed checks the path preview says it lists files first to last of
// total, and lists them.
func assertListed(t *testing.T, h *testutil.PopupHarness, first, last, total int) {
	t.Helper()
	want := fmt.Sprintf("(%d-%d of %d)", first, last, total)
	if got := listed(h); got != want {
		t.Errorf("range = %q, want %q", got, want)
	}
	view := testutil.StripANSI(h.View())
	if first > 1 && testutil.ContainsLine(view, file(first-1)) {
		t.Errorf("lists %s, before the first file listed", file(first-1))
	}
	for _, n := range []int{first, last} {
		if !testutil.ContainsLine(view, file(n)) {
			t.Errorf("does not list %s", file(n))
		}
	}
	if last < total && testutil.ContainsLine(view, file(last+1)) {
		t.Errorf("lists %s, after the last file listed", file(last+1))
	}
}

func sendTimes(h *testutil.PopupHarness, n int, key tea.KeyType) {
	for range n {
		h.SendSpecialKey(key)
	}
}

// span is the range of files the path preview lists: first to last, 1-based.
type span struct{ first, last int }

// The compact layout (popup 80 wide) lists 6 files at a time, the wide one
// (100 wide) 17, with two library sources: half a page is 3 files and 8.
var layouts = []struct {
	name     string
	width    int
	files    int
	top      span // what the path preview lists first
	halfDown span // after one ctrl+d
	end      span // the last page
	halfUp   span // after one ctrl+u from the last page
}{
	{"compact", 80, 30, span{1, 6}, span{4, 9}, span{25, 30}, span{22, 27}},
	{"wide", 100, 50, span{1, 17}, span{9, 25}, span{34, 50}, span{26, 42}},
}

func TestImport_PathPreviewScrollsByHalfPagesAndStopsAtTheEnds(t *testing.T) {
	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			h, _ := pathPreview(t, longPlan(l.files), sampleLibrarySources(), l.width, 40)
			assertListed(t, h, l.top.first, l.top.last, l.files)

			h.SendSpecialKey(tea.KeyCtrlD)
			assertListed(t, h, l.halfDown.first, l.halfDown.last, l.files)

			sendTimes(h, l.files, tea.KeyCtrlD)
			assertListed(t, h, l.end.first, l.end.last, l.files)
			if err := h.AssertViewContains("(no MusicBrainz track: not imported)"); err != "" {
				t.Error(err)
			}

			h.SendSpecialKey(tea.KeyCtrlU)
			assertListed(t, h, l.halfUp.first, l.halfUp.last, l.files)

			sendTimes(h, l.files, tea.KeyCtrlU)
			assertListed(t, h, l.top.first, l.top.last, l.files)
		})
	}
}

func TestImport_PathPreviewJumpsToTheLastPageAndBack(t *testing.T) {
	jumps := []struct {
		name         string
		toEnd, toTop func(*testutil.PopupHarness)
	}{
		{
			"G and g",
			func(h *testutil.PopupHarness) { h.SendKey("G") },
			func(h *testutil.PopupHarness) { h.SendKey("g") },
		},
		{
			"end and home",
			func(h *testutil.PopupHarness) { h.SendSpecialKey(tea.KeyEnd) },
			func(h *testutil.PopupHarness) { h.SendSpecialKey(tea.KeyHome) },
		},
	}
	for _, l := range layouts {
		for _, j := range jumps {
			t.Run(l.name+"/"+j.name, func(t *testing.T) {
				h, _ := pathPreview(t, longPlan(l.files), sampleLibrarySources(), l.width, 40)

				j.toEnd(h)
				assertListed(t, h, l.end.first, l.end.last, l.files)
				if err := h.AssertViewContains("(no MusicBrainz track: not imported)"); err != "" {
					t.Error(err)
				}
				j.toEnd(h)
				assertListed(t, h, l.end.first, l.end.last, l.files)

				j.toTop(h)
				assertListed(t, h, l.top.first, l.top.last, l.files)
			})
		}
	}
}

// With several library sources, j/k select the library source and leave the
// list where it is.
func TestImport_PathPreviewSelectsTheSourceWithoutScrolling(t *testing.T) {
	h, m := pathPreview(t, longPlan(30), sampleLibrarySources(), 80, 40)
	h.SendSpecialKey(tea.KeyCtrlD)
	assertListed(t, h, 4, 9, 30)

	h.SendKey("j")
	h.SendSpecialKey(tea.KeyDown) // wraps around
	h.SendSpecialKey(tea.KeyDown)
	if m.selectedSource != 1 {
		t.Errorf("selected source %d, want 1", m.selectedSource)
	}
	assertView(t, h, "> /music/library2")
	assertListed(t, h, 4, 9, 30)

	h.SendKey("k")
	h.SendSpecialKey(tea.KeyUp)
	assertView(t, h, "> /music/library2")
	assertListed(t, h, 4, 9, 30)
}

// With one library source or none, j/k have no source to select: they scroll
// by one file.
func TestImport_PathPreviewWithOneOrNoSourceScrollsByOneFile(t *testing.T) {
	for _, c := range []struct {
		name    string
		sources []string
		page    int
	}{
		{"one source", []string{"/music/only"}, 19},
		{"no source", nil, 17},
	} {
		t.Run(c.name, func(t *testing.T) {
			h, _ := pathPreview(t, longPlan(30), c.sources, 100, 40)
			assertListed(t, h, 1, c.page, 30)

			h.SendKey("k") // already at the top
			h.SendSpecialKey(tea.KeyUp)
			assertListed(t, h, 1, c.page, 30)

			h.SendKey("j")
			assertListed(t, h, 2, c.page+1, 30)
			h.SendSpecialKey(tea.KeyDown)
			assertListed(t, h, 3, c.page+2, 30)
			h.SendKey("k")
			assertListed(t, h, 2, c.page+1, 30)
			h.SendSpecialKey(tea.KeyUp)
			assertListed(t, h, 1, c.page, 30)

			h.SendKey("G")
			h.SendKey("j") // already at the end
			h.SendSpecialKey(tea.KeyDown)
			assertListed(t, h, 31-c.page, 30, 30)
		})
	}
}

// A list scrolled to its end, then given more room, shows a full page.
func TestImport_PathPreviewScrolledToTheEndShowsAFullPageOnceTaller(t *testing.T) {
	h, _ := pathPreview(t, longPlan(30), sampleLibrarySources(), 80, 26)
	h.SendKey("G")
	assertListed(t, h, 30, 30, 30)

	h.SetSize(80, 40)
	assertListed(t, h, 25, 30, 30)

	h.SendSpecialKey(tea.KeyCtrlU)
	assertListed(t, h, 22, 27, 30)
}

func TestImport_PathPreviewNamesTheScrollKeysWhenTheListDoesNotFit(t *testing.T) {
	t.Run("several sources", func(t *testing.T) {
		h, _ := pathPreview(t, longPlan(30), sampleLibrarySources(), 80, 40)
		view := testutil.StripANSI(h.View())
		line := testutil.FindLine(view, "(1-6 of 30)")
		if !strings.Contains(line, "Scroll") || !strings.Contains(line, "ctrl+d") {
			t.Errorf("range line %q names no scroll key", line)
		}
		if strings.Contains(line, "j/k") {
			t.Errorf("range line %q offers j/k, which select the library source", line)
		}
		assertView(t, h, "[j/k] Select Library")
	})
	t.Run("one source", func(t *testing.T) {
		h, _ := pathPreview(t, longPlan(30), []string{"/music/only"}, 80, 40)
		line := testutil.FindLine(testutil.StripANSI(h.View()), " of 30)")
		if !strings.Contains(line, "j/k") || !strings.Contains(line, "Scroll") {
			t.Errorf("range line %q does not offer j/k to scroll", line)
		}
	})
}

// The text that names the scroll keys fits a popup 64 wide, whose inner width
// is 56.
func TestImport_PathPreviewScrollKeysFitANarrowPopup(t *testing.T) {
	for _, sources := range [][]string{sampleLibrarySources(), {"/music/only"}, nil} {
		h, _ := pathPreview(t, longPlan(300), sources, 64, 40)
		h.SendKey("G")
		line := testutil.FindLine(testutil.StripANSI(h.View()), " of 300)")
		if !strings.Contains(line, "Scroll") {
			t.Fatalf("range line %q with %d sources names no scroll key", line, len(sources))
		}
		if w := testutil.MeasureWidth(line); w > 56 {
			t.Errorf("range line %q is %d columns wide, want at most 56", line, w)
		}
	}
}

// When every file fits, nothing scrolls and nothing says it could.
func TestImport_PathPreviewThatFitsDoesNotScroll(t *testing.T) {
	for _, sources := range [][]string{sampleLibrarySources(), {"/music/only"}} {
		h, _ := pathPreview(t, samplePlan(), sources, 100, 40)
		for _, key := range []tea.KeyType{tea.KeyCtrlD, tea.KeyEnd, tea.KeyDown} {
			h.SendSpecialKey(key)
		}
		h.SendKey("G")
		h.SendKey("j")

		if got := listed(h); got != "" {
			t.Errorf("range %q for a list that fits", got)
		}
		if err := h.AssertViewNotContains("Scroll"); err != "" {
			t.Error(err)
		}
		assertView(t, h, "01 - Track One.flac", "03 - Track Three.flac")
	}
}

// sendScrollKeys sends every key that scrolls a list of the popup.
func sendScrollKeys(h *testutil.PopupHarness) {
	for _, key := range []string{"j", "k", "g", "G"} {
		h.SendKey(key)
	}
	for _, key := range []tea.KeyType{tea.KeyCtrlD, tea.KeyCtrlU, tea.KeyHome, tea.KeyEnd, tea.KeyDown, tea.KeyUp} {
		h.SendSpecialKey(key)
	}
}

// Where every item fits, and in the progress list that follows the import,
// the scroll keys change nothing, and never start or close the import.
func TestImport_ScrollKeysChangeNothingWhereNothingScrolls(t *testing.T) {
	m := New(sampleDownload(), []string{"/music/only"}, &fakeImporter{plan: longPlan(30)})
	m.SetSize(100, 40)
	h := testutil.NewPopupHarness(m)
	h.ExecuteAndSend(h.LastCommand())
	h.ClearCommands()
	imp, _ := m.importer.(*fakeImporter)

	check := func(state State) {
		t.Helper()
		before, imports := h.View(), len(imp.imported)
		sendScrollKeys(h)
		if m.state != state || len(h.Commands()) != 0 || len(imp.imported) != imports {
			t.Errorf("scroll keys in state %d: state %d, %d commands, imported %v",
				state, m.state, len(h.Commands()), imp.imported)
		}
		if after := h.View(); after != before {
			t.Errorf("scroll keys changed the view in state %d", state)
		}
		h.ClearCommands()
	}

	check(StateTagPreview)
	h.SendEnter()
	h.SendEnter()
	h.ClearCommands()
	check(StateImporting)
	h.SendMsg(core.ImportFinished{DownloadID: 1})
	check(StateComplete)
}

// In a popup too short to list a file, the scroll keys are harmless.
func TestImport_ScrollKeysAreHarmlessInAShortPopup(t *testing.T) {
	for _, sources := range [][]string{sampleLibrarySources(), {"/music/only"}, nil} {
		h, m := pathPreview(t, longPlan(30), sources, 80, 24)
		if p := testutil.PanicOf(func() { sendScrollKeys(h); h.View() }); p != nil {
			t.Fatalf("panicked with %d sources: %v", len(sources), p)
		}
		if m.state != StatePathPreview || len(h.Commands()) != 0 {
			t.Errorf("state %d, %d commands", m.state, len(h.Commands()))
		}
	}
}

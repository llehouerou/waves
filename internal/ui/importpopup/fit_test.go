package importpopup

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// boxRows is how many rows the box drawn around the popup's view takes.
func boxRows(h *testutil.PopupHarness, m *Model) int {
	return testutil.BoxRows(h.View(), m.Width(), m.Height())
}

// shown is the range the view says its list shows, first and last 1-based,
// and whether it says one.
func shown(t *testing.T, h *testutil.PopupHarness) (first, last, total int, ok bool) {
	t.Helper()
	r := listed(h)
	if r == "" {
		return 0, 0, 0, false
	}
	if _, err := fmt.Sscanf(r, "(%d-%d of %d)", &first, &last, &total); err != nil {
		t.Fatalf("range %q: %v", r, err)
	}
	return first, last, total, true
}

// rangeLine is the line of the view that holds its range.
func rangeLine(h *testutil.PopupHarness) string {
	view := testutil.StripANSI(h.View())
	return testutil.FindLine(view, listed(h))
}

// tagPreview opens the import popup of a 30-file plan with one library
// source, sized width×height, on its tag preview.
func tagPreview(t *testing.T, width, height int) (*testutil.PopupHarness, *Model) {
	t.Helper()
	m := New(sampleDownload(), []string{"/music/only"}, &fakeImporter{plan: longPlan(30)})
	m.SetSize(width, height)
	h := testutil.NewPopupHarness(m)
	h.ExecuteAndSend(h.LastCommand())
	if m.state != StateTagPreview || !m.planned() {
		t.Fatalf("state %d, planned %v: want the tag preview", m.state, m.planned())
	}
	h.ClearCommands()
	return h, m
}

// importing is tagPreview taken to the import the core started.
func importing(t *testing.T, width, height int) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m := tagPreview(t, width, height)
	h.SendEnter()
	h.SendEnter()
	if m.state != StateImporting {
		t.Fatalf("state %d, want the import", m.state)
	}
	h.ClearCommands()
	return h, m
}

// fileStates is where the import of longPlan(len(states)) is.
func fileStates(states ...core.FileState) []core.FileProgress {
	files := make([]core.FileProgress, len(states))
	for i, state := range states {
		files[i] = core.FileProgress{Path: "/downloads/Test Album/" + file(i+1), State: state}
		if state == core.FileFailed {
			files[i].Err = errors.New("disk full")
		}
	}
	return files
}

// every is the state of each of 30 files.
func every(state core.FileState) []core.FileState {
	states := make([]core.FileState, 30)
	for i := range states {
		states[i] = state
	}
	return states
}

// failedImport is importing once the core said all 30 files failed.
func failedImport(t *testing.T, width, height int) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m := importing(t, width, height)
	h.SendMsg(core.ImportFinished{DownloadID: 1, Files: fileStates(every(core.FileFailed)...)})
	if m.state != StateComplete {
		t.Fatalf("state %d, want the import complete", m.state)
	}
	return h, m
}

// failure is how the complete view names the failed file n.
func failure(n int) string {
	return "- " + file(n) + ": disk full"
}

// showsTag reports whether the tag preview lists the tag named field.
func showsTag(h *testutil.PopupHarness, field string) bool {
	for line := range strings.SplitSeq(testutil.StripANSI(h.View()), "\n") {
		if strings.HasPrefix(line, field+"  ") {
			return true
		}
	}
	return false
}

const tagKeys = "[j/k ctrl+d/u g/G] Scroll"

// A tag preview taller than its popup scrolls its tags, and keeps its help
// line.
func TestImport_TagPreviewScrollsItsTagsInAShortPopup(t *testing.T) {
	h, m := tagPreview(t, 80, 24) // 20 rows for 34 lines
	assertView(t, h, "[Enter] Continue")
	if !showsTag(h, "Artist") {
		t.Error("the Artist tag is not shown")
	}
	if rows := boxRows(h, m); rows > 24 {
		t.Errorf("box of %d rows, want at most 24", rows)
	}
	first, last, total, ok := shown(t, h)
	if !ok || first != 1 || last < first || total != 18 {
		t.Fatalf("range %d-%d of %d (%v), want the first tags of 18", first, last, total, ok)
	}
	if line := rangeLine(h); !strings.Contains(line, tagKeys) {
		t.Errorf("range line %q does not name the scroll keys", line)
	}

	h.SendKey("j")
	if f, _, _, _ := shown(t, h); f != 2 {
		t.Errorf("j: from tag %d, want 2", f)
	}
	h.SendKey("k")
	if f, _, _, _ := shown(t, h); f != 1 {
		t.Errorf("k: from tag %d, want 1", f)
	}
	h.SendSpecialKey(tea.KeyCtrlD)
	if f, _, _, _ := shown(t, h); f <= 1 {
		t.Errorf("ctrl+d: from tag %d, want further", f)
	}
	h.SendSpecialKey(tea.KeyCtrlU)
	if f, _, _, _ := shown(t, h); f != 1 {
		t.Errorf("ctrl+u: from tag %d, want 1", f)
	}

	h.SendKey("G")
	assertView(t, h, "MB Release ID", "[Enter] Continue")
	if _, l, _, _ := shown(t, h); l != 18 {
		t.Errorf("G: up to tag %d, want 18", l)
	}
	h.SendKey("g")
	if !showsTag(h, "Artist") {
		t.Error("g: the Artist tag is not shown")
	}
	if len(h.Commands()) != 0 || m.state != StateTagPreview {
		t.Errorf("scrolling: state %d, %d commands", m.state, len(h.Commands()))
	}
}

// A popup told it may grow takes the rows it needs up to its limit, and no
// more; one told nothing stays at its height.
func TestImport_GrowsUpToItsLimitAndNoFurther(t *testing.T) {
	h, m := tagPreview(t, 100, 20) // 34 lines for 16 rows
	if rows := boxRows(h, m); rows != 20 {
		t.Errorf("no limit: box of %d rows, want 20", rows)
	}
	m.SetMaxHeight(30)
	if rows := boxRows(h, m); rows != 30 {
		t.Errorf("limit 30: box of %d rows, want 30", rows)
	}
	m.SetMaxHeight(60)
	if rows := boxRows(h, m); rows != 38 {
		t.Errorf("limit 60: box of %d rows, want 38: the 34 lines of the view", rows)
	}
	if r := listed(h); r != "" {
		t.Errorf("limit 60: range %q for a view that fits", r)
	}
	m.SetMaxHeight(10)
	if rows := boxRows(h, m); rows != 20 {
		t.Errorf("limit 10: box of %d rows, want the popup's 20", rows)
	}
}

// In a short popup the path preview lists a file at least, and fits.
func TestImport_PathPreviewListsAFileInAShortPopup(t *testing.T) {
	for _, size := range [][2]int{{80, 16}, {64, 16}, {80, 12}} {
		for _, sources := range [][]string{sampleLibrarySources(), {"/music/only"}, nil} {
			name := fmt.Sprintf("%dx%d/%d sources", size[0], size[1], len(sources))
			t.Run(name, func(t *testing.T) {
				h, m := pathPreview(t, longPlan(30), sources, size[0], size[1])
				first, last, _, ok := shown(t, h)
				if !ok || first < 1 || last < first {
					t.Fatalf("range %q, want a file at least", listed(h))
				}
				assertView(t, h, file(first), m.pathPreviewHelpText())
				if rows := boxRows(h, m); rows > size[1] {
					t.Errorf("box of %d rows, want at most %d:\n%s", rows, size[1], testutil.StripANSI(h.View()))
				}
			})
		}
	}
}

// The failed files of a finished import scroll, and a reason wider than the
// popup counts for the rows it takes.
func TestImport_CompleteViewScrollsItsFailuresInItsPopup(t *testing.T) {
	h, m := failedImport(t, 100, 40)
	assertView(t, h, "[Enter] Close", failure(1))
	if rows := boxRows(h, m); rows > 40 {
		t.Errorf("box of %d rows, want at most 40", rows)
	}
	if line := rangeLine(h); !strings.Contains(line, tagKeys) {
		t.Errorf("range line %q does not name the scroll keys", line)
	}
	h.SendKey("G")
	assertView(t, h, failure(30), "[Enter] Close")
	h.SendKey("g")
	assertView(t, h, failure(1))

	files := fileStates(every(core.FileFailed)...)
	for i := range files {
		files[i].Err = errors.New(strings.Repeat("the disk is full ", 8)) // 136 columns: 2 rows
	}
	h, m = importing(t, 100, 40)
	h.SendMsg(core.ImportFinished{DownloadID: 1, Files: files})
	if rows := boxRows(h, m); rows > 40 {
		t.Errorf("long reasons: box of %d rows, want at most 40", rows)
	}
	h.SendKey("G")
	assertView(t, h, "[Enter] Close", file(30))
	if rows := boxRows(h, m); rows > 40 {
		t.Errorf("long reasons, last page: box of %d rows, want at most 40", rows)
	}
}

// The progress list follows the import: the file being imported is shown,
// with a range that names no key.
func TestImport_ProgressListFollowsTheImport(t *testing.T) {
	for _, size := range [][2]int{{100, 24}, {64, 16}} {
		h, m := importing(t, size[0], size[1])
		assertView(t, h, "Progress:", "[Esc] Close", file(1))
		if r := listed(h); r == "" || strings.Contains(rangeLine(h), "Scroll") {
			t.Errorf("%v: range line %q, want one naming no key", size, rangeLine(h))
		}

		states := every(core.FilePending)
		for i := range 19 {
			states[i] = core.FileDone
		}
		states[19] = core.FileWorking
		h.SendMsg(core.ImportProgressed{DownloadID: 1, Files: fileStates(states...)})
		assertView(t, h, file(20), "Progress: 19/30")

		states[19] = core.FileDone
		h.SendMsg(core.ImportProgressed{DownloadID: 1, Files: fileStates(states...)})
		assertView(t, h, file(21))

		h.SendMsg(core.ImportProgressed{DownloadID: 1, Files: fileStates(every(core.FileDone)...)})
		assertView(t, h, file(30), "[Esc] Close")
		if rows := boxRows(h, m); rows > size[1] {
			t.Errorf("%v: box of %d rows", size, rows)
		}
	}
}

// A list scrolled to its end, then given more rows, shows a full last page.
func TestImport_ListsScrolledToTheEndShowAFullPageOnceTaller(t *testing.T) {
	lists := []struct {
		name string
		open func(t *testing.T, width, height int) (*testutil.PopupHarness, *Model)
	}{
		{"tag preview", tagPreview},
		{"failed files", failedImport},
	}
	for _, l := range lists {
		t.Run(l.name, func(t *testing.T) {
			h, m := l.open(t, 100, 20)
			h.SendKey("G")
			m.SetSize(100, 30)
			first, last, total, ok := shown(t, h)
			if !ok || last != total {
				t.Errorf("up to %d of %d, want the last", last, total)
			}
			h.SendKey("g")
			topFirst, topLast, _, ok := shown(t, h)
			if !ok || last-first != topLast-topFirst {
				t.Errorf("last page %d-%d, a page %d-%d: want a full page", first, last, topFirst, topLast)
			}
		})
	}
}

// helpLines are the views of the popup, and the help line each always
// shows: none while the plan is prepared.
var helpLines = []struct {
	name string
	open func(t *testing.T) (*testutil.PopupHarness, *Model)
	help string
}{
	{"preparing, no help line", func(t *testing.T) (*testutil.PopupHarness, *Model) {
		m := New(sampleDownload(), nil, &fakeImporter{plan: longPlan(30)})
		m.SetSize(100, 40)
		h := testutil.NewPopupHarness(m)
		h.ClearCommands() // the plan asked, not answered
		return h, m
	}, ""},
	{"plan refused", func(t *testing.T) (*testutil.PopupHarness, *Model) {
		m := New(sampleDownload(), nil, &fakeImporter{prepareErr: errors.New("no files")})
		m.SetSize(100, 40)
		h := testutil.NewPopupHarness(m)
		h.ExecuteAndSend(h.LastCommand())
		h.ClearCommands()
		return h, m
	}, "[Esc] Close"},
	{"tag preview", func(t *testing.T) (*testutil.PopupHarness, *Model) { return tagPreview(t, 100, 40) }, "[Enter] Continue"},
	{"path preview, several sources", func(t *testing.T) (*testutil.PopupHarness, *Model) {
		return pathPreview(t, longPlan(30), sampleLibrarySources(), 100, 40)
	}, "[Enter] Start Import   [j/k] Select Library"},
	{"path preview, one source", func(t *testing.T) (*testutil.PopupHarness, *Model) {
		return pathPreview(t, longPlan(30), []string{"/music/only"}, 100, 40)
	}, "[Enter] Start Import   [Esc] Back"},
	{"path preview, no source", func(t *testing.T) (*testutil.PopupHarness, *Model) {
		return pathPreview(t, longPlan(30), nil, 100, 40)
	}, "[Esc] Back"},
	{"importing", func(t *testing.T) (*testutil.PopupHarness, *Model) { return importing(t, 100, 40) }, "[Esc] Close (the import continues)"},
	{"failed", func(t *testing.T) (*testutil.PopupHarness, *Model) { return failedImport(t, 100, 40) }, "[Enter] Close"},
	{"imported", func(t *testing.T) (*testutil.PopupHarness, *Model) {
		h, m := importing(t, 100, 40)
		h.SendMsg(core.ImportFinished{DownloadID: 1, Files: fileStates(every(core.FileDone)...)})
		return h, m
	}, "[Enter] Close"},
}

// At any height the scroll keys are harmless, and the help line is shown.
func TestImport_EveryViewKeepsItsHelpLineAtAnyHeight(t *testing.T) {
	for _, v := range helpLines {
		t.Run(v.name, func(t *testing.T) {
			for _, width := range []int{64, 100} {
				for height := 1; height <= 40; height++ {
					h, m := v.open(t)
					m.SetSize(width, height)
					state := m.state
					if p := testutil.PanicOf(func() { sendScrollKeys(h); h.View() }); p != nil {
						t.Fatalf("%dx%d: panicked: %v", width, height, p)
					}
					if m.state != state || len(h.Commands()) != 0 {
						t.Errorf("%dx%d: state %d, %d commands", width, height, m.state, len(h.Commands()))
					}
					if v.help != "" && !h.ViewContains(v.help) {
						t.Errorf("%dx%d: no help line %q:\n%s", width, height, v.help, testutil.StripANSI(h.View()))
					}
					if f, l, _, ok := shown(t, h); ok && (f < 1 || l < f) {
						t.Errorf("%dx%d: range %d-%d", width, height, f, l)
					}
				}
			}
		})
	}
}

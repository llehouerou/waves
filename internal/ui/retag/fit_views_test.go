package retag

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// === Every view in its box (issue #404) ===

// terminals are the sizes the views are checked at: the popup manager sizes
// the popup for each, and lets it grow to its rows.
var terminals = [][2]int{{80, 24}, {100, 16}, {120, 40}}

// anyRange matches the range a list shows under its page.
var anyRange = regexp.MustCompile(`\((\d+)-(\d+) of (\d+)\)`)

// fileName is the name of the file n of thirtyFiles.
func fileName(n int) string {
	return fmt.Sprintf("track %02d.mp3", n)
}

// thirtyFiles are 30 files of the album, each in state.
func thirtyFiles(state core.FileState) []core.FileProgress {
	files := make([]core.FileProgress, 30)
	for i := range files {
		files[i] = core.FileProgress{Path: "/music/low/trust/" + fileName(i+1), State: state}
		if state == core.FileFailed {
			files[i].Err = errors.New("read-only file")
		}
	}
	return files
}

// popupIn is the retag popup of "trust" by "low" at state, sized for a
// terminal of width×height.
func popupIn(state State, width, height int) (*testutil.PopupHarness, *Model) {
	m := New("low", "trust", fakeMB{}, &fakeLibrary{})
	m.state = state
	sizeFor(m, width, height)
	h := testutil.NewPopupHarness(m)
	h.ClearCommands() // the read of the tags, which these views have
	return h, m
}

// groupList is the popup on a list of 30 release groups.
func groupList(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateReleaseGroupResults, width, height)
	for i := range 30 {
		m.foundGroups = append(m.foundGroups, group(fmt.Sprint("g", i), "Low", fmt.Sprintf("Album %02d", i+1), "2001", "Album"))
	}
	m.listReleaseGroups()
	return h, m
}

// releaseList is the popup on a list of 30 releases.
func releaseList(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateReleaseResults, width, height)
	m.listedGroupTitle = "Trust"
	for i := range 30 {
		m.releases = append(m.releases, core.MBRelease{
			ID: fmt.Sprint("r", i), Title: fmt.Sprintf("Release %02d", i+1),
			TrackCount: 3, Date: "2002", Country: "US", Formats: "CD",
		})
	}
	return h, m
}

// retagging is the popup following the retag of 30 files.
func retagging(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateRetagging, width, height)
	m.files = thirtyFiles(core.FilePending)
	return h, m
}

// failedRetag is the popup on the end of a retag whose 30 files failed.
func failedRetag(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := retagging(width, height)
	h.SendMsg(core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: thirtyFiles(core.FileFailed)})
	return h, m
}

// popupModel is the retag popup of h.
func popupModel(t *testing.T, h *testutil.PopupHarness) *Model {
	t.Helper()
	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatalf("the popup is a %T", h.Popup())
	}
	return m
}

// backwards is the range of view whose first item comes after its last, ""
// when there is none.
func backwards(view string) string {
	var first, last, total int
	r := anyRange.FindString(view)
	if _, err := fmt.Sscanf(r, "(%d-%d of %d)", &first, &last, &total); err != nil || first <= last {
		return ""
	}
	return r
}

// view is the view of the popup, ANSI stripped.
func view(h *testutil.PopupHarness) string {
	return testutil.StripANSI(h.View())
}

// rangeOf is the range the view says its list shows, "" for none.
func rangeOf(h *testutil.PopupHarness) string {
	return anyRange.FindString(view(h))
}

// assertInBox checks that the view holds help on one row of the text area,
// and that its box is no taller than the terminal of terminalHeight rows.
func assertInBox(t *testing.T, h *testutil.PopupHarness, m *Model, help string, terminalHeight int) {
	t.Helper()
	line := testutil.FindLine(view(h), help)
	if line == "" {
		t.Fatalf("no %q in:\n%s", help, view(h))
	}
	if w := lipgloss.Width(line); w > popup.TextWidth(m.Width()) {
		t.Errorf("help line %q is %d columns, the text area %d", line, w, popup.TextWidth(m.Width()))
	}
	if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows > terminalHeight {
		t.Errorf("box of %d rows in a terminal of %d", rows, terminalHeight)
	}
}

func TestRetag_CompleteViewScrollsItsFailedFiles(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			h, m := failedRetag(size[0], size[1])
			if m.state != StateComplete {
				t.Fatalf("state %d, want the complete view", m.state)
			}
			assertInBox(t, h, m, "[Enter] Close", size[1])
			assertInBox(t, h, m, "30 files failed", size[1])
			r := rangeOf(h)
			if !strings.HasPrefix(r, "(1-") || !strings.Contains(testutil.FindLine(view(h), r), "[j/k ctrl+d/u g/G] Scroll") {
				t.Fatalf("range %q, want the first failed files and the scroll keys", r)
			}

			h.SendKey("G")
			assertView(t, h, fileName(30)+": read-only file")
			h.SendKey("g")
			assertView(t, h, fileName(1)+": read-only file")
			for _, key := range []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune("j")}, {Type: tea.KeyCtrlD},
				{Type: tea.KeyRunes, Runes: []rune("k")}, {Type: tea.KeyCtrlU},
			} {
				before := rangeOf(h)
				h.SendMsg(key)
				if rangeOf(h) == before {
					t.Errorf("%s left the range at %s", key, before)
				}
			}
			if len(h.Commands()) != 0 || m.state != StateComplete {
				t.Fatalf("scrolling: state %d, %d commands", m.state, len(h.Commands()))
			}
			assertInBox(t, h, m, "[Enter] Close", size[1])
			h.SendEnter()
			if _, ok := lastAction(t, h).(Complete); !ok {
				t.Error("Enter does not complete")
			}
		})
	}
}

func TestRetag_CompleteViewInATallTerminalListsEveryFailedFile(t *testing.T) {
	h, _ := failedRetag(200, 60)
	if r := rangeOf(h); r != "" {
		t.Errorf("range %q, want every failed file", r)
	}
	for n := 1; n <= 30; n++ {
		assertView(t, h, fileName(n))
	}
}

// A failure too long for a row keeps its whole reason.
func TestRetag_CompleteViewKeepsALongReasonWhole(t *testing.T) {
	h, m := retagging(80, 24)
	files := thirtyFiles(core.FileFailed)
	files[29].Err = errors.New(strings.Repeat("disk ", 30) + "full")
	h.SendMsg(core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: files})
	h.SendKey("G")
	assertInBox(t, h, m, "[Enter] Close", 24)
	if !strings.Contains(strings.Join(strings.Fields(view(h)), " "), strings.Repeat("disk ", 30)+"full") {
		t.Errorf("the last reason is not whole:\n%s", view(h))
	}
}

func TestRetag_ProgressViewFollowsTheRetag(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 16}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			h, m := retagging(size[0], size[1])
			assertInBox(t, h, m, "[Esc] Close", size[1])
			assertInBox(t, h, m, "Progress:", size[1])
			r := rangeOf(h)
			if r == "" || strings.Contains(testutil.FindLine(view(h), r), "Scroll") {
				t.Errorf("range %q, want one that names no key", testutil.FindLine(view(h), r))
			}
			if strings.Contains(view(h), "more") {
				t.Errorf("the view says how many more:\n%s", view(h))
			}

			files := thirtyFiles(core.FileDone)
			files[19].State = core.FileWorking
			for i := 20; i < 30; i++ {
				files[i].State = core.FilePending
			}
			h.SendMsg(core.RetagProgressed{AlbumArtist: "low", Album: "trust", Files: files})
			assertView(t, h, fileName(20))

			h.SendMsg(core.RetagProgressed{AlbumArtist: "low", Album: "trust", Files: thirtyFiles(core.FileDone)})
			assertView(t, h, fileName(30))
			assertInBox(t, h, m, "[Esc] Close", size[1])
		})
	}
}

// cursorRow is the row of the list under the cursor, "" when none is shown.
func cursorRow(h *testutil.PopupHarness) string {
	for _, line := range testutil.SplitLines(view(h)) {
		if strings.HasPrefix(line, "> ") {
			return line
		}
	}
	return ""
}

func TestRetag_ListsKeepTheirCursorAndHelpLineInTheBox(t *testing.T) {
	lists := []struct {
		name string
		open func(width, height int) (*testutil.PopupHarness, *Model)
		row  string
		help string
	}{
		{"release groups", groupList, "Album %02d", "[a] Albums only"},
		{"releases", releaseList, "Release %02d", "[Backspace] Back"},
	}
	for _, list := range lists {
		for _, size := range terminals {
			t.Run(fmt.Sprint(list.name, size), func(t *testing.T) {
				h, m := list.open(size[0], size[1])
				check := func(n int) {
					t.Helper()
					assertInBox(t, h, m, list.help, size[1])
					if row := cursorRow(h); !strings.Contains(row, fmt.Sprintf(list.row, n)) {
						t.Fatalf("cursor on %d, the cursor row %q", n, row)
					}
				}
				check(1)
				for n := 2; n <= 30; n++ {
					h.SendKey("j")
					check(n)
				}
				for n := 29; n >= 1; n-- {
					h.SendKey("k")
					check(n)
				}
			})
		}
	}
}

// A row longer than the text area is cut to it, its title giving way to
// what follows it.
func TestRetag_ListRowsAreCutToTheTextArea(t *testing.T) {
	long := strings.Repeat("Things We Lost ", 6)
	h, m := groupList(80, 24)
	m.foundGroups[0] = group("long", "Low", long, "2001", "Album", "Live")
	m.listReleaseGroups()
	h.SendKey("j")
	for _, n := range []int{0, 1} {
		row := testutil.FindLine(view(h), "Things We Lost")
		if w := lipgloss.Width(row); w > popup.TextWidth(m.Width()) || !strings.Contains(row, "(2001)") {
			t.Errorf("cursor at %d: row %q of %d columns, want at most %d and the year", n, row, w, popup.TextWidth(m.Width()))
		}
		h.SendKey("k")
	}

	h, m = releaseList(80, 24)
	m.releases[0].Title = long
	row := testutil.FindLine(view(h), "Things We Lost")
	if w := lipgloss.Width(row); w > popup.TextWidth(m.Width()) || !strings.Contains(row, "(2002)") {
		t.Errorf("release row %q of %d columns, want at most %d and the year", row, w, popup.TextWidth(m.Width()))
	}
}

func TestRetag_GroupListHelpFitsANarrowTerminalWithTheFilter(t *testing.T) {
	h, m := groupList(80, 24)
	assertInBox(t, h, m, "Albums only: off", 24)
	h.SendKey("a")
	assertInBox(t, h, m, "Albums only: on", 24)
}

// The scroll keys of the tag preview do nothing in the lists and while
// retagging: they start, scroll and close nothing.
func TestRetag_ScrollKeysDoNothingInTheListsAndTheProgressView(t *testing.T) {
	for _, open := range []func(width, height int) (*testutil.PopupHarness, *Model){groupList, releaseList, retagging} {
		h, m := open(80, 24)
		h.SendKey("j")
		before, state := view(h), m.state
		for _, key := range []tea.KeyType{tea.KeyCtrlD, tea.KeyCtrlU, tea.KeyHome, tea.KeyEnd} {
			h.SendSpecialKey(key)
		}
		h.SendKey("g")
		h.SendKey("G")
		if view(h) != before || m.state != state || len(h.Commands()) != 0 {
			t.Errorf("state %d: the scroll keys changed the view:\n%s\nwant:\n%s", state, view(h), before)
		}
	}
}

// helpOf is a text the help line of the popup's state holds, in lower case:
// the shortest help lines name their keys so.
func helpOf(m *Model) string {
	if m.state == StateComplete {
		return "[enter] close"
	}
	return "esc"
}

// everyState is the popup in each of its states, with long lists.
func everyState(width, height int) map[string]*testutil.PopupHarness {
	states := map[string]*testutil.PopupHarness{}
	for name, s := range map[string]State{"loading": StateLoading, "searching": StateSearching, "release loading": StateReleaseLoading, "details loading": StateReleaseDetailsLoading} {
		h, m := popupIn(s, width, height)
		m.statusMsg = "Searching MusicBrainz..."
		states[name] = h
	}
	h, m := popupIn(StateLoading, width, height)
	m.errorMsg = "unavailable"
	states["error"] = h
	states["groups"], _ = groupList(width, height)
	h, m = groupList(width, height)
	m.searchMode = true
	m.errorMsg = "unavailable"
	states["groups searching"] = h
	states["no groups"], _ = popupIn(StateReleaseGroupResults, width, height)
	states["releases"], _ = releaseList(width, height)
	states["no releases"], _ = popupIn(StateReleaseResults, width, height)
	states["retagging"], _ = retagging(width, height)
	states["complete"], _ = failedRetag(width, height)
	h, _ = retagging(width, height)
	h.SendMsg(core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: thirtyFiles(core.FileDone), Err: errors.New("locked")})
	states["library failed"] = h
	return states
}

// No view panics at any size, and each keeps its help line.
func TestRetag_EveryViewKeepsItsHelpLineAtAnyHeight(t *testing.T) {
	for _, width := range []int{64, 100} {
		for height := 1; height <= 40; height++ {
			for name, h := range everyState(width, height) {
				m := popupModel(t, h)
				for _, key := range []string{"j", "k", "g", "G", "ctrl+d", "ctrl+u"} {
					if r := testutil.PanicOf(func() {
						if key == "ctrl+d" || key == "ctrl+u" {
							h.SendMsg(tea.KeyMsg{Type: map[string]tea.KeyType{"ctrl+d": tea.KeyCtrlD, "ctrl+u": tea.KeyCtrlU}[key]})
						} else {
							h.SendKey(key)
						}
						h.View()
					}); r != nil {
						t.Fatalf("%s at %dx%d, %s: panics: %v", name, width, height, key, r)
					}
					// A popup sized no rows draws nothing.
					if m.Height() > 0 && !strings.Contains(strings.ToLower(view(h)), helpOf(m)) {
						t.Fatalf("%s at %dx%d, %s: no help line in:\n%s", name, width, height, key, view(h))
					}
					if r := backwards(view(h)); r != "" {
						t.Fatalf("%s at %dx%d: range %s reads backwards", name, width, height, r)
					}
				}
			}
		}
	}
}

// A popup told no limit stays in its own box; one told a taller limit grows
// up to it and no further.
func TestRetag_ViewsGrowUpToTheirLimitAndNoFurther(t *testing.T) {
	for name, h := range everyState(80, 24) {
		m := popupModel(t, h)
		m.SetMaxHeight(0)
		if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows > m.Height() {
			t.Errorf("%s with no limit: a box of %d rows, want at most %d", name, rows, m.Height())
		}
		m.SetMaxHeight(m.Height() + 6)
		if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows > m.Height()+6 {
			t.Errorf("%s up to %d rows: a box of %d rows", name, m.Height()+6, rows)
		}
	}
	h, m := groupList(80, 24)
	m.SetMaxHeight(m.Height() + 6)
	if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows != m.Height()+6 {
		t.Errorf("a long list up to %d rows: a box of %d rows, want %d", m.Height()+6, rows, m.Height()+6)
	}
}

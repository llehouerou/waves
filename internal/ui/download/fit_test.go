package download

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// === Every view in its box (issue #404) ===

// sizeFor sizes the popup as the popup manager does for a terminal of
// width×height, and lets it grow to the terminal's rows.
func sizeFor(m *Model, width, height int) {
	m.SetSize(width*80/100, height*70/100)
	m.SetMaxHeight(height)
}

// anyRange matches the range a list shows under its page.
var anyRange = regexp.MustCompile(`\((\d+)-(\d+) of (\d+)\)`)

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

// popupIn is a download popup at state, sized for a terminal of
// width×height, the artist and the release picked.
func popupIn(state State, width, height int) (*testutil.PopupHarness, *Model) {
	m := New(&fakeDownloads{}, fakeMB{}, FilterConfig{}, nil)
	m.state = state
	m.selectedArtist = &core.MBArtist{Name: "Low"}
	if state >= StateReleaseLoading {
		m.selectedReleaseGroup = &core.MBReleaseGroup{Title: "Trust", FirstRelease: "2002"}
	}
	sizeFor(m, width, height)
	return testutil.NewPopupHarness(m), m
}

// lists are the lists of the popup the user moves in, each of 30 rows:
// open returns the popup on it, row names the row n.
var lists = []struct {
	name string
	open func(width, height int) (*testutil.PopupHarness, *Model)
	row  func(n int) string
}{
	{"artists", artistList, func(n int) string { return fmt.Sprintf("Artist %02d", n) }},
	{"release groups", groupList, func(n int) string { return fmt.Sprintf("Album %02d", n) }},
	{"releases", releaseList, func(n int) string { return fmt.Sprintf("Release %02d", n) }},
	{"slskd results", slskdList, func(n int) string { return fmt.Sprintf("user%02d", n) }},
	{"new releases", newReleasesList, func(n int) string { return fmt.Sprintf("Artist %02d", n) }},
}

func artistList(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateArtistResults, width, height)
	m.selectedArtist = nil
	for i := range 30 {
		m.artistResults = append(m.artistResults, core.MBArtist{
			ID: strconv.Itoa(i), Name: fmt.Sprintf("Artist %02d", i+1), Type: "Group", Country: "US", BeginYear: "1993",
		})
	}
	return h, m
}

func groupList(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateReleaseGroupResults, width, height)
	m.selectedReleaseGroup = nil
	for i := range 30 {
		m.releaseGroups = append(m.releaseGroups, core.MBReleaseGroup{
			ID: strconv.Itoa(i), Title: fmt.Sprintf("Album %02d", i+1), FirstRelease: "2001", PrimaryType: "Album",
		})
	}
	return h, m
}

func releaseList(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateReleaseResults, width, height)
	for i := range 30 {
		m.releases = append(m.releases, core.MBRelease{
			ID: strconv.Itoa(i), Title: fmt.Sprintf("Release %02d", i+1), TrackCount: 10, Date: "2002", Country: "US", Formats: "CD",
		})
	}
	return h, m
}

func slskdList(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateSlskdResults, width, height)
	for i := range 30 {
		m.slskdResults = append(m.slskdResults, core.SlskdResult{
			Username: fmt.Sprintf("user%02d", i+1), Directory: "music\\Low\\Trust", Format: "FLAC", FileCount: 10,
		})
	}
	return h, m
}

// thirtyReleases are 30 new releases, three a day from today back.
func thirtyReleases() []core.Release {
	rows := make([]core.Release, 30)
	for i := range rows {
		rows[i] = relRow(fmt.Sprintf("Artist %02d", i+1), -(i / 3), true)
	}
	return rows
}

func newReleasesList(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := popupIn(StateReleasesResults, width, height)
	m.selectedArtist = nil
	m.relDiscoveries = true
	m.relRows = thirtyReleases()
	return h, m
}

// cursorRow is the row of view under the cursor, "" when none is shown.
func cursorRow(view string) string {
	for _, line := range testutil.SplitLines(testutil.StripANSI(view)) {
		if strings.HasPrefix(line, "> ") {
			return line
		}
	}
	return ""
}

// assertInBox checks that the view of the popup holds the help line of its
// state, and that its box is no taller than the terminal of terminalHeight
// rows. It returns the view.
func assertInBox(t *testing.T, h *testutil.PopupHarness, m *Model, terminalHeight int) string {
	t.Helper()
	view := h.View()
	help := testutil.StripANSI(m.help())
	if !strings.Contains(testutil.StripANSI(view), help) {
		t.Fatalf("no help line %q in:\n%s", help, testutil.StripANSI(view))
	}
	if rows := testutil.BoxRows(view, m.Width(), m.Height()); rows > terminalHeight {
		t.Fatalf("box of %d rows in a terminal of %d:\n%s", rows, terminalHeight, testutil.StripANSI(view))
	}
	return view
}

func TestDownload_GrowsAsTheManagerTellsIt(t *testing.T) {
	var p popup.Popup = New(nil, nil, FilterConfig{}, nil)
	if _, ok := p.(popup.Growing); !ok {
		t.Error("the download popup does not grow")
	}
}

func TestDownload_ListsKeepTheirCursorAndHelpLineInTheBox(t *testing.T) {
	for _, list := range lists {
		t.Run(list.name, func(t *testing.T) {
			for height := 10; height <= 40; height++ {
				h, m := list.open(200, height)
				check := func(n int) {
					t.Helper()
					v := assertInBox(t, h, m, height)
					if row := cursorRow(v); !strings.Contains(row, list.row(n)) {
						t.Fatalf("%d rows, cursor on %d: the cursor row %q in:\n%s", height, n, row, testutil.StripANSI(v))
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
			}
		})
	}
}

func TestDownload_ListsInATallTerminalAreDrawnWhole(t *testing.T) {
	for _, list := range lists {
		t.Run(list.name, func(t *testing.T) {
			h, m := list.open(200, 60)
			assertInBox(t, h, m, 60)
			if r := anyRange.FindString(view(h)); r != "" {
				t.Errorf("range %q, want every row", r)
			}
			for n := 1; n <= 30; n++ {
				if !strings.Contains(view(h), list.row(n)) {
					t.Errorf("row %d is not drawn", n)
				}
			}
		})
	}
}

// threeNotices is the new releases list with three notices above it.
func threeNotices(width, height int) (*testutil.PopupHarness, *Model) {
	h, m := newReleasesList(width, height)
	m.relErr = "Could not refresh releases: offline"
	m.relDiscoveries = false
	m.errorMsg = "slskd not configured"
	return h, m
}

func TestDownload_NewReleasesKeepTheirHeaderAndTheDayOfTheirFirstRow(t *testing.T) {
	h, m := threeNotices(80, 14)
	assertInBox(t, h, m, 14)
	if !strings.Contains(view(h), "Recent 30") {
		t.Errorf("no header in:\n%s", view(h))
	}
	for range 4 {
		h.SendKey("j")
	}
	// The page goes on past the first row of a day: the header of the day of
	// its first row comes above it.
	for range 10 {
		h.SendKey("j")
		lines := testutil.SplitLines(view(h))
		for i, line := range lines {
			if strings.Contains(line, "Artist ") {
				if i == 0 || !strings.Contains(lines[i-1], "──") {
					t.Fatalf("the first release %q has no day above it in:\n%s", line, view(h))
				}
				break
			}
		}
		assertInBox(t, h, m, 14)
	}
}

func TestDownload_SlskdRowsFitTheTextArea(t *testing.T) {
	for width := 60; width <= 200; width++ {
		h, m := slskdList(width, 40)
		m.slskdResults[0].Username = strings.Repeat("любитель", 5)
		m.slskdResults[1].Directory = strings.Repeat("music\\", 20)
		lines := testutil.SplitLines(view(h))
		header := -1
		for i, line := range lines {
			if strings.HasPrefix(line, "  User") {
				header = i
			}
		}
		if header < 0 {
			t.Fatalf("%d columns: no header in:\n%s", width, view(h))
		}
		// The header, the separator and the rows, down to the range or the
		// blank line under them.
		for _, line := range lines[header:] {
			if line == "" || strings.HasPrefix(line, "(") {
				break
			}
			if w := lipgloss.Width(line); w > popup.TextWidth(m.Width()) {
				t.Fatalf("%d columns: %q is %d columns, the text area %d", width, line, w, popup.TextWidth(m.Width()))
			}
		}
		titles := []string{"User", "Format", "kbps", "Files", "Size", "Speed"}
		if width == 120 {
			titles = append(titles, "Directory")
		}
		if width == 80 || width == 120 {
			for _, title := range titles {
				if !strings.Contains(lines[header], title) {
					t.Errorf("%d columns: the header %q has no %s", width, lines[header], title)
				}
			}
		}
	}
}

// longNames are the lists with names 80 columns long, the releases with a
// format of 16.
func longNames(width int) []*testutil.PopupHarness {
	long := strings.Repeat("Things We Lost ", 6)[:80]
	h1, m := artistList(width, 40)
	m.artistResults[0].Name = long
	m.artistResults[1].Disambiguation = long
	h2, m := groupList(width, 40)
	m.releaseGroups[0].Title = long
	m.releaseGroups[0].SecondaryTypes = []string{"Live", "Compilation"}
	h3, m := releaseList(width, 40)
	m.releases[0].Title = long
	for i := range m.releases {
		m.releases[i].Formats = "2xCD + DVD-Video"
	}
	return []*testutil.PopupHarness{h1, h2, h3}
}

func TestDownload_ListRowsFitTheTextArea(t *testing.T) {
	for width := 80; width <= 200; width++ {
		for _, h := range longNames(width) {
			m := getModel(t, h)
			for _, cursorAt := range []int{0, 1} {
				for _, line := range testutil.SplitLines(view(h)) {
					if w := lipgloss.Width(line); w > popup.TextWidth(m.Width()) {
						t.Fatalf("%d columns, state %d: %q is %d columns, the text area %d", width, m.state, line, w, popup.TextWidth(m.Width()))
					}
				}
				if cursorAt == 0 {
					h.SendKey("j")
				}
			}
		}
	}
}

// everyState is the popup in each of its states, with long lists.
func everyState(width, height int) map[string]*testutil.PopupHarness {
	states := map[string]*testutil.PopupHarness{}
	for _, s := range []State{
		StateReleasesLoading, StateSearch, StateArtistSearching, StateReleaseGroupLoading,
		StateReleaseLoading, StateReleaseDetailsLoading, StateSlskdSearching, StateDownloading,
	} {
		states[fmt.Sprint("state ", s)], _ = popupIn(s, width, height)
	}
	for _, list := range lists {
		states[list.name], _ = list.open(width, height)
		h, m := list.open(width, height)
		m.errorMsg = "MusicBrainz is unavailable"
		m.statusMsg = "Searching..."
		states[list.name+" with an error"] = h
	}
	states["notices"], _ = threeNotices(width, height)
	h, m := newReleasesList(width, height)
	m.relSearching = true
	m.relQuery = "art"
	states["new releases searched"] = h
	for _, s := range []State{StateArtistResults, StateReleaseGroupResults, StateReleaseResults, StateSlskdResults, StateReleasesResults} {
		states[fmt.Sprint("empty ", s)], _ = popupIn(s, width, height)
	}
	return states
}

func TestDownload_HelpLinesFitANarrowTerminal(t *testing.T) {
	for name, h := range everyState(80, 24) {
		m := getModel(t, h)
		help := testutil.FindLine(view(h), testutil.StripANSI(m.help()))
		if w := lipgloss.Width(help); help == "" || w > 58 {
			t.Errorf("%s: help line %q of %d columns, want one of 58 at most", name, help, w)
		}
	}
	h, m := groupList(80, 24)
	before := testutil.StripANSI(m.help())
	h.SendKey("a")
	if after := testutil.StripANSI(m.help()); !strings.Contains(before, "Albums [on]") || !strings.Contains(after, "Albums [off]") {
		t.Errorf("the albums filter: %q then %q", before, after)
	}
	h, m = releaseList(80, 24)
	before = testutil.StripANSI(m.help())
	h.SendKey("d")
	if after := testutil.StripANSI(m.help()); !strings.Contains(before, "Dedup [on]") || !strings.Contains(after, "Dedup [off]") {
		t.Errorf("the dedup filter: %q then %q", before, after)
	}
}

func TestDownload_AWideStepIndicatorKeepsTheHelpLine(t *testing.T) {
	for _, height := range []int{16, 24} {
		h, m := releaseList(80, height)
		m.selectedArtist = &core.MBArtist{Name: strings.Repeat("a", 20)}
		m.selectedReleaseGroup = &core.MBReleaseGroup{Title: strings.Repeat("r", 40)}
		assertInBox(t, h, m, height)
	}
}

// No view panics at any size, and each keeps its help line.
func TestDownload_EveryViewKeepsItsHelpLineAtAnyHeight(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("j")}, {Type: tea.KeyRunes, Runes: []rune("k")},
		{Type: tea.KeyRunes, Runes: []rune("g")}, {Type: tea.KeyRunes, Runes: []rune("G")},
		{Type: tea.KeyCtrlD}, {Type: tea.KeyCtrlU},
	}
	for _, width := range []int{64, 100} {
		for height := 1; height <= 40; height++ {
			t.Run(fmt.Sprintf("%dx%d", width, height), func(t *testing.T) {
				for name, h := range everyState(width, height) {
					m := getModel(t, h)
					for _, key := range keys {
						var v string
						if r := testutil.PanicOf(func() {
							h.SendMsg(key)
							v = testutil.StripANSI(h.View())
						}); r != nil {
							t.Fatalf("%s, %s: panics: %v", name, key, r)
						}
						if m.Height() > 0 && !strings.Contains(v, testutil.StripANSI(m.help())) {
							t.Fatalf("%s, %s: no help line in:\n%s", name, key, v)
						}
						if r := backwards(v); r != "" {
							t.Fatalf("%s: range %s reads backwards", name, r)
						}
					}
				}
			})
		}
	}
}

// A popup told no limit stays in its own box; one told a taller limit grows
// up to it and no further.
func TestDownload_ViewsGrowUpToTheirLimitAndNoFurther(t *testing.T) {
	for name, h := range everyState(80, 24) {
		m := getModel(t, h)
		m.SetMaxHeight(0)
		if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows != m.Height() {
			t.Errorf("%s with no limit: a box of %d rows, want %d", name, rows, m.Height())
		}
		m.SetMaxHeight(m.Height() + 6)
		if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows > m.Height()+6 || rows < m.Height() {
			t.Errorf("%s up to %d rows: a box of %d rows", name, m.Height()+6, rows)
		}
	}
	for _, list := range lists {
		h, m := list.open(80, 24)
		m.SetMaxHeight(m.Height() + 6)
		if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows != m.Height()+6 {
			t.Errorf("%s up to %d rows: a box of %d rows, want %d", list.name, m.Height()+6, rows, m.Height()+6)
		}
	}
}

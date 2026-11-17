package download

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

const (
	longCyrillic = "Звезда по имени Солнце Звезда по имени Солнце Звезда по имени Солнце Звезда по имени Солнце"
	longCJK      = "坂本龍一千のナイフ坂本龍一千のナイフ坂本龍一千のナイフ坂本龍一千のナイフ坂本龍一千のナイフ"
)

// viewLine renders the popup and returns its line holding substr, ANSI
// stripped, failing when there is none or when it is not valid UTF-8.
func viewLine(t *testing.T, h *testutil.PopupHarness, substr string) string {
	t.Helper()
	view := testutil.StripANSI(h.View())
	line := testutil.FindLine(view, substr)
	if line == "" {
		t.Fatalf("no line with %q in:\n%s", substr, view)
	}
	if !utf8.ValidString(line) {
		t.Fatalf("line is not valid UTF-8: %q", line)
	}
	return line
}

// gapBefore returns the number of spaces between the "..." ending a cut name
// on line and the next column, which starts with next.
func gapBefore(t *testing.T, line, next string) int {
	t.Helper()
	_, rest, ok := strings.Cut(line, "...")
	if !ok {
		t.Fatalf("no cut name on %q", line)
	}
	gap := len(rest) - len(strings.TrimLeft(rest, " "))
	if !strings.HasPrefix(rest[gap:], next) {
		t.Fatalf("the cut name on %q is not followed by %q", line, next)
	}
	return gap
}

// The step indicator shows an artist name that fits its 20 columns whole,
// and cuts a wider one to 20 columns with "..." (issue #203).
func TestStepIndicator_CutsTheArtistNameByDisplayWidth(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Группа крови", "✓ Группа крови  →"},
		{"Государственный академический оркестр", "✓ Государственный а...  →"},
	}
	for _, tt := range tests {
		h := newDownloadPopup()
		m := getModel(t, h)
		m.state = StateReleaseGroupResults
		m.selectedArtist = &core.MBArtist{Name: tt.name}
		m.releaseGroups = []core.MBReleaseGroup{{Title: "Abbey Road"}}
		viewLine(t, h, tt.want)
	}
}

// The lists of the download popup cut a long name to the width of its column,
// between two characters: a Cyrillic one fills it, so the next column follows
// after its gap of 2; a CJK one may leave one column more (issue #203).
func TestLists_CutALongNameToItsColumn(t *testing.T) {
	lists := []struct {
		name  string
		setup func(m *Model, name string)
		next  string
	}{
		{"artist name", func(m *Model, name string) {
			m.state = StateArtistResults
			m.artistResults = []core.MBArtist{{Name: name, Type: "Group", Country: "RU"}}
		}, "Group"},
		{"artist disambiguation", func(m *Model, name string) {
			m.state = StateArtistResults
			m.artistResults = []core.MBArtist{{Name: "Кино", Disambiguation: name, Type: "Group", Country: "RU"}}
		}, "Group"},
		{"release group", func(m *Model, name string) {
			m.state = StateReleaseGroupResults
			m.selectedArtist = &core.MBArtist{Name: "Кино"}
			m.releaseGroups = []core.MBReleaseGroup{{Title: name, FirstRelease: "1988", PrimaryType: "Album"}}
		}, "1988"},
		{"release", func(m *Model, name string) {
			m.state = StateReleaseResults
			m.selectedArtist = &core.MBArtist{Name: "Кино"}
			m.releases = []core.MBRelease{{Title: name, TrackCount: 10, Date: "1988", Country: "SU", Formats: "Vinyl"}}
		}, "10 tracks"},
	}
	for _, list := range lists {
		t.Run(list.name, func(t *testing.T) {
			for _, name := range []string{longCyrillic, longCJK} {
				h := newDownloadPopup()
				list.setup(getModel(t, h), name)
				gap := gapBefore(t, viewLine(t, h, string([]rune(name)[:4])), list.next)
				if name == longCyrillic && gap != 2 {
					t.Errorf("%s: %d spaces before %q, want 2", name, gap, list.next)
				}
				if name == longCJK && gap != 2 && gap != 3 {
					t.Errorf("%s: %d spaces before %q, want 2 or 3", name, gap, list.next)
				}
			}
		})
	}
}

// The User column of the slskd results cuts a long user name to its 18
// columns, between two characters (issue #203).
func TestSlskdResults_CutALongUserNameToItsColumn(t *testing.T) {
	tests := []struct {
		user string
		want string
	}{
		{"любитель_музыки_из_москвы", "> любитель_музыки... Music"},
		{"坂本龍一坂本龍一坂本龍一", "> 坂本龍一坂本龍..."},
	}
	for _, tt := range tests {
		h := newDownloadPopup()
		m := getModel(t, h)
		m.state = StateSlskdResults
		m.selectedReleaseGroup = &core.MBReleaseGroup{Title: "Группа крови"}
		m.slskdResults = []core.SlskdResult{{Username: tt.user, Directory: "Music", Format: "FLAC", FileCount: 10}}
		viewLine(t, h, tt.want)
	}
}

// newSlskdResultsPopup returns the download popup at width x 40, showing
// results.
func newSlskdResultsPopup(t *testing.T, width int, results []core.SlskdResult) *testutil.PopupHarness {
	t.Helper()
	h := newDownloadPopup()
	h.SetSize(width, 40)
	m := getModel(t, h)
	m.state = StateSlskdResults
	m.selectedReleaseGroup = &core.MBReleaseGroup{Title: "Группа крови"}
	m.slskdResults = results
	return h
}

// slskdRows returns the first rows of the slskd results, ANSI stripped,
// failing when a row is not valid UTF-8 or not as wide as the header.
func slskdRows(t *testing.T, h *testutil.PopupHarness, rows int) []string {
	t.Helper()
	view := strings.Split(testutil.StripANSI(h.View()), "\n")
	for i, line := range view {
		if !strings.HasPrefix(line, "  User ") {
			continue
		}
		if i+2+rows > len(view) {
			t.Fatalf("fewer than %d rows under the header in:\n%s", rows, strings.Join(view, "\n"))
		}
		lines := view[i+2 : i+2+rows]
		for _, row := range lines {
			if !utf8.ValidString(row) {
				t.Fatalf("row is not valid UTF-8: %q", row)
			}
			if got, want := lipgloss.Width(row), lipgloss.Width(line); got != want {
				t.Errorf("row %q is %d columns wide, header %d", row, got, want)
			}
		}
		return lines
	}
	t.Fatalf("no header in:\n%s", strings.Join(view, "\n"))
	return nil
}

// Every row of the slskd results is as wide as the header, under the cursor
// or not, whatever the script of the user and folder names, in both layouts
// (issue #313).
func TestSlskdResults_RowsAreAsWideAsTheHeader(t *testing.T) {
	results := []core.SlskdResult{
		{Username: "音楽好き", Directory: "音楽\\坂本龍一", Format: "FLAC", FileCount: 11},
		{Username: "любитель", Directory: "музыка\\Кино\\Группа крови", Format: "FLAC", FileCount: 11},
		{Username: "bob", Directory: "music\\Kino\\1988 - Gruppa krovi", Format: "MP3", BitRate: 320, FileCount: 11},
	}
	for _, width := range []int{120, 80} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			slskdRows(t, newSlskdResultsPopup(t, width, results), len(results))
		})
	}
}

// In the wide layout, the Directory column shows a folder that fits whole,
// and cuts a wider one from its start, between two characters, to its 50
// columns: it begins with "..." and keeps the end. The row stays as wide as
// the header (issue #313).
func TestSlskdResults_CutALongFolderFromItsStart(t *testing.T) {
	const (
		cyrillic = "музыка\\Кино\\1988 - Группа крови (FLAC)\\Диск 1 из 2 переиздание"
		cjk      = "音楽\\坂本龍一\\千のナイフ\\戦場のメリークリスマス・オリジナル・サウンドトラック"
		ascii    = "music\\Kino\\1988 - Gruppa krovi (Remastered 2010)\\CD 1"
	)
	results := []core.SlskdResult{
		{Username: "u0", Directory: "Группа крови", Format: "FLAC"},
		{Username: "u1", Directory: cyrillic, Format: "FLAC"},
		{Username: "u2", Directory: cjk, Format: "FLAC"},
		{Username: "u3", Directory: ascii, Format: "FLAC"},
	}
	rows := slskdRows(t, newSlskdResultsPopup(t, 120, results), len(results))
	// The Directory column: after the 2-column marker, the 18 of User and a
	// space, 50 columns.
	folder := func(row string) string {
		return strings.TrimRight(ansi.Cut(row, 21, 71), " ")
	}

	if got := folder(rows[0]); got != "Группа крови" {
		t.Errorf("folder that fits shown as %q", got)
	}
	for _, tt := range []struct {
		row      int
		dir      string
		minWidth int
	}{{1, cyrillic, 50}, {2, cjk, 49}} {
		got := folder(rows[tt.row])
		kept, ok := strings.CutPrefix(got, "...")
		if !ok || !strings.HasSuffix(tt.dir, kept) {
			t.Errorf("%q shown as %q, want ... and its end", tt.dir, got)
		}
		if w := lipgloss.Width(got); w < tt.minWidth || w > 50 {
			t.Errorf("%q shown in %d columns, want %d to 50", tt.dir, w, tt.minWidth)
		}
	}
	if got, want := folder(rows[3]), "..."+ascii[len(ascii)-47:]; got != want {
		t.Errorf("ASCII folder shown as %q, want %q", got, want)
	}
}

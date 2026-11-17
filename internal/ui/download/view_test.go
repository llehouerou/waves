package download

import (
	"strings"
	"testing"
	"unicode/utf8"

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

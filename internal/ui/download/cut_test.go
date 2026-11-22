package download

import (
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// cutPopup is a fresh download popup on a MusicBrainz that has total release
// groups of Nirvana and total releases of Unplugged, of which it lists one;
// as many as it lists if total is 0.
func cutPopup(total int) (*testutil.PopupHarness, *Model) {
	m := New(&fakeDownloads{}, fakeMB{cut: total}, FilterConfig{}, nil)
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m), m
}

// assertNotice checks the view tells the cut of its list with want, or tells
// no cut if want is empty.
func assertNotice(t *testing.T, h *testutil.PopupHarness, want string) {
	t.Helper()
	view := testutil.StripANSI(h.View())
	if want == "" {
		if strings.Contains(view, "read from MusicBrainz") {
			t.Errorf("a list read in full tells a cut:\n%s", view)
		}
		return
	}
	if !strings.Contains(view, want) {
		t.Errorf("the view does not tell %q:\n%s", want, view)
	}
}

func TestReleaseGroups_ACutListTellsBothNumbers(t *testing.T) {
	h, m := cutPopup(6235)
	listGroups(t, h, m)
	const notice = "1 of 6235 release groups read from MusicBrainz"
	assertNotice(t, h, notice)

	for range 2 {
		h.SendKey("a") // the albums-only filter changes the rows, not the cut
		assertNotice(t, h, notice)
	}

	h.SendKey("backspace")
	assertNotice(t, h, "")
}

func TestReleaseGroups_AListReadInFullTellsNoCut(t *testing.T) {
	h, m := cutPopup(0)
	listGroups(t, h, m)
	assertNotice(t, h, "")
}

func TestReleases_ACutListTellsBothNumbers(t *testing.T) {
	h, m := cutPopup(2500)
	listReleases(t, h, m)
	const notice = "1 of 2500 releases read from MusicBrainz"
	assertNotice(t, h, notice)

	for range 2 {
		h.SendKey("d") // the deduplication changes the rows, not the cut
		assertNotice(t, h, notice)
	}

	// Back on the release groups, the releases are forgotten with their cut.
	h.SendKey("backspace")
	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want StateReleaseGroupResults", m.state)
	}
	assertNotice(t, h, "1 of 2500 release groups read from MusicBrainz")
	if strings.Contains(testutil.StripANSI(h.View()), "releases read from MusicBrainz") {
		t.Errorf("the release groups tell the cut of the releases left")
	}
}

func TestReleases_AListReadInFullTellsNoCut(t *testing.T) {
	h, m := cutPopup(0)
	listReleases(t, h, m)
	assertNotice(t, h, "")
}

// releasesOfRow is a popup on a MusicBrainz that has total releases of
// Unplugged, entered from the new releases on unpluggedRow, at the list of
// its releases.
func releasesOfRow(t *testing.T, total int) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m := cutPopup(total)
	m.Update(loaded(t, m.StartReleases(&fakeReleases{list: []core.Release{unpluggedRow()}})))
	stepTo(t, h, m, h.SendEnter(), StateReleaseResults)
	return h, m
}

func TestReleasesFromNewReleases_ACutListTellsBothNumbers(t *testing.T) {
	h, m := releasesOfRow(t, 2500)
	const notice = "1 of 2500 releases read from MusicBrainz"
	assertNotice(t, h, notice)
	h.SendKey("d")
	assertNotice(t, h, notice)

	h.SendKey("backspace")
	if m.state != StateReleasesResults {
		t.Fatalf("state = %d, want StateReleasesResults", m.state)
	}
	assertNotice(t, h, "")
}

func TestReleasesFromNewReleases_AListReadInFullTellsNoCut(t *testing.T) {
	h, _ := releasesOfRow(t, 0)
	assertNotice(t, h, "")
}

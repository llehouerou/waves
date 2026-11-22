package retag

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// === Lists MusicBrainz gave in part (issue #320) ===

// artistGroupsPopup opens the popup of an album whose tags carry the artist
// ID only, on a MusicBrainz that lists mixedGroups of the artist and both
// releases of "rg" but has total of each (as many as it lists if 0), and
// shows the artist's groups.
func artistGroupsPopup(t *testing.T, total int) (*testutil.PopupHarness, *Model) {
	t.Helper()
	mb := fakeMB{artist: mixedGroups(), cut: total, searches: map[string][]core.MBReleaseGroup{"low": mixedGroups()}}
	h, m := openPopupOn(&fakeLibrary{tags: artistTags("artist")}, mb)
	settle(t, h, h.LastCommand())
	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the list of the artist's release groups", m.state)
	}
	return h, m
}

// releasesOfRG is artistGroupsPopup on the releases of "rg", picked from the
// list.
func releasesOfRG(t *testing.T, total int) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m := artistGroupsPopup(t, total)
	for range 4 {
		h.SendDown()
	}
	settle(t, h, h.SendEnter())
	if m.state != StateReleaseResults {
		t.Fatalf("state = %d, want the releases of rg", m.state)
	}
	return h, m
}

func assertNoCut(t *testing.T, h *testutil.PopupHarness) {
	t.Helper()
	if view := testutil.StripANSI(h.View()); strings.Contains(view, "read from MusicBrainz") {
		t.Errorf("the view tells a cut:\n%s", view)
	}
}

func TestRetag_ACutListOfTheArtistsGroupsTellsBothNumbers(t *testing.T) {
	h, _ := artistGroupsPopup(t, 6235)
	const notice = "5 of 6235 release groups read from MusicBrainz"
	assertView(t, h, notice)

	for range 2 {
		h.SendKey("a") // the albums-only filter changes the rows, not the cut
		assertView(t, h, notice)
	}
}

func TestRetag_AListOfTheArtistsGroupsReadInFullTellsNoCut(t *testing.T) {
	h, _ := artistGroupsPopup(t, 0)
	assertNoCut(t, h)
}

// A search replaces the artist's groups with what it finds, never cut.
func TestRetag_ASearchForgetsTheCutOfTheArtistsGroups(t *testing.T) {
	h, m := artistGroupsPopup(t, 6235)

	h.SendKey("/")
	h.SendSpecialKey(tea.KeyCtrlU)
	h.SendKey("low")
	settle(t, h, h.SendEnter())

	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the list of the release groups found", m.state)
	}
	assertNoCut(t, h)
}

func TestRetag_ACutListOfReleasesTellsBothNumbers(t *testing.T) {
	h, m := releasesOfRG(t, 2500)
	assertView(t, h, "2 of 2500 releases read from MusicBrainz")

	// Back on the groups, the releases are forgotten with their cut.
	h.SendSpecialKey(tea.KeyBackspace)
	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the list of the release groups", m.state)
	}
	if view := testutil.StripANSI(h.View()); strings.Contains(view, "releases read from MusicBrainz") {
		t.Errorf("the release groups tell the cut of the releases left:\n%s", view)
	}
}

func TestRetag_AListOfReleasesReadInFullTellsNoCut(t *testing.T) {
	h, _ := releasesOfRG(t, 0)
	assertNoCut(t, h)
}

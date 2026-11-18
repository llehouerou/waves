package retag

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// === The release groups listed (issue #210) ===

// group is a release group of title by artist, of the types given.
func group(id, artist, title, year, primary string, secondary ...string) core.MBReleaseGroup {
	return core.MBReleaseGroup{
		ID: id, Title: title, Artist: artist, FirstRelease: year,
		PrimaryType: primary, SecondaryTypes: secondary,
	}
}

// mixedGroups are release groups of every type by "low", none named "trust":
// the studio album "rg" alone is an album as the filter means it.
func mixedGroups() []core.MBReleaseGroup {
	return []core.MBReleaseGroup{
		group("live", "Low", "Live at Rock City", "2004", "Album", "Live"),
		group("comp", "Low", "A Lifetime of Temporary Relief", "2004", "Album", "Compilation"),
		group("ep", "Low", "Songs for a Dead Pilot", "1997", "EP"),
		group("single", "Low", "Canada", "2002", "Single"),
		group("rg", "Low", "Things We Lost in the Fire", "2001", "Album"),
	}
}

// mixedRows are the rows of mixedGroups, in their order.
var mixedRows = []string{
	"Low - Live at Rock City (2004) [Album] +Live",
	"Low - A Lifetime of Temporary Relief (2004) [Album] +Compilation",
	"Low - Songs for a Dead Pilot (1997) [EP]",
	"Low - Canada (2002) [Single]",
	"Low - Things We Lost in the Fire (2001) [Album]",
}

// searchedPopup opens the popup of "trust" by "low", whose tags carry no
// MusicBrainz ID, and hands it the groups its search finds. It returns what
// the popup started on them.
func searchedPopup(t *testing.T, groups ...core.MBReleaseGroup) (*testutil.PopupHarness, *Model, tea.Cmd) {
	t.Helper()
	lib := &fakeLibrary{tags: untaggedTags()}
	h, m := openPopupOn(lib, fakeMB{searches: map[string][]core.MBReleaseGroup{"low trust": groups}})
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the search
	_, cmd = h.ExecuteAndSend(cmd)              // the groups found
	h.ClearCommands()
	return h, m, cmd
}

// listingPopup is searchedPopup on the list of the groups found.
func listingPopup(t *testing.T, groups ...core.MBReleaseGroup) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m, cmd := searchedPopup(t, groups...)
	if cmd != nil || m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the list of the release groups found", m.state)
	}
	return h, m
}

// listed returns the rows of the release groups the popup lists, in order.
func listed(h *testutil.PopupHarness) []string {
	var rows []string
	inList := false
	for _, line := range testutil.SplitLines(testutil.StripANSI(h.View())) {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Select a release group:"):
			inList = true
		case inList && line == "" && len(rows) > 0:
			return rows
		case inList && line != "":
			rows = append(rows, strings.TrimPrefix(line, "> "))
		}
	}
	return rows
}

func assertListed(t *testing.T, h *testutil.PopupHarness, want ...string) {
	t.Helper()
	if got := listed(h); !slices.Equal(got, want) {
		t.Errorf("listed:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// assertListsReleasesOfTrust checks that the popup, on cmd, went straight to
// the releases of the group of "trust" by "low", "rg": fakeMB fails the
// releases of any other.
func assertListsReleasesOfTrust(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if m.state != StateReleaseLoading || cmd == nil {
		t.Fatalf("state = %d, want the releases of rg loading", m.state)
	}
	msg, ok := cmd().(mbworkflow.ReleasesResultMsg)
	if !ok || msg.Err != nil || len(msg.Releases) != 2 {
		t.Errorf("the popup asked %#v, want the releases of rg", msg)
	}
}

func TestRetag_TheAlbumsOwnLiveGroupIsSelectedOverAStudioAlbumOfAnotherArtist(t *testing.T) {
	_, m, cmd := searchedPopup(t,
		group("tribute", "Various Artists", "Trust", "2010", "Album"),
		searchedTrust(),
	)
	assertListsReleasesOfTrust(t, m, cmd)
}

func TestRetag_EveryReleaseGroupFoundIsListedWithItsType(t *testing.T) {
	h, _ := listingPopup(t, mixedGroups()...)
	assertListed(t, h, mixedRows...)
	assertView(t, h, "[a] Albums only: off")
}

func TestRetag_EveryReleaseGroupOfTheArtistIsListedWithItsType(t *testing.T) {
	fileTags := untaggedTags()
	for i := range fileTags {
		fileTags[i].MBArtistID = "artist"
	}
	h, m := openPopupOn(&fakeLibrary{tags: fileTags}, fakeMB{artist: mixedGroups()})
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the artist's groups
	_, cmd = h.ExecuteAndSend(cmd)
	if cmd != nil || m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the list of the artist's release groups", m.state)
	}
	assertListed(t, h, mixedRows...)
}

func TestRetag_ATogglesTheAlbumsOnlyFilter(t *testing.T) {
	h, m := listingPopup(t, mixedGroups()...)

	if cmd := h.SendKey("a"); cmd != nil {
		t.Errorf("the toggle started %#v", cmd())
	}
	assertListed(t, h, mixedRows[4])
	assertView(t, h, "[a] Albums only: on")

	if cmd := h.SendKey("a"); cmd != nil {
		t.Errorf("the toggle started %#v", cmd())
	}
	assertListed(t, h, mixedRows...)
	assertView(t, h, "[a] Albums only: off")
	if m.state != StateReleaseGroupResults {
		t.Errorf("state = %d, want the list of the release groups", m.state)
	}
}

func TestRetag_TheFilterOnWithNoAlbumHidesEveryGroupAndSaysSo(t *testing.T) {
	groups := mixedGroups()[:4]
	h, _ := listingPopup(t, groups...)

	h.SendKey("a")
	assertListed(t, h)
	assertView(t, h, "4 release groups hidden by the albums-only filter", "[a] Albums only: on")
	if err := h.AssertViewNotContains("No release groups found"); err != "" {
		t.Error(err)
	}
	if cmd := h.SendEnter(); cmd != nil {
		t.Errorf("Enter on an empty list started %#v", cmd())
	}

	h.SendKey("a")
	assertListed(t, h, mixedRows[:4]...)
}

func TestRetag_TheToggleKeepsTheCursorOnAListedRow(t *testing.T) {
	h, m := listingPopup(t, mixedGroups()...)
	for range 3 {
		h.SendDown()
	}

	h.SendKey("a")
	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the list of the release groups", m.state)
	}
	if !strings.Contains(testutil.StripANSI(h.View()), "> "+mixedRows[4]) {
		t.Errorf("the cursor is on no listed row:\n%s", testutil.StripANSI(h.View()))
	}
}

// The list is longer than the rows it shows: turned on with the cursor far
// down, the filter still shows the row under the cursor.
func TestRetag_TheToggleShowsTheCursorOfAScrolledList(t *testing.T) {
	groups := mixedGroups()
	for i := range 40 {
		groups = append(groups, group(fmt.Sprint("single", i), "Low", fmt.Sprint("Single ", i), "2005", "Single"))
	}
	h, _ := listingPopup(t, groups...)
	for range 35 {
		h.SendDown()
	}

	h.SendKey("a")

	if !strings.Contains(testutil.StripANSI(h.View()), "> "+mixedRows[4]) {
		t.Errorf("the cursor is on no listed row:\n%s", testutil.StripANSI(h.View()))
	}
}

func TestRetag_TheFilterIsSaidWhenNothingWasFound(t *testing.T) {
	h, _ := listingPopup(t)
	assertView(t, h, "No release groups found", "[a] Albums only: off")

	h.SendKey("a")
	assertView(t, h, "No release groups found", "[a] Albums only: on")
}

func TestRetag_TheFilterStaysOnAcrossASearch(t *testing.T) {
	lib := &fakeLibrary{tags: untaggedTags()}
	mb := fakeMB{searches: map[string][]core.MBReleaseGroup{"low trust": mixedGroups(), "low": mixedGroups()}}
	h, m := openPopupOn(lib, mb)
	settle(t, h, h.LastCommand())
	h.SendKey("a")

	h.SendKey("/")
	h.SendSpecialKey(tea.KeyCtrlU)
	h.SendKey("low")
	settle(t, h, h.SendEnter())

	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the list of the release groups", m.state)
	}
	assertListed(t, h, mixedRows[4])
	assertView(t, h, "[a] Albums only: on")
}

func TestRetag_TheFilterStaysOnAcrossTheReleasesOfAGroup(t *testing.T) {
	h, m := listingPopup(t, mixedGroups()...)
	h.SendKey("a")

	settle(t, h, h.SendEnter()) // the releases of the studio album
	if m.state != StateReleaseResults {
		t.Fatalf("state = %d, want the releases of the group", m.state)
	}
	h.SendSpecialKey(tea.KeyBackspace)

	assertListed(t, h, mixedRows[4])
	assertView(t, h, "[a] Albums only: on")
}

func TestRetag_APopupOpensWithTheFilterOff(t *testing.T) {
	h, _ := listingPopup(t, mixedGroups()...)
	h.SendKey("a")

	h, _ = listingPopup(t, mixedGroups()...)
	assertListed(t, h, mixedRows...)
	assertView(t, h, "[a] Albums only: off")
}

func TestRetag_ATypedInTheSearchGoesInTheQuery(t *testing.T) {
	h, m := listingPopup(t, mixedGroups()...)
	h.SendKey("/")
	h.SendSpecialKey(tea.KeyCtrlU)

	h.SendKey("a")

	if got := m.searchInput.Value(); got != "a" {
		t.Errorf("query = %q, want %q", got, "a")
	}
	h.SendSpecialKey(tea.KeyEsc)
	assertListed(t, h, mixedRows...)
	assertView(t, h, "[a] Albums only: off")
}

func TestRetag_TheStudioAlbumAmongTheGroupsNamedAsTheAlbumIsSelected(t *testing.T) {
	h, m, cmd := searchedPopup(t,
		group("single", "Low", "Trust", "2003", "Single"),
		group("rg", "Low", "Trust", "2002", "Album"),
	)
	assertListsReleasesOfTrust(t, m, cmd)

	settle(t, h, cmd) // the releases of the album
	h.SendSpecialKey(tea.KeyBackspace)
	assertListed(t, h,
		"Low - Trust (2003) [Single]",
		"Low - Trust (2002) [Album]",
	)
}

func TestRetag_GroupsNamedAsTheAlbumWithNoStudioAlbumAreListedOnTop(t *testing.T) {
	h, _ := listingPopup(t,
		group("tribute", "Various Artists", "Trust", "2010", "Album"),
		group("single", "Low", "Trust", "2003", "Single"),
		searchedTrust(),
	)
	assertListed(t, h,
		"Low - Trust (2003) [Single]",
		"Low - Trust (2002) [Album] +Live",
		"Various Artists - Trust (2010) [Album]",
	)
}

func TestRetag_TwoStudioAlbumsNamedAsTheAlbumAreListed(t *testing.T) {
	h, _ := listingPopup(t,
		group("rg", "Low", "Trust", "2002", "Album"),
		group("remake", "Low", "Trust", "2012", "Album"),
	)
	assertListed(t, h,
		"Low - Trust (2002) [Album]",
		"Low - Trust (2012) [Album]",
	)
}

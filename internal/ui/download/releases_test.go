package download

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// relRow builds a row dated daysFromToday days from today.
func relRow(artist string, daysFromToday int, inLibrary bool) core.Release {
	return core.Release{
		ReleaseGroupMBID: artist,
		ArtistCreditName: artist,
		ReleaseName:      artist + " album",
		ReleaseDate:      time.Now().AddDate(0, 0, daysFromToday).Format(time.DateOnly),
		PrimaryType:      "Album",
		InLibrary:        inLibrary,
	}
}

// fakeReleases is the new releases of a core: the list and the status it was
// given, and the refreshes it was asked for.
type fakeReleases struct {
	list      []core.Release
	err       error // why the list cannot be read
	status    core.ReleasesStatus
	refreshes []bool // the force of each Refresh, in order
}

func (f *fakeReleases) List() ([]core.Release, error) { return f.list, f.err }

func (f *fakeReleases) Status() core.ReleasesStatus { return f.status }

func (f *fakeReleases) Refresh(force bool) { f.refreshes = append(f.refreshes, force) }

// loaded runs a command of the popup that reads the list, and returns what it
// read.
func loaded(t *testing.T, cmd tea.Cmd) ReleasesLoadedMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("the list is not read")
	}
	msg, ok := cmd().(ReleasesLoadedMsg)
	if !ok {
		t.Fatalf("the command read no list")
	}
	return msg
}

// Opening the list is what keeps it fresh: the core refreshes what is stale,
// and the popup shows what the core says of it.
func TestStartReleases_AsksTheCoreToRefreshAndShowsWhereItIs(t *testing.T) {
	rel := &fakeReleases{
		list:   []core.Release{relRow("Mogwai", -1, true)},
		status: core.ReleasesStatus{Refreshing: true, Discoveries: true, Err: errors.New("listenbrainz status 503")},
	}
	m := New(nil, nil, FilterConfig{}, nil)

	cmd := m.StartReleases(rel)

	if len(rel.refreshes) != 1 || rel.refreshes[0] {
		t.Errorf("refreshes asked = %v, want one, not forced", rel.refreshes)
	}
	if !m.relRefreshing || !m.relDiscoveries || !strings.Contains(m.relErr, "503") {
		t.Errorf("refreshing %v, discoveries %v, error %q: want what the core's status says", m.relRefreshing, m.relDiscoveries, m.relErr)
	}
	m.Update(loaded(t, cmd))
	if m.state != StateReleasesResults || len(m.relRows) != 1 {
		t.Errorf("state %d with %d rows, want the core's list shown", m.state, len(m.relRows))
	}
	// Reading the list is not a refresh that worked: the failure stays told.
	if !strings.Contains(m.relErr, "503") {
		t.Errorf("error = %q, want the failed refresh still told", m.relErr)
	}
}

func TestReleasesKeyR_ForcesARefresh(t *testing.T) {
	rel := &fakeReleases{status: core.ReleasesStatus{Err: errors.New("listenbrainz status 503")}}
	m := New(nil, nil, FilterConfig{}, nil)
	m.Update(loaded(t, m.StartReleases(rel)))

	testutil.NewPopupHarness(m).SendKey("r")

	if len(rel.refreshes) != 2 || !rel.refreshes[1] {
		t.Errorf("refreshes asked = %v, want a forced one after the one of the opening", rel.refreshes)
	}
	if !m.relRefreshing || m.relErr != "" {
		t.Errorf("refreshing %v, error %q: want a refresh running, the old failure gone", m.relRefreshing, m.relErr)
	}
}

func TestReleasesRefreshed_EndsTheRefreshAndReadsTheListAgain(t *testing.T) {
	rel := &fakeReleases{status: core.ReleasesStatus{Refreshing: true}}
	m := New(nil, nil, FilterConfig{}, nil)
	m.Update(loaded(t, m.StartReleases(rel)))

	rel.list = []core.Release{relRow("Mogwai", -1, true)}
	_, cmd := m.Update(core.ReleasesRefreshed{})

	if m.relRefreshing {
		t.Error("the refresh still shows as running")
	}
	m.Update(loaded(t, cmd))
	if len(m.relRows) != 1 {
		t.Errorf("rows = %d, want the refreshed list", len(m.relRows))
	}
}

// A failed refresh never steals focus: the list keeps what it showed, with a
// discreet error line.
func TestReleasesRefreshed_AFailureKeepsTheListAndTellsIt(t *testing.T) {
	rel := &fakeReleases{
		list:   []core.Release{relRow("Mogwai", -1, true)},
		status: core.ReleasesStatus{Refreshing: true},
	}
	m := New(nil, nil, FilterConfig{}, nil)
	m.Update(loaded(t, m.StartReleases(rel)))

	_, cmd := m.Update(core.ReleasesRefreshed{Err: errors.New("listenbrainz status 503")})

	if cmd != nil {
		t.Error("the list is read again after a refresh that changed nothing")
	}
	if m.relRefreshing || !strings.Contains(m.relErr, "503") || len(m.relRows) != 1 {
		t.Errorf("refreshing %v, error %q, %d rows: want the refresh over, its failure told, the list kept", m.relRefreshing, m.relErr, len(m.relRows))
	}
}

func TestWarmupFinished_ReadsTheListAgainWithoutEndingTheRefresh(t *testing.T) {
	rel := &fakeReleases{status: core.ReleasesStatus{Refreshing: true}}
	m := New(nil, nil, FilterConfig{}, nil)
	m.Update(loaded(t, m.StartReleases(rel)))

	// The warm-up ends while the ListenBrainz refresh still runs.
	rel.list = []core.Release{relRow("Jane Weaver", -1, false)}
	_, cmd := m.Update(core.WarmupFinished{})

	m.Update(loaded(t, cmd))
	if len(m.relRows) != 1 {
		t.Errorf("rows = %d, want the list with its new discovery", len(m.relRows))
	}
	if !m.relRefreshing {
		t.Error("the end of the warm-up ended the refresh")
	}
}

// The releases of a core are not this popup's business until it shows them.
func TestReleasesEvents_AreIgnoredByAPopupThatNeverOpenedTheList(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)

	for _, e := range []tea.Msg{core.ReleasesRefreshed{}, core.WarmupFinished{}} {
		if _, cmd := m.Update(e); cmd != nil {
			t.Errorf("%T: the list is read", e)
		}
	}
}

func TestVisibleReleases_TabsFilterAndOrder(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.relRows = []core.Release{
		relRow("old", -10, true),
		relRow("yesterday", -1, false),
		relRow("today", 0, true),
		relRow("soon", 2, false),
		relRow("later", 20, true),
	}

	tests := []struct {
		name   string
		tab    relTab
		filter relFilter
		want   []string
	}{
		{"recent runs backwards from today", relTabRecent, relFilterAll, []string{"today", "yesterday", "old"}},
		{"upcoming runs forwards", relTabUpcoming, relFilterAll, []string{"soon", "later"}},
		{"library only", relTabRecent, relFilterLibrary, []string{"today", "old"}},
		{"discoveries only", relTabUpcoming, relFilterDiscoveries, []string{"soon"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.relTab, m.relFilter = tt.tab, tt.filter
			got := m.visibleReleases()
			if len(got) != len(tt.want) {
				t.Fatalf("got %d rows, want %d", len(got), len(tt.want))
			}
			for i, name := range tt.want {
				if got[i].ArtistCreditName != name {
					t.Errorf("row %d = %q, want %q", i, got[i].ArtistCreditName, name)
				}
			}
		})
	}
}

// One release is one line: nothing the panel renders may exceed the width left
// by the popup border and padding, down to the narrowest portrait terminal.
func TestRenderReleases_NeverWraps(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.state = StateReleasesResults
	m.relDiscoveries = true
	m.relErr = "Could not refresh releases: Get \"https://api.listenbrainz.org\": dial tcp: lookup failed"
	m.statusMsg = "Download queued"

	long := relRow("A Very Long Artist Name Indeed", 0, false)
	long.Seeds = []string{"Stereolab", "Broadcast", "Portishead", "Cate Le Bon"}
	cjk := relRow("\u30b8\u30e3\u30d1\u30cb\u30fc\u30ba\u30fb\u30a2\u30fc\u30c6\u30a3\u30b9\u30c8", -1, true)
	m.relRows = []core.Release{long, cjk}

	for _, style := range []string{"nerd", "unicode", "none"} {
		icons.Init(style)
		for _, box := range []int{100, 78, 58, 44, 30} {
			m.SetSize(box, 20)
			budget := m.releasesWidth()
			panel := m.View()
			for line := range strings.SplitSeq(panel, "\n") {
				if w := lipgloss.Width(line); w > budget {
					t.Errorf("%s at %d cols: line is %d wide (budget %d): %q", style, box, w, budget, line)
				}
			}
		}
	}
	icons.Init("nerd")
}

// A downloading release wears the queued icon and the muted row style.
func TestRenderReleaseRow_ADownloadingReleaseWearsTheQueuedIcon(t *testing.T) {
	row := relRow("Jane Weaver", 0, false)
	row.Downloading = true

	icons.Init("none")
	defer icons.Init("nerd")
	if line := renderReleaseRow(row, 60, false, row.Downloading); !strings.Contains(line, icons.Queued()) {
		t.Errorf("downloading row should carry the queued icon: %q", line)
	}
}

func TestRenderReleaseRow_TitleKeepsItsSpace(t *testing.T) {
	row := relRow("Jane Weaver", 0, false)
	row.Seeds = []string{"Stereolab", "Broadcast", "Portishead", "Cate Le Bon"}

	tests := []struct {
		width int
		want  string // meta fragment expected on the row
	}{
		{100, "Stereolab"}, // wide: seed names
		{58, "←4"},         // narrow: seed counter
		{40, "Album"},      // portrait: type only
	}
	for _, tt := range tests {
		line := renderReleaseRow(row, tt.width, false, false)
		if !strings.Contains(line, tt.want) {
			t.Errorf("width %d: %q does not contain %q", tt.width, line, tt.want)
		}
		if !strings.Contains(line, "Jane Weaver") {
			t.Errorf("width %d: the title was squeezed out: %q", tt.width, line)
		}
		// The cursor prefix is part of the budget: one release is one line.
		if w := lipgloss.Width(line); w > tt.width {
			t.Errorf("width %d: row is %d wide", tt.width, w)
		}
	}
}

func TestReleasesSearch_FiltersAndClears(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.state = StateReleasesResults
	m.relRows = []core.Release{relRow("Mogwai", -1, true), relRow("Radiohead", -1, true)}

	h := testutil.NewPopupHarness(m)
	h.SendKey("/")
	h.SendKey("m")
	h.SendKey("o")
	// Space arrives as KeySpace, not as a rune, and must reach the query.
	h.SendSpecialKey(tea.KeySpace)
	if m.relQuery != "mo " {
		t.Fatalf("query = %q, want %q", m.relQuery, "mo ")
	}
	m.relQuery = "mo"
	if got := m.visibleReleases(); len(got) != 1 || got[0].ArtistCreditName != "Mogwai" {
		t.Fatalf("search should keep Mogwai only, got %d rows", len(got))
	}

	h.SendKey("enter") // keeps the query, leaves typing mode
	if m.relSearching || m.relQuery != "mo" {
		t.Errorf("enter should keep the query, got %q (searching=%v)", m.relQuery, m.relSearching)
	}

	h.SendEscape() // clears the query instead of closing the popup
	if m.relQuery != "" || len(m.visibleReleases()) != 2 {
		t.Error("esc should clear the search and restore the full list")
	}
}

func TestHandleReleasesEnter_RefusesWithoutSlskd(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.state = StateReleasesResults
	m.relRows = []core.Release{relRow("a", -1, true)}

	if cmd := m.handleReleasesEnter(); cmd != nil {
		t.Error("expected no command without slskd configured")
	}
	if m.errorMsg != SlskdMissingMsg {
		t.Errorf("errorMsg = %q, want the slskd configuration message", m.errorMsg)
	}
	if m.state != StateReleasesResults {
		t.Error("state should stay on the releases list")
	}
}

func TestHandleReleasesEnter_JumpsToTheReleasesOfTheRow(t *testing.T) {
	m := New(&fakeDownloads{}, nil, FilterConfig{}, nil)
	m.state = StateReleasesResults
	m.relRows = []core.Release{relRow("Mogwai", -1, true)}

	if cmd := m.handleReleasesEnter(); cmd == nil {
		t.Fatal("expected a fetch command")
	}
	if m.state != StateReleaseLoading {
		t.Errorf("state = %d, want StateReleaseLoading", m.state)
	}
	if !m.fromReleases {
		t.Error("fromReleases should be set")
	}
	if m.selectedArtist == nil || m.selectedArtist.Name != "Mogwai" {
		t.Error("selectedArtist should carry the payload artist name")
	}
	if m.selectedReleaseGroup != nil {
		t.Error("selectedReleaseGroup must stay nil until the release details bring it")
	}
	if m.selectedRelease != nil {
		t.Error("selectedRelease must stay nil until the release is chosen")
	}
}

// A refresh that shrinks the other tab leaves its cursor past the end; enter
// on the highlighted (clamped) row must still work.
func TestHandleReleasesEnter_ClampsAStaleCursor(t *testing.T) {
	m := New(&fakeDownloads{}, nil, FilterConfig{}, nil)
	rel := &fakeReleases{list: []core.Release{relRow("a", -1, true), relRow("b", -2, true), relRow("c", -3, true)}}
	m.Update(loaded(t, m.StartReleases(rel)))
	m.relCursors[relTabRecent].SetPos(2)

	rel.list = rel.list[:1]
	_, cmd := m.Update(core.WarmupFinished{})
	m.Update(loaded(t, cmd))
	m.relCursors[relTabRecent].SetPos(2) // as if clamped on the other tab only

	if cmd := m.handleReleasesEnter(); cmd == nil {
		t.Fatal("enter on the highlighted row must start the download flow")
	}
	if m.selectedArtist == nil || m.selectedArtist.Name != "a" {
		t.Error("the clamped row should be the one selected")
	}
}

func TestReset_ComesBackToTheReleasesList(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.fromReleases = true
	m.relRows = []core.Release{relRow("a", -1, true)}
	m.relTab, m.relFilter = relTabUpcoming, relFilterLibrary
	m.state = StateSlskdResults

	m.Reset()

	if m.state != StateReleasesResults {
		t.Errorf("state = %d, want StateReleasesResults", m.state)
	}
	if len(m.relRows) != 1 || m.relTab != relTabUpcoming || m.relFilter != relFilterLibrary {
		t.Error("list, tab and filter must survive Reset")
	}
}

// otherPopupsRead is the list read for another popup, which opened it.
func otherPopupsRead(t *testing.T, err error) tea.Msg {
	t.Helper()
	other := New(nil, nil, FilterConfig{}, nil)
	return loaded(t, other.StartReleases(&fakeReleases{list: []core.Release{relRow("Low", -1, true)}, err: err}))
}

// A popup that never opened the list drops every list read.
func TestReleasesLoaded_APopupThatNeverOpenedTheListDropsEveryRead(t *testing.T) {
	for name, err := range map[string]error{"read": nil, "failed read": errors.New("database locked")} {
		t.Run(name, func(t *testing.T) {
			h, m := mbPopup(&fakeDownloads{})
			h.AssertDrops(t, otherPopupsRead(t, err))
			if m.state != StateSearch || len(m.relRows) != 0 || m.relErr != "" {
				t.Errorf("state %d, %d rows, error %q: want the search input, no rows", m.state, len(m.relRows), m.relErr)
			}
		})
	}
}

// listingReleases is a popup on the list of rel, read.
func listingReleases(t *testing.T, rel *fakeReleases) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m := mbPopup(&fakeDownloads{})
	h.SendMsg(loaded(t, m.StartReleases(rel)))
	if m.state != StateReleasesResults {
		t.Fatalf("state = %d, want the list", m.state)
	}
	return h, m
}

// A popup on the list drops the reads of another popup, and its own read
// delivered again.
func TestReleasesLoaded_APopupOnTheListDropsTheReadsItDoesNotWaitFor(t *testing.T) {
	rel := &fakeReleases{list: []core.Release{relRow("Mogwai", -1, true)}}
	h, m := mbPopup(&fakeDownloads{})
	own := loaded(t, m.StartReleases(rel))

	h.AssertDrops(t, otherPopupsRead(t, nil))
	h.AssertDrops(t, otherPopupsRead(t, errors.New("database locked")))
	if m.state != StateReleasesLoading {
		t.Fatalf("state = %d, want the list loading", m.state)
	}

	h.SendMsg(own)
	if m.state != StateReleasesResults || len(m.relRows) != 1 {
		t.Fatalf("state %d with %d rows, want its own list", m.state, len(m.relRows))
	}
	h.AssertDrops(t, otherPopupsRead(t, nil))
	h.AssertDrops(t, own)
}

// A read made before a refresh, delivered after the read that follows the
// refresh, does not put the old rows back.
func TestReleasesLoaded_AnEarlierReadDeliveredLateIsDropped(t *testing.T) {
	rel := &fakeReleases{list: []core.Release{relRow("Mogwai", -1, true)}}
	h, m := listingReleases(t, rel)

	_, before := m.Update(core.WarmupFinished{})
	earlier := loaded(t, before)
	rel.list = []core.Release{relRow("Mogwai", -1, true), relRow("Low", -2, true)}
	_, after := m.Update(core.ReleasesRefreshed{})

	h.AssertDrops(t, earlier)
	h.SendMsg(loaded(t, after))
	if len(m.relRows) != 2 {
		t.Fatalf("rows = %d, want the list read after the refresh", len(m.relRows))
	}
	h.AssertDrops(t, earlier)
}

// The read of the list outlives a download queued from it: Reset does not
// end it.
func TestReleasesLoaded_ResetKeepsWaitingForTheList(t *testing.T) {
	rel := &fakeReleases{list: []core.Release{relRow("Mogwai", -1, true)}}
	_, m := listingReleases(t, rel)
	rel.list = append(rel.list, relRow("Low", -2, true))
	_, cmd := m.Update(core.WarmupFinished{})
	m.fromReleases = true

	m.Reset()
	m.Update(loaded(t, cmd))

	if len(m.relRows) != 2 {
		t.Errorf("rows = %d, want the read asked before Reset taken", len(m.relRows))
	}
}

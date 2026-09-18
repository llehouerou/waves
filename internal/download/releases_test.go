package download

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/releases"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// relRow builds a row dated daysFromToday days from today.
func relRow(artist string, daysFromToday int, inLibrary bool) ReleaseRow {
	return ReleaseRow{Release: releases.Release{
		ReleaseGroupMBID: artist,
		ArtistCreditName: artist,
		ReleaseName:      artist + " album",
		ReleaseDate:      time.Now().AddDate(0, 0, daysFromToday).Format(time.DateOnly),
		PrimaryType:      "Album",
		InLibrary:        inLibrary,
	}}
}

func TestVisibleReleases_TabsFilterAndOrder(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)
	m.relRows = []ReleaseRow{
		relRow("old", -10, true),
		relRow("yesterday", -1, false),
		relRow("today", 0, true),
		relRow("soon", 2, false),
		relRow("later", 20, true),
	}

	tests := []struct {
		name   string
		tab    int
		filter int
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
	m := New("", "", FilterConfig{}, nil)
	m.state = StateReleasesResults
	m.relDiscoveries = true
	m.relErr = "Could not refresh releases: Get \"https://api.listenbrainz.org\": dial tcp: lookup failed"
	m.statusMsg = "Download queued"

	long := relRow("A Very Long Artist Name Indeed", 0, false)
	long.Seeds = []string{"Stereolab", "Broadcast", "Portishead", "Cate Le Bon"}
	cjk := relRow("\u30b8\u30e3\u30d1\u30cb\u30fc\u30ba\u30fb\u30a2\u30fc\u30c6\u30a3\u30b9\u30c8", -1, true)
	m.relRows = []ReleaseRow{long, cjk}

	for _, style := range []string{"nerd", "unicode", "none"} {
		icons.Init(style)
		for _, box := range []int{100, 78, 58, 44, 30} {
			m.SetSize(box, 20)
			budget := m.releasesWidth()
			panel := m.renderReleases() + "\n" + m.releasesHelp(budget)
			for line := range strings.SplitSeq(panel, "\n") {
				if w := lipgloss.Width(line); w > budget {
					t.Errorf("%s at %d cols: line is %d wide (budget %d): %q", style, box, w, budget, line)
				}
			}
		}
	}
	icons.Init("nerd")
}

func TestMarkRows_FlagsOwnedAndDownloading(t *testing.T) {
	matched := []releases.Release{
		{ReleaseGroupMBID: "rg-1", ArtistCreditName: "Mogwai", ReleaseName: "The Bad Fire", InLibrary: true},
		{ReleaseGroupMBID: "rg-2", ArtistCreditName: "Jane Weaver", ReleaseName: "Love", InLibrary: false},
	}

	rows := markRows(matched, nil, map[string]bool{"rg-2": true})
	if rows[0].Downloading {
		t.Error("rg-1 has no active download")
	}
	if !rows[1].Downloading {
		t.Error("rg-2 is downloading and should be marked")
	}

	// A downloading release wears the queued icon and the muted row style.
	icons.Init("none")
	defer icons.Init("nerd")
	if line := renderReleaseRow(rows[1], 60, false, rows[1].Downloading); !strings.Contains(line, icons.Queued()) {
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
	m := New("", "", FilterConfig{}, nil)
	m.state = StateReleasesResults
	m.relRows = []ReleaseRow{relRow("Mogwai", -1, true), relRow("Radiohead", -1, true)}

	h := testutil.NewPopupHarness(m)
	h.SendKey("/")
	h.SendKey("m")
	h.SendKey("o")
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

func TestReleasesStart_KeepsCursorWithinLineBudget(t *testing.T) {
	// Five rows on five distinct days: each costs a date header plus a row.
	rows := []ReleaseRow{
		relRow("a", -1, true), relRow("b", -2, true), relRow("c", -3, true),
		relRow("d", -4, true), relRow("e", -5, true),
	}

	// A 6-line budget fits 3 rows: the cursor on the last row scrolls the list.
	if got := releasesStart(rows, 0, 4, 6); got != 2 {
		t.Errorf("start = %d, want 2", got)
	}
	// A budget large enough keeps the list anchored at the offset.
	if got := releasesStart(rows, 0, 4, 20); got != 0 {
		t.Errorf("start = %d, want 0", got)
	}
}

func TestHandleReleasesEnter_RefusesWithoutSlskd(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)
	m.state = StateReleasesResults
	m.relRows = []ReleaseRow{relRow("a", -1, true)}

	if cmd := m.handleReleasesEnter(); cmd != nil {
		t.Error("expected no command without slskd configured")
	}
	if m.errorMsg != slskdMissingMsg {
		t.Errorf("errorMsg = %q, want the slskd configuration message", m.errorMsg)
	}
	if m.state != StateReleasesResults {
		t.Error("state should stay on the releases list")
	}
}

func TestHandleReleasesEnter_SynthesisesMusicBrainzContext(t *testing.T) {
	m := New("http://localhost:5030", "key", FilterConfig{}, nil)
	m.state = StateReleasesResults
	row := relRow("Mogwai", -1, true)
	m.relRows = []ReleaseRow{row}

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
	if m.selectedReleaseGroup == nil || m.selectedReleaseGroup.ID != row.ReleaseGroupMBID {
		t.Error("selectedReleaseGroup should carry the cached release group MBID")
	}
	if m.selectedRelease != nil {
		t.Error("selectedRelease must stay nil until the release is chosen")
	}
}

func TestHandleReleasesLoaded_OnlyARefreshEndsTheRefresh(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)
	m.state = StateReleasesLoading
	m.relRefreshing = true

	// A warm-up reload arrives while the ListenBrainz refresh still runs.
	m.handleReleasesLoaded(ReleasesLoadedMsg{Rows: []ReleaseRow{relRow("a", -1, true)}})
	if !m.relRefreshing {
		t.Error("a warm-up reload must not end the refresh")
	}
	if m.state != StateReleasesResults {
		t.Error("the list should be displayed once rows arrive")
	}

	m.handleReleasesLoaded(ReleasesLoadedMsg{Rows: m.relRows, Refreshed: true})
	if m.relRefreshing {
		t.Error("a refresh outcome must end the refresh")
	}
}

func TestReset_ComesBackToTheReleasesList(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)
	m.fromReleases = true
	m.relRows = []ReleaseRow{relRow("a", -1, true)}
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

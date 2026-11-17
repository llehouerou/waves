package download

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// unpluggedRow is a row of the new releases: a live reissue whose release
// name and date are not those of its release group.
func unpluggedRow() core.Release {
	row := relRow("Nirvana", -1, false)
	row.ReleaseGroupMBID = "rg-unplugged"
	row.ReleaseName = "Unplugged (25th)"
	row.SecondaryType = "Live"
	return row
}

// unpluggedGroup is the release group MusicBrainz gives with the release
// details of unpluggedRow.
func unpluggedGroup() *core.MBReleaseGroup {
	return &core.MBReleaseGroup{
		ID:             "rg-unplugged",
		Title:          "Unplugged",
		PrimaryType:    "Album",
		SecondaryTypes: []string{"Live"},
		FirstRelease:   "1994-11-01",
		Genres:         []string{"grunge", "acoustic rock"},
	}
}

// unpluggedRelease is the release of unpluggedRow the user picks.
var unpluggedRelease = core.MBRelease{ID: "rel-unplugged", Title: "MTV Unplugged", Date: "2019-11-01", TrackCount: 1}

// The requests of the releases of unpluggedRow's group, and of the details of
// unpluggedRelease.
var (
	unpluggedReleases = mbworkflow.Request{Kind: mbworkflow.ReleaseGroupReleases, Of: "rg-unplugged"}
	unpluggedDetails  = mbworkflow.Request{Kind: mbworkflow.ReleaseDetails, Of: "rel-unplugged"}
)

// nirvanaUnplugged is the slskd response to the search of the group's title.
func nirvanaUnplugged() []core.SlskdResponse {
	return []core.SlskdResponse{{
		Username: "bob", HasFreeSlot: true,
		Files: []core.SlskdFile{{Filename: `@@bob\Nirvana - Unplugged\01.flac`, Size: 3}},
	}}
}

// pickedFromReleases is a popup opened on the new releases, whose user
// entered unpluggedRow and picked its release: the release details are being
// loaded.
func pickedFromReleases(t *testing.T, dls *fakeDownloads) (*Model, *testutil.PopupHarness) {
	t.Helper()
	m := New(dls, nil, FilterConfig{}, nil)
	m.SetSize(100, 40)
	m.Update(loaded(t, m.StartReleases(&fakeReleases{list: []core.Release{unpluggedRow()}})))
	h := testutil.NewPopupHarness(m)
	h.SendKey("enter")
	m.Update(mbworkflow.ReleasesResultMsg{Request: unpluggedReleases, Releases: []core.MBRelease{unpluggedRelease}})
	h.SendKey("enter")
	if m.state != StateReleaseDetailsLoading {
		t.Fatalf("state = %d, want StateReleaseDetailsLoading", m.state)
	}
	return m, h
}

// queue runs Enter on the highlighted slskd result and hands its outcome to
// the popup.
func queue(t *testing.T, m *Model, h *testutil.PopupHarness) {
	t.Helper()
	h.SendKey("enter")
	cmd := h.LastCommand()
	if cmd == nil {
		t.Fatal("Enter on a result queued nothing")
	}
	msg, ok := cmd().(SlskdDownloadQueuedMsg)
	if !ok {
		t.Fatal("Enter on a result did not queue a download")
	}
	m.Update(msg)
}

// From the new releases, the download carries the release group MusicBrainz
// gave with the release details, never one made from the row.
func TestFromReleases_QueuesTheReleaseGroupOfTheReleaseDetails(t *testing.T) {
	dls := &fakeDownloads{}
	m, h := pickedFromReleases(t, dls)
	// Until the details arrive, the header tells the row it was entered with.
	header := testutil.FindLine(testutil.StripANSI(m.View()), "② Release")
	if want := "Unplugged (25th) (" + unpluggedRow().ReleaseDate[:4] + ")"; !strings.Contains(header, want) {
		t.Errorf("header before the details = %q, want the row's release name and year %q", header, want)
	}

	group := unpluggedGroup()
	m.Update(mbworkflow.ReleaseDetailsResultMsg{Request: unpluggedDetails, Details: &core.MBReleaseDetails{Release: unpluggedRelease, ReleaseGroup: group}})

	if !slices.Equal(dls.searched, []string{"Nirvana Unplugged"}) {
		t.Fatalf("searched %q, want the row's artist and the group's title", dls.searched)
	}
	m.Update(core.SearchFinished{Query: "Nirvana Unplugged", Responses: nirvanaUnplugged()})
	queue(t, m, h)

	if len(dls.queued) != 1 {
		t.Fatalf("queued %d downloads, want 1", len(dls.queued))
	}
	d := dls.queued[0]
	if !reflect.DeepEqual(d.MBReleaseGroup, unpluggedGroup()) {
		t.Errorf("release group = %+v, want the one of the release details %+v", d.MBReleaseGroup, unpluggedGroup())
	}
	if d.MBReleaseGroupID != "rg-unplugged" || d.MBAlbumTitle != "Unplugged" || d.MBReleaseYear != "1994-11-01" {
		t.Errorf("ID %q, title %q, year %q; want the group's", d.MBReleaseGroupID, d.MBAlbumTitle, d.MBReleaseYear)
	}
	if d.MBArtistName != "Nirvana" {
		t.Errorf("artist = %q, want the row's", d.MBArtistName)
	}

	// Back on the list, the row just queued is marked.
	if m.state != StateReleasesResults {
		t.Fatalf("state = %d, want back on the new releases", m.state)
	}
	line := testutil.FindLine(testutil.StripANSI(m.View()), "Unplugged (25th)")
	if !strings.Contains(line, icons.Queued()) {
		t.Errorf("row %q is not marked as queued", line)
	}
}

// From the artist search, the download carries the group of the release
// details too: the one picked from the list, as MusicBrainz details it.
func TestFromArtist_QueuesTheReleaseGroupOfTheReleaseDetails(t *testing.T) {
	dls := &fakeDownloads{}
	h, m := mbPopup(dls)
	listReleases(t, h, m)

	stepTo(t, h, m, h.SendEnter(), StateSlskdSearching) // the details, then the slskd search
	m.Update(core.SearchFinished{Query: "Nirvana Unplugged", Responses: nirvanaUnplugged()})
	queue(t, m, h)

	if len(dls.queued) != 1 || !reflect.DeepEqual(dls.queued[0].MBReleaseGroup, unpluggedGroup()) {
		t.Errorf("queued %+v, want one download carrying the group of the release details", dls.queued)
	}
}

// Release details without a release group stop the flow on the release list,
// on either route.
func TestReleaseDetailsWithoutAGroup_StopOnTheReleaseList(t *testing.T) {
	routes := map[string]func(t *testing.T, dls *fakeDownloads) (*Model, *testutil.PopupHarness){
		"new releases": pickedFromReleases,
		"artist": func(t *testing.T, dls *fakeDownloads) (*Model, *testutil.PopupHarness) {
			t.Helper()
			m := New(dls, nil, FilterConfig{}, nil)
			m.SetSize(100, 40)
			m.selectedArtist = &core.MBArtist{Name: "Nirvana"}
			m.selectedReleaseGroup = &core.MBReleaseGroup{ID: "rg-unplugged", Title: "Unplugged"}
			m.releasesRaw = []core.MBRelease{unpluggedRelease}
			m.reapplyReleaseFilters()
			m.state = StateReleaseResults
			h := testutil.NewPopupHarness(m)
			h.SendKey("enter")
			return m, h
		},
	}
	for name, open := range routes {
		t.Run(name, func(t *testing.T) {
			dls := &fakeDownloads{}
			m, h := open(t, dls)

			m.Update(mbworkflow.ReleaseDetailsResultMsg{Request: unpluggedDetails, Details: &core.MBReleaseDetails{Release: unpluggedRelease}})

			if m.state != StateReleaseResults || m.errorMsg == "" {
				t.Errorf("state %d, error %q; want the release list with an error", m.state, m.errorMsg)
			}
			if len(dls.searched) != 0 {
				t.Errorf("searched %q, want no slskd search", dls.searched)
			}
			h.SendKey("enter")
			if len(dls.queued) != 0 {
				t.Errorf("queued %+v, want nothing", dls.queued)
			}
		})
	}
}

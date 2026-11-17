package download

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeMB is MusicBrainz as the download popup sees it: the query "nirvana"
// finds the artist "nirvana", whose release group is "rg-unplugged" (listed
// as an album only: the release details tell it is live, as unpluggedGroup),
// whose release is unpluggedRelease. Any other request fails.
type fakeMB struct {
	mbworkflow.Client
}

var errUnavailable = errors.New("unavailable")

func (fakeMB) SearchArtists(query string) ([]core.MBArtist, error) {
	if query != "nirvana" {
		return nil, errUnavailable
	}
	return []core.MBArtist{{ID: "nirvana", Name: "Nirvana"}}, nil
}

func (fakeMB) GetArtistReleaseGroups(artistID string) ([]core.MBReleaseGroup, error) {
	if artistID != "nirvana" {
		return nil, errUnavailable
	}
	return []core.MBReleaseGroup{{ID: "rg-unplugged", Title: "Unplugged", PrimaryType: "Album"}}, nil
}

func (fakeMB) GetReleaseGroupReleases(releaseGroupID string) ([]core.MBRelease, error) {
	if releaseGroupID != "rg-unplugged" {
		return nil, errUnavailable
	}
	return []core.MBRelease{unpluggedRelease}, nil
}

func (fakeMB) GetRelease(releaseID string) (*core.MBReleaseDetails, error) {
	if releaseID != "rel-unplugged" {
		return nil, errUnavailable
	}
	return &core.MBReleaseDetails{Release: unpluggedRelease, ReleaseGroup: unpluggedGroup()}, nil
}

// mbPopup is a fresh download popup on fakeMB and dls.
func mbPopup(dls *fakeDownloads) (*testutil.PopupHarness, *Model) {
	m := New(dls, fakeMB{}, FilterConfig{}, nil)
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m), m
}

// asked is the request whose command the popup returned: it sends it and
// hands back its result, not delivered.
func asked(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("the popup asked MusicBrainz nothing")
	}
	return cmd()
}

// stepTo delivers the result of the request cmd sent and checks the popup is
// at state then.
func stepTo(t *testing.T, h *testutil.PopupHarness, m *Model, cmd tea.Cmd, state State) {
	t.Helper()
	h.SendMsg(asked(t, cmd))
	if m.state != state {
		t.Fatalf("state = %d, want %d", m.state, state)
	}
}

// The steps of the flow from the artist search: each takes a popup at the
// step before to its own, delivering what MusicBrainz answered.

func searchNirvana(t *testing.T, h *testutil.PopupHarness, m *Model) {
	t.Helper()
	h.SendKey("nirvana")
	stepTo(t, h, m, h.SendEnter(), StateArtistResults)
}

func listGroups(t *testing.T, h *testutil.PopupHarness, m *Model) {
	t.Helper()
	searchNirvana(t, h, m)
	stepTo(t, h, m, h.SendEnter(), StateReleaseGroupResults)
}

func listReleases(t *testing.T, h *testutil.PopupHarness, m *Model) {
	t.Helper()
	listGroups(t, h, m)
	stepTo(t, h, m, h.SendEnter(), StateReleaseResults)
}

func searchSlskd(t *testing.T, h *testutil.PopupHarness, m *Model) {
	t.Helper()
	listReleases(t, h, m)
	stepTo(t, h, m, h.SendEnter(), StateSlskdSearching)
}

// foreignResults are MusicBrainz results of each kind the download popup
// takes, one that succeeds and one that fails, for the requests of the flow
// about Nirvana: a popup that did not ask for them, or took them already,
// drops them.
func foreignResults() map[string]tea.Msg {
	artists := mbworkflow.Request{Kind: mbworkflow.SearchArtists, Of: "nirvana"}
	groups := mbworkflow.Request{Kind: mbworkflow.ArtistReleaseGroups, Of: "nirvana"}
	releases := mbworkflow.Request{Kind: mbworkflow.ReleaseGroupReleases, Of: "rg-unplugged"}
	details := mbworkflow.Request{Kind: mbworkflow.ReleaseDetails, Of: "rel-unplugged"}
	return map[string]tea.Msg{
		"artists":                mbworkflow.ArtistSearchResultMsg{Request: artists, Artists: []core.MBArtist{{ID: "nirvana", Name: "Nirvana"}}},
		"failed artists":         mbworkflow.ArtistSearchResultMsg{Request: artists, Err: errUnavailable},
		"release groups":         mbworkflow.SearchResultMsg{Request: groups, ReleaseGroups: []core.MBReleaseGroup{*unpluggedGroup()}},
		"failed release groups":  mbworkflow.SearchResultMsg{Request: groups, Err: errUnavailable},
		"releases":               mbworkflow.ReleasesResultMsg{Request: releases, Releases: []core.MBRelease{unpluggedRelease}},
		"failed releases":        mbworkflow.ReleasesResultMsg{Request: releases, Err: errUnavailable},
		"release details":        mbworkflow.ReleaseDetailsResultMsg{Request: details, Details: &core.MBReleaseDetails{Release: unpluggedRelease, ReleaseGroup: unpluggedGroup()}},
		"failed release details": mbworkflow.ReleaseDetailsResultMsg{Request: details, Err: errUnavailable},
	}
}

// assertDrops checks that the popup drops msg: it starts nothing, asks the
// downloads nothing, and shows what it showed.
func assertDrops(t *testing.T, h *testutil.PopupHarness, m *Model, dls *fakeDownloads, msg tea.Msg) {
	t.Helper()
	state, view, searched := m.state, testutil.StripANSI(h.View()), len(dls.searched)
	var cmd tea.Cmd
	if r := testutil.PanicOf(func() { cmd = h.SendMsg(msg) }); r != nil {
		t.Fatalf("the popup panics: %v", r)
	}
	if cmd != nil {
		t.Errorf("the popup started %#v", cmd())
	}
	if m.state != state {
		t.Errorf("state = %d, want it left at %d", m.state, state)
	}
	if len(dls.searched) != searched {
		t.Errorf("searched slskd for %q", dls.searched[searched:])
	}
	if got := testutil.StripANSI(h.View()); got != view {
		t.Errorf("the view changed:\n%s\nwant:\n%s", got, view)
	}
}

func TestDownload_APopupWaitingForNoResultDropsEveryResult(t *testing.T) {
	popups := map[string]func(t *testing.T, h *testutil.PopupHarness, m *Model){
		"search input":   func(*testing.T, *testutil.PopupHarness, *Model) {},
		"artists":        searchNirvana,
		"release groups": listGroups,
		"releases":       listReleases,
		"slskd":          searchSlskd,
		"new releases": func(t *testing.T, _ *testutil.PopupHarness, m *Model) {
			t.Helper()
			m.Update(loaded(t, m.StartReleases(&fakeReleases{list: []core.Release{unpluggedRow()}})))
		},
	}
	for name, open := range popups {
		for result, msg := range foreignResults() {
			t.Run(name+"/"+result, func(t *testing.T) {
				dls := &fakeDownloads{}
				h, m := mbPopup(dls)
				open(t, h, m)
				assertDrops(t, h, m, dls, msg)
			})
		}
	}
}

// Each request is waited for while results of other requests of its kind
// come: they are dropped, and its own is taken.
func TestDownload_WaitingForOneRequestDropsAnotherOfItsKind(t *testing.T) {
	tests := []struct {
		name   string
		open   func(t *testing.T, h *testutil.PopupHarness, m *Model) tea.Cmd
		others []tea.Msg
		then   State
	}{
		{
			name: "artist search",
			open: func(_ *testing.T, h *testutil.PopupHarness, _ *Model) tea.Cmd {
				h.SendKey("nirvana")
				return h.SendEnter()
			},
			others: []tea.Msg{
				mbworkflow.ArtistSearchResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.SearchArtists, Of: "low"}, Artists: []core.MBArtist{{ID: "low", Name: "Low"}}},
				mbworkflow.ArtistSearchResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.SearchArtists, Of: "low"}, Err: errUnavailable},
			},
			then: StateArtistResults,
		},
		{
			name: "release groups",
			open: func(t *testing.T, h *testutil.PopupHarness, m *Model) tea.Cmd {
				t.Helper()
				searchNirvana(t, h, m)
				return h.SendEnter()
			},
			others: []tea.Msg{
				mbworkflow.SearchResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.ArtistReleaseGroups, Of: "low"}, ReleaseGroups: []core.MBReleaseGroup{{ID: "rg-trust", Title: "Trust", PrimaryType: "Album"}}},
				mbworkflow.SearchResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.ArtistReleaseGroups, Of: "low"}, Err: errUnavailable},
				// A search for the same text is not the artist's release groups.
				mbworkflow.SearchResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.SearchReleaseGroups, Of: "nirvana"}, ReleaseGroups: []core.MBReleaseGroup{{ID: "rg-trust", Title: "Trust", PrimaryType: "Album"}}},
			},
			then: StateReleaseGroupResults,
		},
		{
			name: "releases",
			open: func(t *testing.T, h *testutil.PopupHarness, m *Model) tea.Cmd {
				t.Helper()
				listGroups(t, h, m)
				return h.SendEnter()
			},
			others: []tea.Msg{
				mbworkflow.ReleasesResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.ReleaseGroupReleases, Of: "rg-trust"}, Releases: []core.MBRelease{{ID: "rel-trust", TrackCount: 11}}},
				mbworkflow.ReleasesResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.ReleaseGroupReleases, Of: "rg-trust"}, Err: errUnavailable},
			},
			then: StateReleaseResults,
		},
		{
			name: "release details",
			open: func(t *testing.T, h *testutil.PopupHarness, m *Model) tea.Cmd {
				t.Helper()
				listReleases(t, h, m)
				return h.SendEnter()
			},
			others: []tea.Msg{
				mbworkflow.ReleaseDetailsResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.ReleaseDetails, Of: "rel-trust"}, Details: &core.MBReleaseDetails{Release: core.MBRelease{ID: "rel-trust"}, ReleaseGroup: &core.MBReleaseGroup{ID: "rg-trust", Title: "Trust"}}},
				mbworkflow.ReleaseDetailsResultMsg{Request: mbworkflow.Request{Kind: mbworkflow.ReleaseDetails, Of: "rel-trust"}, Err: errUnavailable},
			},
			then: StateSlskdSearching,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dls := &fakeDownloads{}
			h, m := mbPopup(dls)
			own := tt.open(t, h, m)
			for _, other := range tt.others {
				assertDrops(t, h, m, dls, other)
			}
			stepTo(t, h, m, own, tt.then)
		})
	}
}

// From the new releases, the releases of the row's release group are waited
// for: those of another group are dropped.
func TestDownload_FromTheNewReleasesWaitsForTheReleasesOfTheRow(t *testing.T) {
	dls := &fakeDownloads{}
	h, m := mbPopup(dls)
	m.Update(loaded(t, m.StartReleases(&fakeReleases{list: []core.Release{unpluggedRow()}})))
	own := h.SendEnter()

	other := mbworkflow.Request{Kind: mbworkflow.ReleaseGroupReleases, Of: "rg-trust"}
	assertDrops(t, h, m, dls, mbworkflow.ReleasesResultMsg{Request: other, Releases: []core.MBRelease{{ID: "rel-trust", TrackCount: 11}}})
	assertDrops(t, h, m, dls, mbworkflow.ReleasesResultMsg{Request: other, Err: errUnavailable})

	stepTo(t, h, m, own, StateReleaseResults)
	stepTo(t, h, m, h.SendEnter(), StateSlskdSearching)
}

// A result delivered twice is taken once.
func TestDownload_AResultDeliveredTwiceIsTakenOnce(t *testing.T) {
	dls := &fakeDownloads{}
	h, m := mbPopup(dls)
	listReleases(t, h, m)
	details := asked(t, h.SendEnter())
	h.SendMsg(details)
	if len(dls.searched) != 1 {
		t.Fatalf("searched slskd %d times, want once", len(dls.searched))
	}

	assertDrops(t, h, m, dls, details)
}

package download

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeMB is MusicBrainz as the download popup sees it: the query "nirvana"
// finds the artist "nirvana", whose release group is "rg-unplugged" (listed
// as an album only: the release details tell it is live, as unpluggedGroup),
// whose release is unpluggedRelease. Any other request fails, and every
// request fails while it is down. The details of a release are details
// instead, if set.
type fakeMB struct {
	mbworkflow.Client
	down    bool
	details *core.MBReleaseDetails
}

var errUnavailable = errors.New("unavailable")

func (f fakeMB) SearchArtists(query string) ([]core.MBArtist, error) {
	if f.down || query != "nirvana" {
		return nil, errUnavailable
	}
	return []core.MBArtist{{ID: "nirvana", Name: "Nirvana"}}, nil
}

func (f fakeMB) GetArtistReleaseGroups(artistID string) ([]core.MBReleaseGroup, error) {
	if f.down || artistID != "nirvana" {
		return nil, errUnavailable
	}
	return []core.MBReleaseGroup{{ID: "rg-unplugged", Title: "Unplugged", PrimaryType: "Album"}}, nil
}

func (f fakeMB) GetReleaseGroupReleases(releaseGroupID string) ([]core.MBRelease, error) {
	if f.down || releaseGroupID != "rg-unplugged" {
		return nil, errUnavailable
	}
	return []core.MBRelease{unpluggedRelease}, nil
}

func (f fakeMB) GetRelease(releaseID string) (*core.MBReleaseDetails, error) {
	if f.down {
		return nil, errUnavailable
	}
	if f.details != nil {
		return f.details, nil
	}
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

// closedPopupsResults are MusicBrainz answers of each kind the download
// popup takes, one that succeeds and one that fails, each to a request of the
// flow about Nirvana made by a popup closed before it came: a popup that did
// not ask for them drops them.
func closedPopupsResults(t *testing.T) map[string]tea.Msg {
	t.Helper()
	results := map[string]tea.Msg{}
	steps := map[string]func(t *testing.T, h *testutil.PopupHarness, m *Model){
		"artists": func(_ *testing.T, h *testutil.PopupHarness, _ *Model) {
			h.SendKey("nirvana")
		},
		"release groups":  searchNirvana,
		"releases":        listGroups,
		"release details": listReleases,
	}
	for kind, step := range steps {
		for prefix, down := range map[string]bool{"": false, "failed": true} {
			h, m := mbPopup(&fakeDownloads{})
			step(t, h, m)
			m.mbClient = fakeMB{down: down}
			results[strings.TrimSpace(prefix+" "+kind)] = asked(t, h.SendEnter())
			h.SendEscape()
		}
	}
	return results
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
	results := closedPopupsResults(t)
	for name, open := range popups {
		for result, msg := range results {
			t.Run(name+"/"+result, func(t *testing.T) {
				dls := &fakeDownloads{}
				h, m := mbPopup(dls)
				open(t, h, m)
				assertDrops(t, h, m, dls, msg)
			})
		}
	}
}

// Each request is waited for while the answers of a popup closed since, and
// the answers to the requests this popup made before, come: they are
// dropped, and its own is taken.
func TestDownload_WaitingForOneRequestDropsEveryOtherAnswer(t *testing.T) {
	closed := closedPopupsResults(t)
	dls := &fakeDownloads{}
	h, m := mbPopup(dls)
	h.SendKey("nirvana")
	earlier := make([]tea.Msg, 0, 4)
	for _, then := range []State{StateArtistResults, StateReleaseGroupResults, StateReleaseResults, StateSlskdSearching} {
		own := h.SendEnter()
		for name, msg := range closed {
			t.Run(name, func(t *testing.T) { assertDrops(t, h, m, dls, msg) })
		}
		for _, msg := range earlier {
			assertDrops(t, h, m, dls, msg)
		}
		msg := asked(t, own)
		h.SendMsg(msg)
		if m.state != then {
			t.Fatalf("state = %d, want %d", m.state, then)
		}
		earlier = append(earlier, msg)
	}
}

// From the new releases, the releases of the row's release group are waited
// for: those of another group are dropped.
func TestDownload_FromTheNewReleasesWaitsForTheReleasesOfTheRow(t *testing.T) {
	dls := &fakeDownloads{}
	h, m := mbPopup(dls)
	m.Update(loaded(t, m.StartReleases(&fakeReleases{list: []core.Release{unpluggedRow()}})))
	own := h.SendEnter()

	closed := closedPopupsResults(t)
	assertDrops(t, h, m, dls, closed["releases"])
	assertDrops(t, h, m, dls, closed["failed releases"])

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

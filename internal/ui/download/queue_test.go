package download

import (
	"errors"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// The highlighted slskd result is recorded against the selected release.
func TestSelectedDownload(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.state = StateSlskdResults
	rg := &musicbrainz.ReleaseGroup{ID: "rg", Title: "Album", FirstRelease: "1999"}
	m.selectedArtist = &musicbrainz.Artist{Name: "Artist"}
	m.selectedReleaseGroup = rg
	m.selectedRelease = &musicbrainz.Release{ID: "rel"}
	details := &musicbrainz.ReleaseDetails{ReleaseGroup: rg}
	m.selectedReleaseDetails = details
	m.slskdResults = []core.SlskdResult{
		{Username: "alice", Directory: `@@alice\A`},
		{Username: "bob", Directory: `@@bob\B`, Files: []slskd.File{{Filename: `@@bob\B\01.flac`, Size: 3}}},
	}
	m.slskdCursor.SetPos(1)

	got, ok := m.selectedDownload()

	want := downloads.Download{
		MBReleaseGroupID: "rg", MBReleaseID: "rel", MBArtistName: "Artist",
		MBAlbumTitle: "Album", MBReleaseYear: "1999", MBReleaseGroup: rg, MBReleaseDetails: details,
		SlskdUsername: "bob", SlskdDirectory: `@@bob\B`,
		Files: []downloads.DownloadFile{{Filename: `@@bob\B\01.flac`, Size: 3}},
	}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("selectedDownload() = %+v, %v\nwant %+v", got, ok, want)
	}

	// Enter queues exactly that download.
	if cmd := m.handleSlskdEnter(); cmd == nil || m.state != StateDownloading {
		t.Errorf("Enter: cmd %v, state %d; want a queue command and StateDownloading", cmd != nil, m.state)
	}

	for name, clear := range map[string]func(){
		"artist":        func() { m.selectedArtist = nil },
		"release group": func() { m.selectedReleaseGroup = nil },
	} {
		clear()
		if _, ok := m.selectedDownload(); ok {
			t.Errorf("recorded a download without a %s", name)
		}
	}
	m.state = StateSlskdResults
	if cmd := m.handleSlskdEnter(); cmd != nil {
		t.Error("Enter queued a download it can't record")
	}
}

// queueingPopup is searchingPopup on its results, whose user pressed Enter
// on bob's folder: it returns the queueing, its answer not delivered.
func queueingPopup(t *testing.T, dls *fakeDownloads) (*testutil.PopupHarness, *Model, tea.Cmd) {
	t.Helper()
	m := searchingPopup(t, dls)
	m.Update(core.SearchFinished{Query: "Low Trust", Responses: lowTrust()})
	h := testutil.NewPopupHarness(m)
	h.ClearCommands()
	cmd := h.SendEnter()
	if m.state != StateDownloading || cmd == nil {
		t.Fatalf("state = %d, want the download queueing", m.state)
	}
	return h, m, cmd
}

// closedPopupsQueueings are the outcomes of the queueing of bob's folder by a
// popup closed before they came: one accepted, one refused.
func closedPopupsQueueings(t *testing.T) map[string]tea.Msg {
	t.Helper()
	outcomes := map[string]tea.Msg{}
	for name, err := range map[string]error{"queued": nil, "refused": errors.New("folder taken")} {
		h, _, cmd := queueingPopup(t, &fakeDownloads{queueErr: err})
		outcomes[name] = cmd()
		h.SendEscape()
	}
	return outcomes
}

// A refused queue keeps the user on the results: the app tells the error.
func TestDownloadQueued_ARefusalGoesBackToTheResults(t *testing.T) {
	h, m, cmd := queueingPopup(t, &fakeDownloads{queueErr: errors.New("refused")})

	if cmd := h.SendMsg(cmd()); cmd != nil {
		t.Errorf("the popup started %#v", cmd())
	}
	if m.state != StateSlskdResults {
		t.Errorf("state = %d, want the results", m.state)
	}
}

// An accepted queue tells the app, which syncs the recorded download.
func TestDownloadQueued_TellsTheApp(t *testing.T) {
	h, m, cmd := queueingPopup(t, &fakeDownloads{})

	cmd = h.SendMsg(cmd())
	if cmd == nil {
		t.Fatal("no action after queueing")
	}
	if msg, ok := cmd().(action.Msg); !ok || msg.Action != (Queued{}) {
		t.Errorf("action = %#v, want Queued", cmd())
	}
	if !m.IsDownloadComplete() {
		t.Error("popup not marked complete")
	}
}

// The outcome of a queueing carries the download it is about.
func TestDownloadQueued_SaysWhichDownload(t *testing.T) {
	_, _, cmd := queueingPopup(t, &fakeDownloads{queueErr: errors.New("refused")})
	msg, ok := cmd().(SlskdDownloadQueuedMsg)
	if !ok || msg.Artist != "Low" || msg.Album != "Trust" {
		t.Errorf("outcome = %#v, want the queueing of Low - Trust", msg)
	}
}

// #298: the outcome of the queueing of a popup closed since leaves a popup
// that does not wait for it as it is.
func TestDownloadQueued_APopupDropsTheOutcomeOfAnother(t *testing.T) {
	outcomes := closedPopupsQueueings(t)
	popups := map[string]func(t *testing.T) (*testutil.PopupHarness, *Model){
		"search input": func(*testing.T) (*testutil.PopupHarness, *Model) { return mbPopup(&fakeDownloads{}) },
		"results": func(t *testing.T) (*testutil.PopupHarness, *Model) {
			t.Helper()
			m := searchingPopup(t, &fakeDownloads{})
			m.Update(core.SearchFinished{Query: "Low Trust", Responses: lowTrust()})
			return testutil.NewPopupHarness(m), m
		},
		"queueing": func(t *testing.T) (*testutil.PopupHarness, *Model) {
			t.Helper()
			h, m, _ := queueingPopup(t, &fakeDownloads{})
			return h, m
		},
	}
	for name, open := range popups {
		for outcome, msg := range outcomes {
			t.Run(name+"/"+outcome, func(t *testing.T) {
				h, m := open(t)
				state := m.state
				h.AssertDrops(t, msg)
				if m.state != state || m.IsDownloadComplete() {
					t.Errorf("state = %d, complete %v; want it left at %d", m.state, m.IsDownloadComplete(), state)
				}
			})
		}
	}
}

// #298: a popup taken back to its results with Backspace while it queued no
// longer waits: the outcome leaves it there.
func TestDownloadQueued_BackspaceStopsWaiting(t *testing.T) {
	for name, err := range map[string]error{"queued": nil, "refused": errors.New("folder taken")} {
		t.Run(name, func(t *testing.T) {
			h, m, cmd := queueingPopup(t, &fakeDownloads{queueErr: err})
			h.SendSpecialKey(tea.KeyBackspace)
			if m.state != StateSlskdResults {
				t.Fatalf("state = %d, want the results", m.state)
			}

			h.AssertDrops(t, cmd())
			if m.state != StateSlskdResults || m.IsDownloadComplete() {
				t.Errorf("state = %d, complete %v; want the results", m.state, m.IsDownloadComplete())
			}
		})
	}
}

// An outcome delivered twice is taken once.
func TestDownloadQueued_AnOutcomeDeliveredTwiceIsTakenOnce(t *testing.T) {
	h, m, cmd := queueingPopup(t, &fakeDownloads{queueErr: errors.New("refused")})
	msg := cmd()
	h.SendMsg(msg)
	h.SendDown()

	h.AssertDrops(t, msg)
	if m.state != StateSlskdResults {
		t.Errorf("state = %d, want the results", m.state)
	}
}

// From the releases list, the popup goes back to it to queue more, with the
// release marked as queued.
func TestDownloadQueued_FromReleases(t *testing.T) {
	m, h, details := pickedFromReleases(t, &fakeDownloads{}, fakeMB{})
	h.ExecuteAndSend(details)
	m.Update(core.SearchFinished{Query: "Nirvana Unplugged", Responses: nirvanaUnplugged()})

	_, cmd := h.ExecuteAndSend(h.SendEnter())

	if cmd == nil {
		t.Fatal("no action after queueing")
	}
	if msg, ok := cmd().(action.Msg); !ok || msg.Action != (Queued{}) {
		t.Errorf("action = %#v, want Queued", cmd())
	}
	if !m.relQueued["rg-unplugged"] || m.IsDownloadComplete() {
		t.Errorf("relQueued = %v, complete = %v; want marked, not complete", m.relQueued, m.IsDownloadComplete())
	}
}

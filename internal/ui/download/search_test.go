package download

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
)

// fakeDownloads is the downloads domain as the popup sees it: slskd is
// configured, and what is asked of it is recorded.
type fakeDownloads struct {
	searched  []string
	searchErr error
	queued    []core.Download
	queueErr  error
}

func (f *fakeDownloads) Configured() bool { return true }

func (f *fakeDownloads) Search(query string) error {
	f.searched = append(f.searched, query)
	return f.searchErr
}

func (f *fakeDownloads) Queue(d core.Download) (id int64, err error) {
	f.queued = append(f.queued, d)
	return 1, f.queueErr
}

// searchingPopup is a popup whose user picked a release of Low's "Trust":
// the slskd search has just been asked for.
func searchingPopup(t *testing.T, dls *fakeDownloads) *Model {
	t.Helper()
	m := New(dls, nil, FilterConfig{}, nil)
	m.SetSize(100, 40)
	m.selectedArtist = &core.MBArtist{Name: "Low"}
	m.selectedReleaseGroup = &core.MBReleaseGroup{ID: "rg", Title: "Trust"}
	m.state = StateReleaseDetailsLoading
	m.Update(mbworkflow.ReleaseDetailsResultMsg{Details: &core.MBReleaseDetails{ReleaseGroup: m.selectedReleaseGroup}})
	return m
}

// lowTrust is a response to the search of searchingPopup: one FLAC of bob's.
func lowTrust() []core.SlskdResponse {
	return []core.SlskdResponse{{
		Username: "bob", HasFreeSlot: true,
		Files: []core.SlskdFile{{Filename: `@@bob\Low - Trust\01.flac`, Size: 3}},
	}}
}

func TestSearch_PickingAReleaseAsksTheDownloadsForASearch(t *testing.T) {
	dls := &fakeDownloads{}
	m := searchingPopup(t, dls)

	if !slices.Equal(dls.searched, []string{"Low Trust"}) {
		t.Errorf("searched %v, want Low Trust once", dls.searched)
	}
	if m.state != StateSlskdSearching {
		t.Errorf("state = %d, want StateSlskdSearching", m.state)
	}
}

func TestSearch_ShowsWhereTheCoreSaysTheSearchIs(t *testing.T) {
	m := searchingPopup(t, &fakeDownloads{})

	for _, tt := range []struct {
		event core.SearchProgressed
		want  string
	}{
		{core.SearchProgressed{Phase: "searching", State: "InProgress", Responses: 2}, "Searching Soulseek (searching) - 2 users responded"},
		{core.SearchProgressed{Phase: "searching", State: "InProgress", Responses: 4, Elapsed: 12 * time.Second}, "Searching Soulseek (searching) - 4 users (12s)"},
		{core.SearchProgressed{Phase: "collecting", State: "Completed", Responses: 5}, "Collecting results... (5 users responded)"},
		{core.SearchProgressed{Phase: "fetching", State: "Completed", Responses: 5}, "Waiting for results... (5 users)"},
	} {
		tt.event.Query = "Low Trust"
		m.Update(tt.event)
		if m.statusMsg != tt.want {
			t.Errorf("status after %+v = %q, want %q", tt.event, m.statusMsg, tt.want)
		}
	}
	if m.state != StateSlskdSearching {
		t.Errorf("state = %d, want still StateSlskdSearching", m.state)
	}
}

func TestSearch_ItsEndShowsTheResults(t *testing.T) {
	m := searchingPopup(t, &fakeDownloads{})

	m.Update(core.SearchFinished{Query: "Low Trust", Responses: lowTrust()})

	if m.state != StateSlskdResults {
		t.Fatalf("state = %d, want StateSlskdResults", m.state)
	}
	if len(m.slskdResults) != 1 || m.slskdResults[0].Username != "bob" || m.slskdResults[0].Format != "FLAC" {
		t.Errorf("results = %+v, want bob's FLAC folder", m.slskdResults)
	}
}

// A search the popup did not ask for is one a closed popup left running.
func TestSearch_IgnoresTheEventsOfAnotherSearch(t *testing.T) {
	m := searchingPopup(t, &fakeDownloads{})

	m.Update(core.SearchProgressed{Query: "Mogwai Rave Tapes", Phase: "collecting", Responses: 9})
	m.Update(core.SearchFinished{Query: "Mogwai Rave Tapes", Responses: lowTrust()})

	if m.state != StateSlskdSearching || len(m.slskdResults) != 0 {
		t.Errorf("state = %d with %d results, want still searching with none", m.state, len(m.slskdResults))
	}
	if strings.Contains(m.statusMsg, "9 users") {
		t.Errorf("status = %q, taken from another search", m.statusMsg)
	}
}

func TestSearch_AFailureGoesBackToTheReleaseGroups(t *testing.T) {
	boom := errors.New("boom")
	failures := map[string]func() *Model{
		"refused at once": func() *Model {
			return searchingPopup(t, &fakeDownloads{searchErr: boom})
		},
		"failed while running": func() *Model {
			m := searchingPopup(t, &fakeDownloads{})
			m.Update(core.SearchFinished{Query: "Low Trust", Err: boom})
			return m
		},
	}
	for name, fail := range failures {
		m := fail()
		if m.state != StateReleaseGroupResults {
			t.Errorf("%s: state = %d, want StateReleaseGroupResults", name, m.state)
		}
		if m.errorMsg != "slskd error: boom" || m.statusMsg != "" {
			t.Errorf("%s: error = %q, status = %q; want the slskd error and no status", name, m.errorMsg, m.statusMsg)
		}
	}
}

func TestQueue_GoesThroughTheDownloads(t *testing.T) {
	dls := &fakeDownloads{queueErr: errors.New("folder taken")}
	m := searchingPopup(t, dls)
	m.Update(core.SearchFinished{Query: "Low Trust", Responses: lowTrust()})

	cmd := m.handleSlskdEnter()
	if cmd == nil {
		t.Fatal("Enter on a result queued nothing")
	}
	msg := cmd()

	if len(dls.queued) != 1 || dls.queued[0].SlskdUsername != "bob" || dls.queued[0].MBReleaseGroupID != "rg" {
		t.Errorf("queued %+v, want bob's folder for release group rg", dls.queued)
	}
	if queued, ok := msg.(SlskdDownloadQueuedMsg); !ok || queued.Err == nil {
		t.Errorf("message = %#v, want the refusal of the downloads", msg)
	}
}

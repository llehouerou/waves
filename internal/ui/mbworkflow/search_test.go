package mbworkflow

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

// mockClient implements the Client interface for testing.
type mockClient struct {
	searchArtistsFunc           func(query string) ([]musicbrainz.Artist, error)
	searchReleaseGroupsFunc     func(query string) ([]musicbrainz.ReleaseGroup, error)
	getArtistReleaseGroupsFunc  func(artistID string) ([]musicbrainz.ReleaseGroup, error)
	getReleaseGroupReleasesFunc func(releaseGroupID string) ([]musicbrainz.Release, error)
	getReleaseFunc              func(mbid string) (*musicbrainz.ReleaseDetails, error)
	getCoverArtFunc             func(releaseMBID string) ([]byte, error)
}

func (m *mockClient) SearchArtists(query string) ([]musicbrainz.Artist, error) {
	if m.searchArtistsFunc != nil {
		return m.searchArtistsFunc(query)
	}
	return []musicbrainz.Artist{}, nil
}

func (m *mockClient) SearchReleaseGroups(query string) ([]musicbrainz.ReleaseGroup, error) {
	if m.searchReleaseGroupsFunc != nil {
		return m.searchReleaseGroupsFunc(query)
	}
	return []musicbrainz.ReleaseGroup{}, nil
}

func (m *mockClient) GetArtistReleaseGroups(artistID string) ([]musicbrainz.ReleaseGroup, error) {
	if m.getArtistReleaseGroupsFunc != nil {
		return m.getArtistReleaseGroupsFunc(artistID)
	}
	return []musicbrainz.ReleaseGroup{}, nil
}

func (m *mockClient) GetReleaseGroupReleases(releaseGroupID string) ([]musicbrainz.Release, error) {
	if m.getReleaseGroupReleasesFunc != nil {
		return m.getReleaseGroupReleasesFunc(releaseGroupID)
	}
	return []musicbrainz.Release{}, nil
}

func (m *mockClient) GetRelease(mbid string) (*musicbrainz.ReleaseDetails, error) {
	if m.getReleaseFunc != nil {
		return m.getReleaseFunc(mbid)
	}
	return &musicbrainz.ReleaseDetails{}, nil
}

func (m *mockClient) GetCoverArt(releaseMBID string) ([]byte, error) {
	if m.getCoverArtFunc != nil {
		return m.getCoverArtFunc(releaseMBID)
	}
	return []byte{}, nil
}

func TestFetchCoverArtCmd(t *testing.T) {
	expectedData := []byte{0x89, 0x50, 0x4E, 0x47}
	client := &mockClient{
		getCoverArtFunc: func(_ string) ([]byte, error) {
			return expectedData, nil
		},
	}

	cmd := FetchCoverArtCmd(client, "r1")
	msg := cmd()

	result, ok := msg.(CoverArtResultMsg)
	if !ok {
		t.Fatalf("expected CoverArtResultMsg, got %T", msg)
	}
	if result.ReleaseID != "r1" {
		t.Errorf("the cover is said to be of release %q, want r1", result.ReleaseID)
	}
	if len(result.Data) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(result.Data))
	}
}

func TestFetchCoverArtCmd_NoCoverArt(t *testing.T) {
	client := &mockClient{
		getCoverArtFunc: func(_ string) ([]byte, error) {
			// Cover Art Archive returns nil with no error for 404
			return nil, nil //nolint:nilnil // Matches real Cover Art Archive behavior
		},
	}

	cmd := FetchCoverArtCmd(client, "r1")
	msg := cmd()

	result, ok := msg.(CoverArtResultMsg)
	if !ok {
		t.Fatalf("expected CoverArtResultMsg, got %T", msg)
	}
	if result.ReleaseID != "r1" {
		t.Errorf("the cover is said to be of release %q, want r1", result.ReleaseID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	if result.Data != nil {
		t.Error("expected nil data for missing cover art")
	}
}

func TestFetchCoverArtCmd_Error(t *testing.T) {
	expectedErr := errors.New("network error")
	client := &mockClient{
		getCoverArtFunc: func(_ string) ([]byte, error) {
			return nil, expectedErr
		},
	}

	cmd := FetchCoverArtCmd(client, "r1")
	msg := cmd()

	result, ok := msg.(CoverArtResultMsg)
	if !ok {
		t.Fatalf("expected CoverArtResultMsg, got %T", msg)
	}
	if result.ReleaseID != "r1" {
		t.Errorf("the cover is said to be of release %q, want r1", result.ReleaseID)
	}
	if !errors.Is(result.Err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, result.Err)
	}
}

// asking is a client that knows one thing of each kind: the artist and the
// release groups of the query "low", the release groups of the artist "a",
// the releases of the group "rg" and the details of the release "r". Anything
// else is unavailable.
func asking() *mockClient {
	unavailable := errors.New("unavailable")
	return &mockClient{
		searchArtistsFunc: func(query string) ([]musicbrainz.Artist, error) {
			if query != "low" {
				return nil, unavailable
			}
			return []musicbrainz.Artist{{ID: "a", Name: "Low"}}, nil
		},
		searchReleaseGroupsFunc: func(query string) ([]musicbrainz.ReleaseGroup, error) {
			if query != "low" {
				return nil, unavailable
			}
			return []musicbrainz.ReleaseGroup{{ID: "rg-found", Title: "Trust"}}, nil
		},
		getArtistReleaseGroupsFunc: func(artistID string) ([]musicbrainz.ReleaseGroup, error) {
			if artistID != "a" {
				return nil, unavailable
			}
			return []musicbrainz.ReleaseGroup{{ID: "rg", Title: "Trust"}}, nil
		},
		getReleaseGroupReleasesFunc: func(releaseGroupID string) ([]musicbrainz.Release, error) {
			if releaseGroupID != "rg" {
				return nil, unavailable
			}
			return []musicbrainz.Release{{ID: "r", Title: "Trust"}}, nil
		},
		getReleaseFunc: func(mbid string) (*musicbrainz.ReleaseDetails, error) {
			if mbid != "r" {
				return nil, unavailable
			}
			return &musicbrainz.ReleaseDetails{Release: musicbrainz.Release{ID: "r", Title: "Trust"}}, nil
		},
	}
}

// Each kind of request asks the client what it names, and its result message
// carries what the client answered and names the request: release groups
// found by a search are not those of an artist.
func TestAsk_SendsTheRequestAndItsResultNamesIt(t *testing.T) {
	tests := []struct {
		req  Request
		want tea.Msg
	}{
		{
			Request{Kind: SearchArtists, Of: "low"},
			ArtistSearchResultMsg{Request: Request{Kind: SearchArtists, Of: "low"}, Artists: []musicbrainz.Artist{{ID: "a", Name: "Low"}}},
		},
		{
			Request{Kind: SearchReleaseGroups, Of: "low"},
			SearchResultMsg{Request: Request{Kind: SearchReleaseGroups, Of: "low"}, ReleaseGroups: []musicbrainz.ReleaseGroup{{ID: "rg-found", Title: "Trust"}}},
		},
		{
			Request{Kind: ArtistReleaseGroups, Of: "a"},
			SearchResultMsg{Request: Request{Kind: ArtistReleaseGroups, Of: "a"}, ReleaseGroups: []musicbrainz.ReleaseGroup{{ID: "rg", Title: "Trust"}}},
		},
		{
			Request{Kind: ReleaseGroupReleases, Of: "rg"},
			ReleasesResultMsg{Request: Request{Kind: ReleaseGroupReleases, Of: "rg"}, Releases: []musicbrainz.Release{{ID: "r", Title: "Trust"}}},
		},
		{
			Request{Kind: ReleaseDetails, Of: "r"},
			ReleaseDetailsResultMsg{Request: Request{Kind: ReleaseDetails, Of: "r"}, Details: &musicbrainz.ReleaseDetails{Release: musicbrainz.Release{ID: "r", Title: "Trust"}}},
		},
	}
	for _, tt := range tests {
		var p Pending
		if got := p.Ask(asking(), tt.req)(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("asking %+v: %#v, want %#v", tt.req, got, tt.want)
		}
	}
}

// A request the client fails: its result names it, with the error.
func TestAsk_AFailedRequestNamesItsRequest(t *testing.T) {
	for _, kind := range []RequestKind{SearchArtists, SearchReleaseGroups, ArtistReleaseGroups, ReleaseGroupReleases, ReleaseDetails} {
		req := Request{Kind: kind, Of: "unknown"}
		var p Pending
		got, err := answered(p.Ask(asking(), req)())
		if got != req || err == nil {
			t.Errorf("asking %+v: answers %+v with error %v, want it with an error", req, got, err)
		}
	}
}

// answered is the request a result message names, and its error.
func answered(msg tea.Msg) (req Request, err error) {
	switch msg := msg.(type) {
	case ArtistSearchResultMsg:
		return msg.Request, msg.Err
	case SearchResultMsg:
		return msg.Request, msg.Err
	case ReleasesResultMsg:
		return msg.Request, msg.Err
	case ReleaseDetailsResultMsg:
		return msg.Request, msg.Err
	}
	return Request{}, fmt.Errorf("%T answers no request", msg)
}

func TestPending_TakesTheResultOfTheRequestWaitedForOnce(t *testing.T) {
	var p Pending
	asked := Request{Kind: ReleaseDetails, Of: "r1"}
	if p.Take(asked) {
		t.Error("a pending that waits for nothing took a result")
	}
	if p.Take(Request{}) {
		t.Error("a pending that waits for nothing took a result that names no request")
	}

	if cmd := p.Ask(&mockClient{}, asked); cmd == nil {
		t.Fatal("asking sends nothing")
	}
	for _, other := range []Request{
		{Kind: ReleaseDetails, Of: "r2"},
		{Kind: ReleaseGroupReleases, Of: "r1"},
		{},
	} {
		if p.Take(other) {
			t.Errorf("waiting for %+v, took %+v", asked, other)
		}
	}
	if !p.Take(asked) {
		t.Fatalf("waiting for %+v, did not take its result", asked)
	}
	if p.Take(asked) {
		t.Error("the same result was taken twice")
	}
}

// Asking sends the request asked, whose result names it, and waits for that
// one only, in place of the one asked before.
func TestPending_AsksTheRequestAndWaitsForItInPlaceOfTheOneBefore(t *testing.T) {
	var p Pending
	before := Request{Kind: ReleaseGroupReleases, Of: "rg"}
	p.Ask(&mockClient{}, before)

	asked := Request{Kind: ArtistReleaseGroups, Of: "a"}
	req, err := answered(p.Ask(asking(), asked)())
	if err != nil || req != asked {
		t.Fatalf("asking %+v sent a request answering %+v (error %v)", asked, req, err)
	}
	if p.Take(before) {
		t.Error("took the result of the request asked before")
	}
	if !p.Take(req) {
		t.Error("did not take the result of the request asked")
	}
}

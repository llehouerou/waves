package workflow

import (
	"errors"
	"testing"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

// Test constants.
const testReleaseGroupID2 = "rg2"

// mockClient implements the Client interface for testing.
type mockClient struct {
	searchArtistsFunc                func(query string) ([]musicbrainz.Artist, error)
	searchReleaseGroupsFunc          func(query string) ([]musicbrainz.ReleaseGroup, error)
	searchReleaseGroupsByArtistAlbum func(artist, album string) ([]musicbrainz.ReleaseGroup, error)
	getArtistReleaseGroupsFunc       func(artistID string) ([]musicbrainz.ReleaseGroup, error)
	getReleaseGroupReleasesFunc      func(releaseGroupID string) ([]musicbrainz.Release, error)
	getReleaseFunc                   func(mbid string) (*musicbrainz.ReleaseDetails, error)
	getCoverArtFunc                  func(releaseMBID string) ([]byte, error)
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

func (m *mockClient) SearchReleaseGroupsByArtistAlbum(artist, album string) ([]musicbrainz.ReleaseGroup, error) {
	if m.searchReleaseGroupsByArtistAlbum != nil {
		return m.searchReleaseGroupsByArtistAlbum(artist, album)
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

func TestNewSearchFlow(t *testing.T) {
	client := &mockClient{}
	flow := NewSearchFlow(client)

	if flow == nil {
		t.Fatal("expected non-nil SearchFlow")
	}
	if flow.client != client {
		t.Error("expected client to be set")
	}
	if flow.Query() != "" {
		t.Error("expected empty query")
	}
	if len(flow.ReleaseGroups()) != 0 {
		t.Error("expected empty release groups")
	}
	if len(flow.Releases()) != 0 {
		t.Error("expected empty releases")
	}
}

func TestSearchFlow_Search(t *testing.T) {
	expectedGroups := []musicbrainz.ReleaseGroup{
		{ID: "rg1", Title: "Album 1", Artist: "Artist 1"},
		{ID: testReleaseGroupID2, Title: "Album 2", Artist: "Artist 2"},
	}

	client := &mockClient{
		searchReleaseGroupsFunc: func(query string) ([]musicbrainz.ReleaseGroup, error) {
			if query != "test query" {
				t.Errorf("unexpected query: %s", query)
			}
			return expectedGroups, nil
		},
	}

	flow := NewSearchFlow(client)
	cmd := flow.Search("test query")

	if flow.Query() != "test query" {
		t.Errorf("expected query to be 'test query', got '%s'", flow.Query())
	}

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	msg := cmd()
	result, ok := msg.(SearchResultMsg)
	if !ok {
		t.Fatalf("expected SearchResultMsg, got %T", msg)
	}

	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	if len(result.ReleaseGroups) != 2 {
		t.Errorf("expected 2 release groups, got %d", len(result.ReleaseGroups))
	}
}

func TestSearchFlow_Search_Error(t *testing.T) {
	expectedErr := errors.New("search failed")
	client := &mockClient{
		searchReleaseGroupsFunc: func(_ string) ([]musicbrainz.ReleaseGroup, error) {
			return nil, expectedErr
		},
	}

	flow := NewSearchFlow(client)
	cmd := flow.Search("query")
	msg := cmd()

	result, ok := msg.(SearchResultMsg)
	if !ok {
		t.Fatalf("expected SearchResultMsg, got %T", msg)
	}
	if !errors.Is(result.Err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, result.Err)
	}
}

func TestSearchFlow_SearchByArtistAlbum(t *testing.T) {
	expectedGroups := []musicbrainz.ReleaseGroup{
		{ID: "rg1", Title: "Album", Artist: "Artist"},
	}

	client := &mockClient{
		searchReleaseGroupsByArtistAlbum: func(artist, album string) ([]musicbrainz.ReleaseGroup, error) {
			if artist != "Artist" || album != "Album" {
				t.Errorf("unexpected args: artist=%s, album=%s", artist, album)
			}
			return expectedGroups, nil
		},
	}

	flow := NewSearchFlow(client)
	cmd := flow.SearchByArtistAlbum("Artist", "Album")

	if flow.Query() != "Artist - Album" {
		t.Errorf("expected query to be 'Artist - Album', got '%s'", flow.Query())
	}

	msg := cmd()
	result, ok := msg.(SearchResultMsg)
	if !ok {
		t.Fatalf("expected SearchResultMsg, got %T", msg)
	}
	if len(result.ReleaseGroups) != 1 {
		t.Errorf("expected 1 release group, got %d", len(result.ReleaseGroups))
	}
}

func TestSearchFlow_FetchArtistReleaseGroups(t *testing.T) {
	expectedGroups := []musicbrainz.ReleaseGroup{
		{ID: "rg1", Title: "Album 1"},
		{ID: testReleaseGroupID2, Title: "Album 2"},
	}

	client := &mockClient{
		getArtistReleaseGroupsFunc: func(artistID string) ([]musicbrainz.ReleaseGroup, error) {
			if artistID != "artist-id" {
				t.Errorf("unexpected artistID: %s", artistID)
			}
			return expectedGroups, nil
		},
	}

	flow := NewSearchFlow(client)
	cmd := flow.FetchArtistReleaseGroups("artist-id")

	msg := cmd()
	result, ok := msg.(SearchResultMsg)
	if !ok {
		t.Fatalf("expected SearchResultMsg, got %T", msg)
	}
	if len(result.ReleaseGroups) != 2 {
		t.Errorf("expected 2 release groups, got %d", len(result.ReleaseGroups))
	}
}

func TestSearchFlow_SelectReleaseGroup(t *testing.T) {
	groups := []musicbrainz.ReleaseGroup{
		{ID: "rg1", Title: "Album 1"},
		{ID: testReleaseGroupID2, Title: "Album 2"},
	}

	flow := NewSearchFlow(&mockClient{})
	flow.SetReleaseGroups(groups)

	// Select valid index
	flow.SelectReleaseGroup(1)
	selected := flow.SelectedReleaseGroup()
	if selected == nil {
		t.Fatal("expected non-nil selected release group")
	}
	if selected.ID != testReleaseGroupID2 {
		t.Errorf("expected ID '%s', got '%s'", testReleaseGroupID2, selected.ID)
	}

	// Select invalid index (too high)
	flow.SelectReleaseGroup(10)
	// Should still be the previous selection
	if flow.SelectedReleaseGroup().ID != testReleaseGroupID2 {
		t.Error("selection should not change for invalid index")
	}

	// Select invalid index (negative)
	flow.SelectReleaseGroup(-1)
	if flow.SelectedReleaseGroup().ID != testReleaseGroupID2 {
		t.Error("selection should not change for negative index")
	}
}

func TestSearchFlow_FetchReleases(t *testing.T) {
	expectedReleases := []musicbrainz.Release{
		{ID: "r1", Title: "Release 1"},
		{ID: "r2", Title: "Release 2"},
	}

	client := &mockClient{
		getReleaseGroupReleasesFunc: func(releaseGroupID string) ([]musicbrainz.Release, error) {
			if releaseGroupID != "rg1" {
				t.Errorf("unexpected releaseGroupID: %s", releaseGroupID)
			}
			return expectedReleases, nil
		},
	}

	flow := NewSearchFlow(client)
	flow.SetReleaseGroups([]musicbrainz.ReleaseGroup{{ID: "rg1", Title: "Album"}})
	flow.SelectReleaseGroup(0)

	cmd := flow.FetchReleases()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	msg := cmd()
	result, ok := msg.(ReleasesResultMsg)
	if !ok {
		t.Fatalf("expected ReleasesResultMsg, got %T", msg)
	}
	if len(result.Releases) != 2 {
		t.Errorf("expected 2 releases, got %d", len(result.Releases))
	}
}

func TestSearchFlow_FetchReleases_NoSelection(t *testing.T) {
	flow := NewSearchFlow(&mockClient{})

	cmd := flow.FetchReleases()
	if cmd != nil {
		t.Error("expected nil command when no release group selected")
	}
}

func TestSearchFlow_SelectRelease(t *testing.T) {
	releases := []musicbrainz.Release{
		{ID: "r1", Title: "Release 1"},
		{ID: "r2", Title: "Release 2"},
	}

	flow := NewSearchFlow(&mockClient{})
	flow.SetReleases(releases)

	flow.SelectRelease(0)
	selected := flow.SelectedRelease()
	if selected == nil {
		t.Fatal("expected non-nil selected release")
	}
	if selected.ID != "r1" {
		t.Errorf("expected ID 'r1', got '%s'", selected.ID)
	}
}

func TestSearchFlow_FetchReleaseDetails(t *testing.T) {
	expectedDetails := &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{ID: "r1", Title: "Release"},
		Tracks:  []musicbrainz.Track{{Position: 1, Title: "Track 1"}},
	}

	client := &mockClient{
		getReleaseFunc: func(mbid string) (*musicbrainz.ReleaseDetails, error) {
			if mbid != "r1" {
				t.Errorf("unexpected mbid: %s", mbid)
			}
			return expectedDetails, nil
		},
	}

	flow := NewSearchFlow(client)
	flow.SetReleases([]musicbrainz.Release{{ID: "r1", Title: "Release"}})
	flow.SelectRelease(0)

	cmd := flow.FetchReleaseDetails()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	msg := cmd()
	result, ok := msg.(ReleaseDetailsResultMsg)
	if !ok {
		t.Fatalf("expected ReleaseDetailsResultMsg, got %T", msg)
	}
	if result.Details.ID != "r1" {
		t.Errorf("expected ID 'r1', got '%s'", result.Details.ID)
	}
	if len(result.Details.Tracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(result.Details.Tracks))
	}
}

func TestSearchFlow_FetchReleaseDetails_NoSelection(t *testing.T) {
	flow := NewSearchFlow(&mockClient{})

	cmd := flow.FetchReleaseDetails()
	if cmd != nil {
		t.Error("expected nil command when no release selected")
	}
}

func TestSearchFlow_FetchReleaseDetailsByID(t *testing.T) {
	expectedDetails := &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{ID: "r1", Title: "Release"},
	}

	client := &mockClient{
		getReleaseFunc: func(mbid string) (*musicbrainz.ReleaseDetails, error) {
			if mbid != "specific-id" {
				t.Errorf("unexpected mbid: %s", mbid)
			}
			return expectedDetails, nil
		},
	}

	flow := NewSearchFlow(client)
	cmd := flow.FetchReleaseDetailsByID("specific-id")

	msg := cmd()
	result, ok := msg.(ReleaseDetailsResultMsg)
	if !ok {
		t.Fatalf("expected ReleaseDetailsResultMsg, got %T", msg)
	}
	if result.Details.ID != "r1" {
		t.Errorf("expected ID 'r1', got '%s'", result.Details.ID)
	}
}

func TestSearchFlow_FetchCoverArt(t *testing.T) {
	expectedData := []byte{0x89, 0x50, 0x4E, 0x47}

	client := &mockClient{
		getCoverArtFunc: func(releaseMBID string) ([]byte, error) {
			if releaseMBID != "r1" {
				t.Errorf("unexpected releaseMBID: %s", releaseMBID)
			}
			return expectedData, nil
		},
	}

	flow := NewSearchFlow(client)
	flow.SetReleases([]musicbrainz.Release{{ID: "r1", Title: "Release"}})
	flow.SelectRelease(0)

	cmd := flow.FetchCoverArt()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	msg := cmd()
	result, ok := msg.(CoverArtResultMsg)
	if !ok {
		t.Fatalf("expected CoverArtResultMsg, got %T", msg)
	}
	if len(result.Data) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(result.Data))
	}
}

func TestSearchFlow_FetchCoverArt_NoSelection(t *testing.T) {
	flow := NewSearchFlow(&mockClient{})

	cmd := flow.FetchCoverArt()
	if cmd != nil {
		t.Error("expected nil command when no release selected")
	}
}

func TestSearchFlow_FetchCoverArtByID(t *testing.T) {
	expectedData := []byte{0x89, 0x50, 0x4E, 0x47}

	client := &mockClient{
		getCoverArtFunc: func(releaseMBID string) ([]byte, error) {
			if releaseMBID != "specific-id" {
				t.Errorf("unexpected releaseMBID: %s", releaseMBID)
			}
			return expectedData, nil
		},
	}

	flow := NewSearchFlow(client)
	cmd := flow.FetchCoverArtByID("specific-id")

	msg := cmd()
	result, ok := msg.(CoverArtResultMsg)
	if !ok {
		t.Fatalf("expected CoverArtResultMsg, got %T", msg)
	}
	if len(result.Data) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(result.Data))
	}
}

func TestSearchFlow_Selected(t *testing.T) {
	flow := NewSearchFlow(&mockClient{})

	groups := []musicbrainz.ReleaseGroup{{ID: "rg1", Title: "Album"}}
	releases := []musicbrainz.Release{{ID: "r1", Title: "Release"}}

	flow.SetReleaseGroups(groups)
	flow.SetReleases(releases)
	flow.SelectReleaseGroup(0)
	flow.SelectRelease(0)

	rg, r := flow.Selected()
	if rg == nil || r == nil {
		t.Fatal("expected non-nil release group and release")
	}
	if rg.ID != "rg1" {
		t.Errorf("expected release group ID 'rg1', got '%s'", rg.ID)
	}
	if r.ID != "r1" {
		t.Errorf("expected release ID 'r1', got '%s'", r.ID)
	}
}

func TestSearchFlow_Reset(t *testing.T) {
	flow := NewSearchFlow(&mockClient{})

	flow.SetReleaseGroups([]musicbrainz.ReleaseGroup{{ID: "rg1"}})
	flow.SetReleases([]musicbrainz.Release{{ID: "r1"}})
	flow.SelectReleaseGroup(0)
	flow.SelectRelease(0)

	// Set query by calling Search (we'll ignore the command)
	_ = flow.Search("test")

	flow.Reset()

	if flow.Query() != "" {
		t.Error("expected empty query after reset")
	}
	if len(flow.ReleaseGroups()) != 0 {
		t.Error("expected empty release groups after reset")
	}
	if len(flow.Releases()) != 0 {
		t.Error("expected empty releases after reset")
	}
	rg, r := flow.Selected()
	if rg != nil {
		t.Error("expected nil selected release group after reset")
	}
	if r != nil {
		t.Error("expected nil selected release after reset")
	}
}

// Test standalone command functions

func TestSearchCmd(t *testing.T) {
	expectedGroups := []musicbrainz.ReleaseGroup{{ID: "rg1", Title: "Album"}}
	client := &mockClient{
		searchReleaseGroupsFunc: func(_ string) ([]musicbrainz.ReleaseGroup, error) {
			return expectedGroups, nil
		},
	}

	cmd := SearchCmd(client, "test")
	msg := cmd()

	result, ok := msg.(SearchResultMsg)
	if !ok {
		t.Fatalf("expected SearchResultMsg, got %T", msg)
	}
	if len(result.ReleaseGroups) != 1 {
		t.Errorf("expected 1 release group, got %d", len(result.ReleaseGroups))
	}
}

func TestSearchByArtistAlbumCmd(t *testing.T) {
	expectedGroups := []musicbrainz.ReleaseGroup{{ID: "rg1", Title: "Album"}}
	client := &mockClient{
		searchReleaseGroupsByArtistAlbum: func(_, _ string) ([]musicbrainz.ReleaseGroup, error) {
			return expectedGroups, nil
		},
	}

	cmd := SearchByArtistAlbumCmd(client, "Artist", "Album")
	msg := cmd()

	result, ok := msg.(SearchResultMsg)
	if !ok {
		t.Fatalf("expected SearchResultMsg, got %T", msg)
	}
	if len(result.ReleaseGroups) != 1 {
		t.Errorf("expected 1 release group, got %d", len(result.ReleaseGroups))
	}
}

func TestFetchArtistReleaseGroupsCmd(t *testing.T) {
	expectedGroups := []musicbrainz.ReleaseGroup{{ID: "rg1", Title: "Album"}}
	client := &mockClient{
		getArtistReleaseGroupsFunc: func(_ string) ([]musicbrainz.ReleaseGroup, error) {
			return expectedGroups, nil
		},
	}

	cmd := FetchArtistReleaseGroupsCmd(client, "artist-id")
	msg := cmd()

	result, ok := msg.(SearchResultMsg)
	if !ok {
		t.Fatalf("expected SearchResultMsg, got %T", msg)
	}
	if len(result.ReleaseGroups) != 1 {
		t.Errorf("expected 1 release group, got %d", len(result.ReleaseGroups))
	}
}

func TestFetchReleasesCmd(t *testing.T) {
	expectedReleases := []musicbrainz.Release{{ID: "r1", Title: "Release"}}
	client := &mockClient{
		getReleaseGroupReleasesFunc: func(_ string) ([]musicbrainz.Release, error) {
			return expectedReleases, nil
		},
	}

	cmd := FetchReleasesCmd(client, "rg-id")
	msg := cmd()

	result, ok := msg.(ReleasesResultMsg)
	if !ok {
		t.Fatalf("expected ReleasesResultMsg, got %T", msg)
	}
	if len(result.Releases) != 1 {
		t.Errorf("expected 1 release, got %d", len(result.Releases))
	}
}

func TestFetchReleaseDetailsCmd(t *testing.T) {
	expectedDetails := &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{ID: "r1", Title: "Release"},
	}
	client := &mockClient{
		getReleaseFunc: func(_ string) (*musicbrainz.ReleaseDetails, error) {
			return expectedDetails, nil
		},
	}

	cmd := FetchReleaseDetailsCmd(client, "r1")
	msg := cmd()

	result, ok := msg.(ReleaseDetailsResultMsg)
	if !ok {
		t.Fatalf("expected ReleaseDetailsResultMsg, got %T", msg)
	}
	if result.Details.ID != "r1" {
		t.Errorf("expected ID 'r1', got '%s'", result.Details.ID)
	}
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
	if !errors.Is(result.Err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, result.Err)
	}
}

func TestSearchArtistsCmd(t *testing.T) {
	expectedArtists := []musicbrainz.Artist{
		{ID: "artist1", Name: "Test Artist"},
		{ID: "artist2", Name: "Another Artist"},
	}
	client := &mockClient{
		searchArtistsFunc: func(query string) ([]musicbrainz.Artist, error) {
			if query != "test artist" {
				t.Errorf("unexpected query: %s", query)
			}
			return expectedArtists, nil
		},
	}

	cmd := SearchArtistsCmd(client, "test artist")
	msg := cmd()

	result, ok := msg.(ArtistSearchResultMsg)
	if !ok {
		t.Fatalf("expected ArtistSearchResultMsg, got %T", msg)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	if len(result.Artists) != 2 {
		t.Errorf("expected 2 artists, got %d", len(result.Artists))
	}
}

func TestSearchArtistsCmd_Error(t *testing.T) {
	expectedErr := errors.New("search failed")
	client := &mockClient{
		searchArtistsFunc: func(_ string) ([]musicbrainz.Artist, error) {
			return nil, expectedErr
		},
	}

	cmd := SearchArtistsCmd(client, "query")
	msg := cmd()

	result, ok := msg.(ArtistSearchResultMsg)
	if !ok {
		t.Fatalf("expected ArtistSearchResultMsg, got %T", msg)
	}
	if !errors.Is(result.Err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, result.Err)
	}
}

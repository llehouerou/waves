package workflow

import (
	"errors"
	"testing"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

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

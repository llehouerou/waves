package musicbrainz

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// Three release groups, in the order of relevance MusicBrainz gives a search:
// the best match is the oldest.
const releaseGroupsBody = `{"release-groups": [
	{"id": "beatles", "title": "Abbey Road", "first-release-date": "1969-09-26"},
	{"id": "various", "title": "Abbey Road", "first-release-date": "2012-01-01"},
	{"id": "single", "title": "Abbey Road", "first-release-date": "2026-01-01"}
]}`

func releaseGroupsClient() *Client {
	return &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(releaseGroupsBody))}, nil
	})}}
}

func groupIDs(groups []ReleaseGroup) string {
	ids := make([]string, 0, len(groups))
	for i := range groups {
		ids = append(ids, groups[i].ID)
	}
	return strings.Join(ids, ",")
}

func TestSearchReleaseGroups_KeepsTheOrderOfRelevance(t *testing.T) {
	t.Parallel()
	groups, err := releaseGroupsClient().SearchReleaseGroups("Beatles Abbey Road")
	if err != nil {
		t.Fatalf("SearchReleaseGroups: %v", err)
	}
	if got, want := groupIDs(groups), "beatles,various,single"; got != want {
		t.Errorf("release groups = %s, want %s", got, want)
	}
}

func TestSearchReleaseGroupsByArtistAlbum_KeepsTheOrderOfRelevance(t *testing.T) {
	t.Parallel()
	groups, err := releaseGroupsClient().SearchReleaseGroupsByArtistAlbum("Beatles", "Abbey Road")
	if err != nil {
		t.Fatalf("SearchReleaseGroupsByArtistAlbum: %v", err)
	}
	if got, want := groupIDs(groups), "beatles,various,single"; got != want {
		t.Errorf("release groups = %s, want %s", got, want)
	}
}

func TestGetArtistReleaseGroups_ListsTheNewestFirst(t *testing.T) {
	t.Parallel()
	groups, err := releaseGroupsClient().GetArtistReleaseGroups("artist")
	if err != nil {
		t.Fatalf("GetArtistReleaseGroups: %v", err)
	}
	if got, want := groupIDs(groups), "single,various,beatles"; got != want {
		t.Errorf("release groups = %s, want %s", got, want)
	}
}

// The browse endpoint of release groups gives their artist credit only to a
// request that asks for it, as the real MusicBrainz does.
func TestGetArtistReleaseGroups_CarriesTheArtistOfEachGroup(t *testing.T) {
	t.Parallel()
	c := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		credit := ""
		if slices.Contains(strings.Fields(req.URL.Query().Get("inc")), "artist-credits") {
			credit = `, "artist-credit": [{"name": "Low", "joinphrase": ""}]`
		}
		body := `{"release-groups": [{"id": "rg", "title": "Trust", "first-release-date": "2002-09-30"` + credit + `}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}

	groups, err := c.GetArtistReleaseGroups("artist")
	if err != nil {
		t.Fatalf("GetArtistReleaseGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].Artist != "Low" {
		t.Errorf("release groups = %+v, want Trust by Low", groups)
	}
}

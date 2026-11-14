package musicbrainz

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// A release lookup embeds the release group of the release: the release
// details carry all of it, as a browsed release group does.
func TestGetRelease_CarriesTheReleaseGroupOfTheRelease(t *testing.T) {
	const body = `{
		"id": "reissue", "title": "Live at the Rainbow", "date": "2011-11-08",
		"release-group": {
			"id": "rg", "title": "Live at the Rainbow",
			"primary-type": "Album", "secondary-types": ["Live"],
			"first-release-date": "1975-09-12",
			"genres": [{"name": "rock"}, {"name": "progressive rock"}],
			"artist-credit": [{"name": "Band", "joinphrase": ""}]
		}
	}`
	c := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}

	details, err := c.GetRelease("reissue")
	if err != nil {
		t.Fatalf("GetRelease: %v", err)
	}

	group := details.ReleaseGroup
	if group == nil {
		t.Fatal("the release details carry no release group")
	}
	if group.ID != "rg" || group.Title != "Live at the Rainbow" || group.PrimaryType != "Album" ||
		group.FirstRelease != "1975-09-12" || group.Artist != "Band" {
		t.Errorf("release group = %+v, want rg, Live at the Rainbow, Album, first released 1975-09-12, by Band", group)
	}
	if !slices.Equal(group.SecondaryTypes, []string{"Live"}) {
		t.Errorf("secondary types = %v, want [Live]", group.SecondaryTypes)
	}
	if !slices.Equal(group.Genres, []string{"rock", "progressive rock"}) {
		t.Errorf("genres = %v, want [rock progressive rock]", group.Genres)
	}
	if details.ReleaseType != "Album" {
		t.Errorf("release type = %q, want Album", details.ReleaseType)
	}
}

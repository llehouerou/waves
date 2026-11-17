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

// A track of a release carries the ID of each artist it credits, in the order
// MusicBrainz gives them; a credit with no ID adds none.
func TestGetRelease_TracksCarryTheIDsOfTheirArtists(t *testing.T) {
	const body = `{
		"id": "r", "title": "Split",
		"media": [{"position": 1, "track-count": 3, "tracks": [
			{"id": "t1", "position": 1, "title": "Both", "artist-credit": [
				{"name": "A", "joinphrase": " & ", "artist": {"id": "idA", "name": "A"}},
				{"name": "B", "joinphrase": "", "artist": {"id": "idB", "name": "B"}}]},
			{"id": "t2", "position": 2, "title": "One", "artist-credit": [
				{"name": "A", "joinphrase": "", "artist": {"id": "idA", "name": "A"}}]},
			{"id": "t3", "position": 3, "title": "Unknown", "artist-credit": [
				{"name": "A", "joinphrase": " & ", "artist": {"id": "idA", "name": "A"}},
				{"name": "X", "joinphrase": "", "artist": {"name": "X"}}]}
		]}]
	}`
	c := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}

	details, err := c.GetRelease("r")
	if err != nil {
		t.Fatalf("GetRelease: %v", err)
	}

	want := [][]string{{"idA", "idB"}, {"idA"}, {"idA"}}
	if len(details.Tracks) != len(want) {
		t.Fatalf("%d tracks, want %d", len(details.Tracks), len(want))
	}
	for i, track := range details.Tracks {
		if !slices.Equal(track.ArtistIDs, want[i]) {
			t.Errorf("track %q: artist IDs = %v, want %v", track.Title, track.ArtistIDs, want[i])
		}
	}
}

package mbworkflow

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/ui/request"
)

// mockClient implements the Client interface for testing.
type mockClient struct {
	searchArtistsFunc           func(query string) ([]musicbrainz.Artist, error)
	searchReleaseGroupsFunc     func(query string) ([]musicbrainz.ReleaseGroup, error)
	getArtistReleaseGroupsFunc  func(artistID string) (musicbrainz.Browsed[musicbrainz.ReleaseGroup], error)
	getReleaseGroupReleasesFunc func(releaseGroupID string) (musicbrainz.Browsed[musicbrainz.Release], error)
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

func (m *mockClient) GetArtistReleaseGroups(artistID string) (musicbrainz.Browsed[musicbrainz.ReleaseGroup], error) {
	if m.getArtistReleaseGroupsFunc != nil {
		return m.getArtistReleaseGroupsFunc(artistID)
	}
	return musicbrainz.Browsed[musicbrainz.ReleaseGroup]{}, nil
}

func (m *mockClient) GetReleaseGroupReleases(releaseGroupID string) (musicbrainz.Browsed[musicbrainz.Release], error) {
	if m.getReleaseGroupReleasesFunc != nil {
		return m.getReleaseGroupReleasesFunc(releaseGroupID)
	}
	return musicbrainz.Browsed[musicbrainz.Release]{}, nil
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

// asking is a client that knows one thing of each kind: the artist and the
// release groups of the query "low", the release groups of the artist "a",
// the releases of the group "rg", the details and the cover of the release
// "r". It also knows the artist "many" and the group "big", of which
// MusicBrainz has more than were read. Anything else is unavailable.
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
		getArtistReleaseGroupsFunc: func(artistID string) (musicbrainz.Browsed[musicbrainz.ReleaseGroup], error) {
			switch artistID {
			case "a":
				return musicbrainz.Browsed[musicbrainz.ReleaseGroup]{Items: []musicbrainz.ReleaseGroup{{ID: "rg", Title: "Trust"}}, Total: 1}, nil
			case "many":
				return musicbrainz.Browsed[musicbrainz.ReleaseGroup]{Items: []musicbrainz.ReleaseGroup{{ID: "rg", Title: "Trust"}}, Total: 6235}, nil
			}
			return musicbrainz.Browsed[musicbrainz.ReleaseGroup]{}, unavailable
		},
		getReleaseGroupReleasesFunc: func(releaseGroupID string) (musicbrainz.Browsed[musicbrainz.Release], error) {
			switch releaseGroupID {
			case "rg":
				return musicbrainz.Browsed[musicbrainz.Release]{Items: []musicbrainz.Release{{ID: "r", Title: "Trust"}}, Total: 1}, nil
			case "big":
				return musicbrainz.Browsed[musicbrainz.Release]{Items: []musicbrainz.Release{{ID: "r", Title: "Trust"}}, Total: 2500}, nil
			}
			return musicbrainz.Browsed[musicbrainz.Release]{}, unavailable
		},
		getReleaseFunc: func(mbid string) (*musicbrainz.ReleaseDetails, error) {
			if mbid != "r" {
				return nil, unavailable
			}
			return &musicbrainz.ReleaseDetails{Release: musicbrainz.Release{ID: "r", Title: "Trust"}}, nil
		},
		getCoverArtFunc: func(releaseMBID string) ([]byte, error) {
			if releaseMBID != "r" {
				return nil, unavailable
			}
			return []byte("cover of r"), nil
		},
	}
}

// ask sends a request through a fresh Pending: it returns its answer and
// the ID of the request.
func ask(build func(request.ID) tea.Cmd) (tea.Msg, request.ID) {
	var p request.Pending
	var asked request.ID
	msg := p.Ask(func(id request.ID) tea.Cmd {
		asked = id
		return build(id)
	})()
	return msg, asked
}

// Each request asks the client what it names, and its answer carries what
// the client answered and the ID of the request.
func TestRequests_AskTheClientAndCarryTheirID(t *testing.T) {
	client := asking()
	tests := []struct {
		name  string
		build func(request.ID) tea.Cmd
		want  func(request.ID) tea.Msg
	}{
		{"artists", SearchArtists(client, "low"), func(id request.ID) tea.Msg {
			return ArtistSearchResultMsg{Request: id, Artists: []musicbrainz.Artist{{ID: "a", Name: "Low"}}}
		}},
		{"release groups found", SearchReleaseGroups(client, "low"), func(id request.ID) tea.Msg {
			return SearchResultMsg{Request: id, ReleaseGroups: []musicbrainz.ReleaseGroup{{ID: "rg-found", Title: "Trust"}}}
		}},
		{"release groups of an artist", ArtistReleaseGroups(client, "a"), func(id request.ID) tea.Msg {
			return SearchResultMsg{Request: id, ReleaseGroups: []musicbrainz.ReleaseGroup{{ID: "rg", Title: "Trust"}}}
		}},
		{"release groups of an artist, cut", ArtistReleaseGroups(client, "many"), func(id request.ID) tea.Msg {
			return SearchResultMsg{
				Request: id, ReleaseGroups: []musicbrainz.ReleaseGroup{{ID: "rg", Title: "Trust"}},
				Cut: Cut{Read: 1, Total: 6235},
			}
		}},
		{"releases", ReleaseGroupReleases(client, "rg"), func(id request.ID) tea.Msg {
			return ReleasesResultMsg{Request: id, Releases: []musicbrainz.Release{{ID: "r", Title: "Trust"}}}
		}},
		{"releases, cut", ReleaseGroupReleases(client, "big"), func(id request.ID) tea.Msg {
			return ReleasesResultMsg{
				Request: id, Releases: []musicbrainz.Release{{ID: "r", Title: "Trust"}},
				Cut: Cut{Read: 1, Total: 2500},
			}
		}},
		{"release details", ReleaseDetails(client, "r"), func(id request.ID) tea.Msg {
			return ReleaseDetailsResultMsg{Request: id, Details: &musicbrainz.ReleaseDetails{Release: musicbrainz.Release{ID: "r", Title: "Trust"}}}
		}},
		{"cover", CoverArt(client, "r"), func(id request.ID) tea.Msg {
			return CoverArtResultMsg{Request: id, Data: []byte("cover of r")}
		}},
	}
	for _, tt := range tests {
		got, id := ask(tt.build)
		if want := tt.want(id); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %#v, want %#v", tt.name, got, want)
		}
	}
}

// A request the client fails: its answer carries its ID, with the error.
func TestRequests_AFailedRequestCarriesItsID(t *testing.T) {
	client := asking()
	for _, build := range []func(request.ID) tea.Cmd{
		SearchArtists(client, "unknown"),
		SearchReleaseGroups(client, "unknown"),
		ArtistReleaseGroups(client, "unknown"),
		ReleaseGroupReleases(client, "unknown"),
		ReleaseDetails(client, "unknown"),
		CoverArt(client, "unknown"),
	} {
		msg, id := ask(build)
		got, err := answered(msg)
		if got != id || err == nil {
			t.Errorf("%T answers %+v with error %v, want %+v with an error", msg, got, err, id)
		}
	}
}

// answered is the ID of the request a message answers, and its error.
func answered(msg tea.Msg) (id request.ID, err error) {
	switch msg := msg.(type) {
	case ArtistSearchResultMsg:
		return msg.Request, msg.Err
	case SearchResultMsg:
		return msg.Request, msg.Err
	case ReleasesResultMsg:
		return msg.Request, msg.Err
	case ReleaseDetailsResultMsg:
		return msg.Request, msg.Err
	case CoverArtResultMsg:
		return msg.Request, msg.Err
	}
	return request.ID{}, fmt.Errorf("%T answers no request", msg)
}

// A cut list is told with both numbers; a list read in full is not told.
func TestCut_NoticeTellsBothNumbers(t *testing.T) {
	if got, want := (Cut{Read: 2000, Total: 6235}).Notice("release groups"), "2000 of 6235 release groups read from MusicBrainz"; got != want {
		t.Errorf("notice = %q, want %q", got, want)
	}
	if got := (Cut{}).Notice("releases"); got != "" {
		t.Errorf("notice of a list read in full = %q, want none", got)
	}
}

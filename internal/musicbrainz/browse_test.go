package musicbrainz

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// pagingMB is a fake MusicBrainz that answers a browse request as the real
// one does: at most limit entities from offset, with the number of entities
// it has in every response. Entity i is named e<i> and was first released in
// year 1000+i, so that MusicBrainz gives them oldest first.
type pagingMB struct {
	entities int // the entities it has
	reported int // the number of entities it says it has, entities when zero
	failAt   int // the offset it answers with an error status, none when zero
	emptyAt  int // the offset from which it answers no entity, none when zero

	requests []url.Values
	times    []time.Time
}

// browseKind is one of the two browse requests: what the request is asked
// with, the keys of the response, and the call of the client.
type browseKind struct {
	name     string
	filter   string // the parameter naming what is browsed
	inc      string
	list     string // the key of the entities in the response
	count    string // the key of their number
	dateKey  string
	call     func(c *Client) ([]string, int, error)
	callName string
}

var browseKinds = []browseKind{
	{
		name: "release groups", filter: "artist", inc: "genres artist-credits",
		list: "release-groups", count: "release-group-count", dateKey: "first-release-date",
		callName: "GetArtistReleaseGroups",
		call: func(c *Client) ([]string, int, error) {
			groups, err := c.GetArtistReleaseGroups("browsed")
			ids := make([]string, 0, len(groups.Items))
			for i := range groups.Items {
				ids = append(ids, groups.Items[i].ID)
			}
			return ids, groups.Total, err
		},
	},
	{
		name: "releases", filter: "release-group", inc: "media",
		list: "releases", count: "release-count", dateKey: "date",
		callName: "GetReleaseGroupReleases",
		call: func(c *Client) ([]string, int, error) {
			releases, err := c.GetReleaseGroupReleases("browsed")
			ids := make([]string, 0, len(releases.Items))
			for i := range releases.Items {
				ids = append(ids, releases.Items[i].ID)
			}
			return ids, releases.Total, err
		},
	},
}

func (f *pagingMB) client(kind browseKind) *Client {
	return &Client{httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		f.requests = append(f.requests, query)
		f.times = append(f.times, time.Now())

		offset, _ := strconv.Atoi(query.Get("offset"))
		limit, _ := strconv.Atoi(query.Get("limit"))
		if f.failAt != 0 && offset == f.failAt {
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("bad"))}, nil
		}
		reported := f.reported
		if reported == 0 {
			reported = f.entities
		}
		end := min(offset+limit, f.entities)
		if f.emptyAt != 0 && offset >= f.emptyAt {
			end = offset
		}
		entities := make([]string, 0, max(end-offset, 0))
		for i := offset; i < end; i++ {
			entities = append(entities, fmt.Sprintf(`{"id": "e%d", %q: "%d-01-01"}`, i, kind.dateKey, 1000+i))
		}
		body := fmt.Sprintf(`{%q: %d, "offset": %d, %q: [%s]}`,
			kind.count, reported, offset, kind.list, strings.Join(entities, ","))
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
}

// newestFirst names the entities from..to-1 of a pagingMB, newest first.
func newestFirst(from, to int) string {
	ids := make([]string, 0, to-from)
	for i := to - 1; i >= from; i-- {
		ids = append(ids, fmt.Sprintf("e%d", i))
	}
	return strings.Join(ids, ",")
}

func TestBrowse_ReadsEveryPageAndSortsTheWholeList(t *testing.T) {
	for _, kind := range browseKinds {
		t.Run(kind.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &pagingMB{entities: 250}
				ids, total, err := kind.call(f.client(kind))
				if err != nil {
					t.Fatalf("%s: %v", kind.callName, err)
				}
				if got, want := strings.Join(ids, ","), newestFirst(0, 250); got != want {
					t.Errorf("%s = %s, want %s", kind.name, got, want)
				}
				if total != 250 {
					t.Errorf("total = %d, want 250", total)
				}
				if len(f.requests) != 3 {
					t.Fatalf("%d requests, want 3", len(f.requests))
				}
				for i, query := range f.requests {
					if offset := query.Get("offset"); offset != strconv.Itoa(100*i) && (i != 0 || offset != "") {
						t.Errorf("request %d: offset = %q, want %d", i, offset, 100*i)
					}
					if query.Get("limit") != "100" || query.Get(kind.filter) != "browsed" || query.Get("inc") != kind.inc {
						t.Errorf("request %d = %v, want limit 100, %s browsed, inc %q", i, query, kind.filter, kind.inc)
					}
				}
			})
		})
	}
}

func TestBrowse_AListOfAPageAtMostTakesOneRequest(t *testing.T) {
	for _, kind := range browseKinds {
		for _, entities := range []int{0, 37, 100} {
			t.Run(fmt.Sprintf("%s/%d", kind.name, entities), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					f := &pagingMB{entities: entities}
					ids, total, err := kind.call(f.client(kind))
					if err != nil {
						t.Fatalf("%s: %v", kind.callName, err)
					}
					if len(ids) != entities || total != entities {
						t.Errorf("%d %s of %d, want %d of %d", len(ids), kind.name, total, entities, entities)
					}
					if len(f.requests) != 1 {
						t.Errorf("%d requests, want 1", len(f.requests))
					}
				})
			})
		}
	}
}

func TestBrowse_WaitsASecondBetweenTwoPages(t *testing.T) {
	for _, kind := range browseKinds {
		t.Run(kind.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &pagingMB{entities: 250}
				if _, _, err := kind.call(f.client(kind)); err != nil {
					t.Fatalf("%s: %v", kind.callName, err)
				}
				for i := 1; i < len(f.times); i++ {
					if gap := f.times[i].Sub(f.times[i-1]); gap < time.Second {
						t.Errorf("request %d came %v after the one before, want at least 1s", i, gap)
					}
				}
			})
		})
	}
}

func TestBrowse_APageThatFailsFailsTheCall(t *testing.T) {
	for _, kind := range browseKinds {
		t.Run(kind.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &pagingMB{entities: 250, failAt: 100}
				ids, _, err := kind.call(f.client(kind))
				if err == nil {
					t.Fatal("no error, want the error of the second page")
				}
				if len(ids) != 0 {
					t.Errorf("%d %s returned with the error, want none", len(ids), kind.name)
				}
			})
		})
	}
}

// The walk ends on an empty page, whatever MusicBrainz says it has: the list
// is told as cut, of the total MusicBrainz reported.
func TestBrowse_AnEmptyPageEndsTheWalk(t *testing.T) {
	for _, kind := range browseKinds {
		t.Run(kind.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &pagingMB{entities: 300, emptyAt: 100}
				ids, total, err := kind.call(f.client(kind))
				if err != nil {
					t.Fatalf("%s: %v", kind.callName, err)
				}
				if len(ids) != 100 || total != 300 {
					t.Errorf("%d %s of %d, want the 100 read of 300", len(ids), kind.name, total)
				}
				if len(f.requests) != 2 {
					t.Errorf("%d requests, want 2", len(f.requests))
				}
			})
		})
	}
}

func TestBrowse_StopsAtTwentyPages(t *testing.T) {
	for _, kind := range browseKinds {
		t.Run(kind.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &pagingMB{entities: 297881}
				ids, total, err := kind.call(f.client(kind))
				if err != nil {
					t.Fatalf("%s: %v", kind.callName, err)
				}
				if len(f.requests) != 20 {
					t.Errorf("%d requests, want 20", len(f.requests))
				}
				if len(ids) != 2000 || total != 297881 {
					t.Errorf("%d %s of %d, want 2000 of 297881", len(ids), kind.name, total)
				}
				if got, want := strings.Join(ids, ","), newestFirst(0, 2000); got != want {
					t.Errorf("%s are not the first 2000 MusicBrainz gave, newest first", kind.name)
				}
			})
		})
	}
}

// A search asks for the most relevant results only: one request, whatever
// MusicBrainz reports it found.
func TestSearches_SendOneRequest(t *testing.T) {
	searches := map[string]func(c *Client) error{
		"SearchArtists": func(c *Client) error {
			_, err := c.SearchArtists("low")
			return err
		},
		"SearchReleaseGroups": func(c *Client) error {
			_, err := c.SearchReleaseGroups("low trust")
			return err
		},
		"SearchReleaseGroupsByArtistAlbum": func(c *Client) error {
			_, err := c.SearchReleaseGroupsByArtistAlbum("low", "trust")
			return err
		},
	}
	for name, search := range searches {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &pagingMB{entities: 250}
				if err := search(f.client(browseKinds[0])); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if len(f.requests) != 1 {
					t.Errorf("%d requests, want 1", len(f.requests))
				}
			})
		})
	}
}

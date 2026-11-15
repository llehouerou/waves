package lastfm

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
)

// These tests swap http.DefaultTransport (lastfmtest.Serve): none is parallel.

// testBound is the shortened bound of the clients these tests build.
const testBound = 100 * time.Millisecond

var lullaby = ScrobbleTrack{Artist: "Low", Track: "Lullaby", Album: "I Could Live in Hope", Timestamp: time.Now()}

// requests are the methods of the client that send requests to Last.fm.
var requests = []struct {
	name   string
	method string // the API method of its first request
	send   func(*Client) error
}{
	{"Scrobble", "track.scrobble", func(c *Client) error { return c.Scrobble(context.Background(), lullaby) }},
	{"GetSimilarArtists", "artist.getsimilar", func(c *Client) error {
		_, err := c.GetSimilarArtists(context.Background(), "Low", 10)
		return err
	}},
	{"GetArtistTopTracks", "artist.gettoptracks", func(c *Client) error {
		_, err := c.GetArtistTopTracks("Low", 10)
		return err
	}},
	{"GetUserArtistTracks", "user.getinfo", func(c *Client) error {
		_, err := c.GetUserArtistTracks("Low", 10)
		return err
	}},
	{"GetToken", "auth.gettoken", func(c *Client) error {
		_, err := c.GetToken()
		return err
	}},
	{"GetSession", "auth.getsession", func(c *Client) error {
		_, _, err := c.GetSession("token")
		return err
	}},
	{"UpdateNowPlaying", "track.updatenowplaying", func(c *Client) error { return c.UpdateNowPlaying(lullaby) }},
	{"ScrobbleBatch", "track.scrobble", func(c *Client) error { return c.ScrobbleBatch([]ScrobbleTrack{lullaby}) }},
}

// linkedClient returns a client linked to an account, whose requests are
// bounded by bound.
func linkedClient(bound time.Duration) *Client {
	c := New("key", "secret")
	c.SetSessionKey("session")
	c.timeout = bound
	return c
}

// holding serves a Last.fm that holds every answer as hold says
// (lastfmtest.Silent or lastfmtest.Stalled) until the test ends.
func holding(t *testing.T, hold string) <-chan lastfmtest.Call {
	t.Helper()
	return lastfmtest.Serve(t, func(string, url.Values) string { return hold })
}

// reached waits for a call of method to reach the fake Last.fm: a call the
// test leaves behind must have reached it before the test ends.
func reached(t *testing.T, calls <-chan lastfmtest.Call, method string) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case call := <-calls:
			if call.Method == method {
				return
			}
		case <-timeout:
			t.Fatalf("no %s call reached Last.fm", method)
		}
	}
}

// returned runs send and returns its error, failing the test when send has
// not returned within a few seconds.
func returned(t *testing.T, send func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- send() }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("still waiting for Last.fm 5s later")
		return nil
	}
}

func TestClient_RequestsLastfmHoldsTimeOutAfterTheBound(t *testing.T) {
	holds := []struct{ name, hold string }{
		{"no headers", lastfmtest.Silent},
		{"headers but no body", lastfmtest.Stalled},
	}
	for _, h := range holds {
		for _, r := range requests {
			t.Run(h.name+"/"+r.name, func(t *testing.T) {
				calls := holding(t, h.hold)
				c := linkedClient(testBound)

				start := time.Now()
				err := returned(t, func() error { return r.send(c) })
				reached(t, calls, r.method)

				if !errors.Is(err, ErrTimeout) {
					t.Fatalf("error = %v, want ErrTimeout", err)
				}
				if elapsed := time.Since(start); elapsed < testBound {
					t.Errorf("returned after %v, before the bound of %v", elapsed, testBound)
				}
				if IsRefusal(err) {
					t.Error("a time-out is a refusal of Last.fm")
				}
				if IsSessionLost(err) {
					t.Error("a time-out loses the session")
				}
			})
		}
	}
}

func TestClient_GetUserArtistTracksBoundsItsSecondRequest(t *testing.T) {
	calls := lastfmtest.Serve(t, func(method string, _ url.Values) string {
		if method == "user.getinfo" {
			return `<lfm status="ok"><user><name>lowfan</name></user></lfm>`
		}
		return lastfmtest.Stalled
	})
	c := linkedClient(testBound)

	err := returned(t, func() error {
		_, err := c.GetUserArtistTracks("Low", 10)
		return err
	})
	reached(t, calls, "user.getartisttracks")

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("error = %v, want ErrTimeout", err)
	}
}

// GetSession keeps the session when Last.fm does not tell the username.
func TestClient_GetSessionBoundsItsSecondRequest(t *testing.T) {
	calls := lastfmtest.Serve(t, func(method string, _ url.Values) string {
		if method == "auth.getsession" {
			return `<lfm status="ok"><session><name>lowfan</name><key>fresh</key><subscriber>0</subscriber></session></lfm>`
		}
		return lastfmtest.Stalled
	})
	c := linkedClient(testBound)

	var username, key string
	err := returned(t, func() error {
		var err error
		username, key, err = c.GetSession("token")
		return err
	})
	reached(t, calls, "user.getinfo")

	if err != nil || username != "unknown" || key != "fresh" {
		t.Fatalf("GetSession = %q, %q, %v; want \"unknown\", \"fresh\", nil", username, key, err)
	}
}

func TestClient_TimeOutSaysLastfmDidNotAnswerInTime(t *testing.T) {
	calls := holding(t, lastfmtest.Stalled)
	c := linkedClient(testBound)

	err := returned(t, func() error {
		_, err := c.GetSimilarArtists(context.Background(), "Low", 10)
		return err
	})
	reached(t, calls, "artist.getsimilar")

	if err == nil || !strings.Contains(err.Error(), "no answer from Last.fm in time") {
		t.Fatalf("error = %v, want one saying Last.fm did not answer in time", err)
	}
}

func TestClient_ContextEndingBeforeTheBoundEndsTheRequest(t *testing.T) {
	sends := []struct {
		name   string
		method string
		send   func(context.Context, *Client) error
	}{
		{"Scrobble", "track.scrobble", func(ctx context.Context, c *Client) error { return c.Scrobble(ctx, lullaby) }},
		{"GetSimilarArtists", "artist.getsimilar", func(ctx context.Context, c *Client) error {
			_, err := c.GetSimilarArtists(ctx, "Low", 10)
			return err
		}},
	}
	for _, s := range sends {
		t.Run(s.name, func(t *testing.T) {
			calls := holding(t, lastfmtest.Stalled)
			c := linkedClient(time.Minute)
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(testBound, cancel)

			err := returned(t, func() error { return s.send(ctx, c) })
			reached(t, calls, s.method)

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if errors.Is(err, ErrTimeout) {
				t.Error("the end of the context is a time-out")
			}
		})
	}
}

func TestNew_BoundsEachRequestTo30Seconds(t *testing.T) {
	if got := New("key", "secret").timeout; got != 30*time.Second {
		t.Errorf("bound = %v, want 30s", got)
	}
}

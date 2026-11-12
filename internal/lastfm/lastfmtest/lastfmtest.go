// Package lastfmtest serves a fake Last.fm to the tests of the packages that
// talk to one.
package lastfmtest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Call is one request received by the fake Last.fm.
type Call struct {
	Method string
	Params url.Values
}

// Serve serves the Last.fm API from an httptest server and returns the calls
// it receives. respond maps an API method to the XML body to answer; a method
// it returns "" for gets a Last.fm error.
//
// shkh/lastfm-go hardcodes the API URL and builds its own http.Client, so the
// only seam is the default transport: tests using this cannot run in parallel.
func Serve(t *testing.T, respond func(method string, params url.Values) string) <-chan Call {
	t.Helper()
	calls := make(chan Call, 64)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		method := r.Form.Get("method")
		calls <- Call{Method: method, Params: r.Form}
		body := respond(method, r.Form)
		if body == "" {
			body = `<lfm status="failed"><error code="8">fake: not served</error></lfm>`
		}
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>` + body))
	}))
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		return orig.RoundTrip(r)
	})
	t.Cleanup(func() {
		http.DefaultTransport = orig
		srv.Close()
	})
	return calls
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

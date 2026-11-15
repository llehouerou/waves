// Package lastfmtest serves a fake Last.fm to the tests of the packages that
// talk to one.
package lastfmtest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// Call is one request received by the fake Last.fm.
type Call struct {
	Method string
	Params url.Values
}

// Unreachable, returned by respond, cuts the connection with no answer: the
// request fails as it does when Last.fm cannot be reached.
const Unreachable = "\x00unreachable"

// Returned by respond, Silent and Stalled hold Last.fm's answer until the
// test ends: Silent sends nothing, Stalled sends the headers of a 200 and
// then holds the body. A test that leaves a call waiting on them waits for
// the call to reach the fake (the channel Serve returns) before it ends: the
// fake restores the default transport once the calls it received let go of it.
const (
	Silent  = "\x00silent"
	Stalled = "\x00stalled"
)

// Error is the body of Last.fm's error answer of code: 6 refuses the request
// itself, 9 refuses the session, 11 asks to try again later.
func Error(code int) string {
	return fmt.Sprintf(`<lfm status="failed"><error code="%d">fake: error %d</error></lfm>`, code, code)
}

// Serve serves the Last.fm API from an httptest server and returns the calls
// it receives. respond maps an API method to the XML body to answer; a method
// it returns "" for gets Error(6): Last.fm refuses the request.
//
// shkh/lastfm-go hardcodes the API URL and builds its own http.Client, so the
// only seam is the default transport: tests using this cannot run in parallel.
func Serve(t *testing.T, respond func(method string, params url.Values) string) <-chan Call {
	t.Helper()
	calls := make(chan Call, 64)
	held := make(chan struct{}) // closed when the test ends

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		method := r.Form.Get("method")
		calls <- Call{Method: method, Params: r.Form}
		body := respond(method, r.Form)
		switch body {
		case Unreachable:
			panic(http.ErrAbortHandler)
		case Silent:
			<-held
			return
		case Stalled:
			w.WriteHeader(http.StatusOK)
			_ = http.NewResponseController(w).Flush() //nolint:errcheck // with no headers, the call waits as on Silent: a test of Stalled fails
			<-held
			return
		}
		if body == "" {
			body = Error(6)
		}
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>` + body))
	}))
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	// roundTrips is read-locked by each round trip: a call that read the
	// default transport holds it until its round trip ends.
	var roundTrips sync.RWMutex
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		roundTrips.RLock()
		defer roundTrips.RUnlock()
		r = r.Clone(r.Context())
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		return orig.RoundTrip(r)
	})
	t.Cleanup(func() {
		close(held)       // or the round trips and srv.Close wait for the answers held
		roundTrips.Lock() // a call left behind by the test still reads the default transport
		http.DefaultTransport = orig
		roundTrips.Unlock()
		srv.Close()
	})
	return calls
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

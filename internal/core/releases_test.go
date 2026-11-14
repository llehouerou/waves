package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/listenbrainz"
)

// fakeListenBrainz serves the fresh releases it was given, and counts how
// often it was asked for them.
type fakeListenBrainz struct {
	mu       sync.Mutex
	releases []listenbrainz.Release
	err      error
	asked    int
	hold     chan struct{} // when set, an answer waits for it to be closed
}

func (f *fakeListenBrainz) FreshReleases(ctx context.Context) ([]listenbrainz.Release, error) {
	f.mu.Lock()
	f.asked++
	releases, err, hold := f.releases, f.err, f.hold
	f.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return releases, err
}

func (f *fakeListenBrainz) timesAsked() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
}

// fresh is an album ListenBrainz lists as released today.
func fresh(releaseGroup, artist, title string) listenbrainz.Release {
	return listenbrainz.Release{
		ReleaseGroupMBID: releaseGroup,
		ArtistCreditName: artist,
		ReleaseName:      title,
		ReleaseDate:      time.Now().Format(time.DateOnly),
		PrimaryType:      "Album",
	}
}

// releaseTitles are the titles of the new releases, in the list's order.
func releaseTitles(t *testing.T, c *Core) []string {
	t.Helper()
	list, err := c.Releases().List()
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	titles := make([]string, len(list))
	for i := range list {
		titles[i] = list[i].ReleaseName
	}
	return titles
}

func TestReleases_ARefreshRunsToItsEndWithNoClientDrivingIt(t *testing.T) {
	seeded := openTestState(t)
	addLibraryTracks(t, seeded.DB(), "Low", 2)
	lb := &fakeListenBrainz{releases: []listenbrainz.Release{
		fresh("rg-low", "Low", "HEY WHAT"),
		fresh("rg-other", "Somebody Else", "Not For Us"),
	}}
	c, _ := newTestCore(t, Options{Database: seeded.closed(t), listenbrainz: lb})
	if c.Releases().Status().Discoveries {
		t.Error("status says discoveries, with no Last.fm API key to find them")
	}

	c.Releases().Refresh(false)

	eventually(t, "the release of a library artist is listed", func() bool {
		titles := releaseTitles(t, c)
		return len(titles) == 1 && titles[0] == "HEY WHAT"
	})
	eventually(t, "the refresh is over", func() bool { return !c.Releases().Status().Refreshing })
}

func TestReleases_ARefreshSaysItRunsThenThatItEnded(t *testing.T) {
	seeded := openTestState(t)
	addLibraryTracks(t, seeded.DB(), "Low", 2)
	lb := &fakeListenBrainz{
		releases: []listenbrainz.Release{fresh("rg-low", "Low", "HEY WHAT")},
		hold:     make(chan struct{}),
	}
	c, _ := newTestCore(t, Options{Database: seeded.closed(t), listenbrainz: lb})

	c.Releases().Refresh(false)
	if !c.Releases().Status().Refreshing {
		t.Error("status does not say a refresh runs")
	}
	expectNoEvent[ReleasesRefreshed](t, c)

	close(lb.hold)
	if e := nextEvent[ReleasesRefreshed](t, c); e.Err != nil {
		t.Errorf("ReleasesRefreshed = %+v, want a refresh that worked", e)
	}
	// The list is there when the event says so: a client reads it at once.
	if titles := releaseTitles(t, c); len(titles) != 1 || titles[0] != "HEY WHAT" {
		t.Errorf("releases = %v, want HEY WHAT", titles)
	}
	if status := c.Releases().Status(); status.Refreshing || status.Err != nil {
		t.Errorf("status = %+v, want no refresh running, none failed", status)
	}
}

// ListenBrainz gives the whole list each time, some ten megabytes of it.
func TestReleases_AFreshCacheIsNotDownloadedAgainUnlessForced(t *testing.T) {
	lb := &fakeListenBrainz{releases: []listenbrainz.Release{fresh("rg-low", "Low", "HEY WHAT")}}
	c, _ := newTestCore(t, Options{listenbrainz: lb})
	c.Releases().Refresh(false) // nothing cached yet
	nextEvent[ReleasesRefreshed](t, c)

	c.Releases().Refresh(false)
	if c.Releases().Status().Refreshing {
		t.Error("status says a refresh runs: the list was downloaded a moment ago")
	}
	expectNoEvent[ReleasesRefreshed](t, c)
	if n := lb.timesAsked(); n != 1 {
		t.Errorf("ListenBrainz was asked %d times, want once", n)
	}

	c.Releases().Refresh(true)
	nextEvent[ReleasesRefreshed](t, c)
	if n := lb.timesAsked(); n != 2 {
		t.Errorf("ListenBrainz was asked %d times, want a second time when forced", n)
	}
}

func TestReleases_OneRefreshRunsAtATime(t *testing.T) {
	lb := &fakeListenBrainz{
		releases: []listenbrainz.Release{fresh("rg-low", "Low", "HEY WHAT")},
		hold:     make(chan struct{}),
	}
	c, _ := newTestCore(t, Options{listenbrainz: lb})
	c.Releases().Refresh(true)
	eventually(t, "ListenBrainz is asked", func() bool { return lb.timesAsked() == 1 })

	c.Releases().Refresh(true)
	c.Releases().Refresh(false)
	time.Sleep(quiet)
	close(lb.hold)

	nextEvent[ReleasesRefreshed](t, c)
	expectNoEvent[ReleasesRefreshed](t, c)
	if n := lb.timesAsked(); n != 1 {
		t.Errorf("ListenBrainz was asked %d times while a refresh ran, want once", n)
	}
}

// A refresh that fails must not interrupt the user: the list keeps what was
// cached, and says so the next time it is opened.
func TestReleases_AFailedRefreshIsToldWithTheListAndRemembered(t *testing.T) {
	unreachable := errors.New("listenbrainz status 503")
	lb := &fakeListenBrainz{err: unreachable}
	c, _ := newTestCore(t, Options{listenbrainz: lb})

	c.Releases().Refresh(false)

	// One stream: a Failed emitted before the end of the refresh comes first.
	for ended := false; !ended; {
		switch e := (<-c.Events()).(type) {
		case Failed:
			t.Errorf("Failed = %+v, want the failure left to ReleasesRefreshed", e)
		case ReleasesRefreshed:
			if !errors.Is(e.Err, unreachable) {
				t.Errorf("ReleasesRefreshed.Err = %v, want ListenBrainz's failure", e.Err)
			}
			ended = true
		}
	}
	if status := c.Releases().Status(); status.Refreshing || !errors.Is(status.Err, unreachable) {
		t.Errorf("status = %+v, want the failure remembered", status)
	}

	lb.mu.Lock()
	lb.err, lb.releases = nil, []listenbrainz.Release{fresh("rg-low", "Low", "HEY WHAT")}
	lb.mu.Unlock()
	c.Releases().Refresh(false) // still nothing cached: stale
	nextEvent[ReleasesRefreshed](t, c)
	if status := c.Releases().Status(); status.Err != nil {
		t.Errorf("status = %+v, want the failure forgotten once a refresh works", status)
	}
}

// bedheadIsSimilar answers that Bedhead is the artist similar to any other.
func bedheadIsSimilar(method string, _ url.Values) string {
	if method == similarMethod {
		return `<lfm status="ok"><similarartists artist="Low">` +
			`<artist><name>Bedhead</name><match>0.9</match></artist>` +
			`</similarartists></lfm>`
	}
	return ""
}

// newDiscoveriesCore builds a core with a Last.fm API key and a library by Low
// and Codeine, whose ListenBrainz lists a release by Bedhead.
func newDiscoveriesCore(t *testing.T) *Core {
	t.Helper()
	seeded := openTestState(t)
	addLibraryTracks(t, seeded.DB(), "Low", 2)
	addLibraryTracks(t, seeded.DB(), "Codeine", 3)
	c, _ := newTestCore(t, Options{
		Database:     seeded.closed(t),
		Lastfm:       config.LastfmConfig{APIKey: "key"},
		Radio:        (&config.Config{}).GetRadioConfig(),
		listenbrainz: &fakeListenBrainz{releases: []listenbrainz.Release{fresh("rg-bedhead", "Bedhead", "WhatFunLifeWas")}},
	})
	return c
}

func TestReleases_TheWarmUpFindsDiscoveriesWithNoClientDrivingIt(t *testing.T) {
	lastfmtest.Serve(t, bedheadIsSimilar)
	c := newDiscoveriesCore(t)
	if !c.Releases().Status().Discoveries {
		t.Error("status says no discoveries: an API key is all they need")
	}

	c.Releases().Refresh(false)

	// Two artists of the library recommend Bedhead, which it has nothing by.
	var list []Release
	eventually(t, "the release of a discovery is listed", func() bool {
		var err error
		if list, err = c.Releases().List(); err != nil {
			t.Fatalf("list releases: %v", err)
		}
		return len(list) == 1
	})
	if r := list[0]; r.ArtistCreditName != "Bedhead" || r.InLibrary || len(r.Seeds) != 2 {
		t.Errorf("release = %+v, want Bedhead's, a discovery recommended by Low and Codeine", r)
	}
}

func TestReleases_OneWarmUpRunsAtATimeAndSaysWhenItEnded(t *testing.T) {
	answered := make(chan struct{})
	answer := sync.OnceFunc(func() { close(answered) }) // Last.fm answers at last
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		<-answered
		return bedheadIsSimilar(method, params)
	})
	c := newDiscoveriesCore(t)
	t.Cleanup(answer) // before the core closes: a failed test must not leave it waiting for Last.fm

	c.Releases().Refresh(false)
	waitCall(t, calls, similarMethod) // the two artists of the library
	waitCall(t, calls, similarMethod) // are with Last.fm

	c.Releases().Refresh(false)
	c.Releases().Refresh(true)
	expectNoCall(t, calls, similarMethod)

	answer()
	nextEvent[WarmupFinished](t, c)

	// Every artist has fresh similar artists now: nothing to ask, nothing
	// that changed to tell.
	c.Releases().Refresh(false)
	expectNoEvent[WarmupFinished](t, c)
	expectNoCall(t, calls, similarMethod)
}

func TestReleases_TheListSaysWhatIsOwnedAndWhatIsDownloading(t *testing.T) {
	seeded := openTestState(t)
	addLibraryTracks(t, seeded.DB(), "Low", 2) // the library has Low's "Album"
	lb := &fakeListenBrainz{releases: []listenbrainz.Release{
		fresh("rg-owned", "Low", "Album"),
		fresh("rg", "Low", "Trust"),
		fresh("rg-new", "Low", "HEY WHAT"),
	}}
	c, _, _ := newDownloadsCore(t, Options{Database: seeded.closed(t), listenbrainz: lb})
	if _, err := c.Downloads().Queue(album("01.flac")); err != nil { // release group "rg"
		t.Fatalf("queue: %v", err)
	}
	c.Releases().Refresh(false)
	nextEvent[ReleasesRefreshed](t, c)

	list, err := c.Releases().List()
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}

	type marks struct{ owned, downloading bool }
	got := map[string]marks{}
	for i := range list {
		got[list[i].ReleaseName] = marks{list[i].Owned, list[i].Downloading}
	}
	want := map[string]marks{"Album": {owned: true}, "Trust": {downloading: true}, "HEY WHAT": {}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("marks = %+v, want %+v", got, want)
	}
}

// closeWithin closes the core and fails the test, after calling unblock, when
// Close has not returned in time.
func closeWithin(t *testing.T, c *Core, unblock func(), what string) {
	t.Helper()
	closed := make(chan struct{})
	go func() {
		c.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		unblock()
		t.Fatalf("Close waits for %s", what)
	}
}

// expectStreamEndsWithout reads the event stream of a closed core to its end,
// failing the test on an event of type E.
func expectStreamEndsWithout[E Event](t *testing.T, c *Core) {
	t.Helper()
	for e := range c.Events() {
		if unwanted, ok := e.(E); ok {
			t.Errorf("unexpected %T: %+v", unwanted, unwanted)
		}
	}
}

// Quitting must not wait for ten megabytes to come down.
func TestClose_CutsARefreshOfTheReleasesShort(t *testing.T) {
	lb := &fakeListenBrainz{hold: make(chan struct{})} // never answers
	c, _ := newTestCore(t, Options{listenbrainz: lb})
	c.Releases().Refresh(true)
	eventually(t, "ListenBrainz is asked", func() bool { return lb.timesAsked() == 1 })

	closeWithin(t, c, func() { close(lb.hold) }, "ListenBrainz to answer")

	// Cut short by Close is not a refresh that failed.
	expectStreamEndsWithout[ReleasesRefreshed](t, c)
}

// Quitting does not wait for Last.fm, which may take long to answer: the
// warm-up is dropped as it is, and what Last.fm answers afterwards is written
// nowhere. The next warm-up asks about those artists again.
func TestClose_LeavesTheWarmUpRequestsInFlightBehind(t *testing.T) {
	answered := make(chan struct{})
	answer := sync.OnceFunc(func() { close(answered) }) // Last.fm answers at last
	var asked, answers atomic.Int32
	lastfmtest.Serve(t, func(method string, params url.Values) string {
		asked.Add(1)
		<-answered
		defer answers.Add(1)
		return bedheadIsSimilar(method, params)
	})
	seeded := openTestState(t)
	const artists = 40
	for i := range artists {
		addLibraryTracks(t, seeded.DB(), fmt.Sprintf("Artist %02d", i), 1)
	}
	database := seeded.closed(t)
	c, _ := newTestCore(t, Options{
		Database: database,
		Lastfm:   config.LastfmConfig{APIKey: "key"},
		Radio:    (&config.Config{}).GetRadioConfig(),
	})
	t.Cleanup(answer) // before the core closes: a failed test must not leave it waiting for Last.fm

	c.Releases().Refresh(false)
	eventually(t, "Last.fm is asked", func() bool { return asked.Load() > 0 })

	closed := make(chan struct{})
	go func() {
		c.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close waits for the answers of Last.fm")
	}
	expectStreamEndsWithout[WarmupFinished](t, c)

	answer()
	eventually(t, "Last.fm has answered", func() bool { return answers.Load() == asked.Load() })
	time.Sleep(quiet) // for the answers to reach waves
	var rows int
	if err := reopen(t, database).DB().QueryRow(`SELECT COUNT(*) FROM lastfm_similar_artists`).Scan(&rows); err != nil {
		t.Fatalf("count similar artists: %v", err)
	}
	if rows != 0 {
		t.Errorf("%d similar artists written after Close, want none", rows)
	}
}

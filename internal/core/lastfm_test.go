package core

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags"
)

// addPending leaves a scrobble of a track by Low that Last.fm did not take,
// as an earlier run would.
func addPending(t *testing.T, mgr *state.Manager, track string) {
	t.Helper()
	err := mgr.AddPendingScrobble(state.PendingScrobble{
		Artist: "Low", Track: track, DurationSecs: 180, Timestamp: time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("add pending scrobble: %v", err)
	}
}

// pendingScrobbles reads the scrobbles still waiting.
func pendingScrobbles(t *testing.T, mgr *state.Manager) []state.PendingScrobble {
	t.Helper()
	pending, err := mgr.GetPendingScrobbles()
	if err != nil {
		t.Fatalf("pending scrobbles: %v", err)
	}
	return pending
}

// Issue #92: nothing but linking in the same session used to start the retry.
func TestPendingScrobbles_AreSentAgainAtStartupWithNoClientArmingAnything(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptScrobbles)
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")

	newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	call := waitCall(t, calls, scrobbleMethod)
	if artist, track := call.Params.Get("artist[0]"), call.Params.Get("track[0]"); artist != "Low" || track != "Lullaby" {
		t.Errorf("scrobbled %s - %s, want Low - Lullaby", artist, track)
	}
	eventually(t, "the scrobble Last.fm took is no longer pending", func() bool {
		return len(pendingScrobbles(t, mgr)) == 0
	})
}

func TestPendingScrobbles_ARefusedOneIsSentAgainOnTheCoresClock(t *testing.T) {
	var up atomic.Bool // Last.fm takes scrobbles again
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		if !up.Load() {
			return ""
		}
		return acceptScrobbles(method, params)
	})
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")
	newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	waitCall(t, calls, scrobbleMethod) // the pass at startup: refused
	up.Store(true)

	waitCall(t, calls, scrobbleMethod) // the next pass, with nothing asking for it
	eventually(t, "the scrobble Last.fm took is no longer pending", func() bool {
		return len(pendingScrobbles(t, mgr)) == 0
	})
}

// refuseEverything is a Last.fm that answers every call with an error of its
// own.
func refuseEverything(string, url.Values) string { return "" }

func TestPendingScrobbles_ARefusalOfLastfmIsCounted(t *testing.T) {
	calls := lastfmtest.Serve(t, refuseEverything)
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")
	newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	waitCall(t, calls, scrobbleMethod)

	eventually(t, "the refusal is counted", func() bool {
		pending := pendingScrobbles(t, mgr)
		return len(pending) == 1 && pending[0].Attempts == 1 && pending[0].LastError != ""
	})
}

// refusedNineTimes leaves every pending scrobble one refusal away from being
// dropped.
func refusedNineTimes(t *testing.T, mgr *state.Manager) {
	t.Helper()
	if _, err := mgr.DB().Exec(`UPDATE lastfm_pending_scrobbles SET attempts = 9`); err != nil {
		t.Fatalf("nine refusals: %v", err)
	}
}

// A scrobble Last.fm will never take must not be asked for until the end of
// time, nor hold back the ones behind it.
func TestPendingScrobbles_TheTenthRefusalDropsIt(t *testing.T) {
	calls := lastfmtest.Serve(t, refuseEverything)
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")
	refusedNineTimes(t, mgr)
	newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	waitCall(t, calls, scrobbleMethod)

	eventually(t, "the scrobble is dropped", func() bool {
		return len(pendingScrobbles(t, mgr)) == 0
	})
}

// Pending scrobbles are there for a session played offline: being offline
// must never be what loses them.
func TestPendingScrobbles_FailingToReachLastfmCostsThemNothing(t *testing.T) {
	calls := lastfmtest.Serve(t, func(string, url.Values) string { return lastfmtest.Unreachable })
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")
	refusedNineTimes(t, mgr)
	newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	waitCall(t, calls, scrobbleMethod)
	time.Sleep(quiet)

	if pending := pendingScrobbles(t, mgr); len(pending) != 1 || pending[0].Attempts != 9 {
		t.Errorf("pending = %+v, want the scrobble kept, still refused nine times", pending)
	}
}

// A Last.fm that is down is asked once per pass, not once per scrobble played
// while it was.
func TestPendingScrobbles_APassStopsAtTheFirstOneThatFails(t *testing.T) {
	calls := lastfmtest.Serve(t, refuseEverything)
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")
	addPending(t, mgr, "Words")
	newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	waitCall(t, calls, scrobbleMethod)
	expectNoCall(t, calls, scrobbleMethod)
}

// Last.fm takes no scrobble older than two weeks.
func TestPendingScrobbles_OlderThanTwoWeeksAreDropped(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptScrobbles)
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")
	fifteenDaysAgo := time.Now().Add(-15 * 24 * time.Hour).Unix()
	if _, err := mgr.DB().Exec(`UPDATE lastfm_pending_scrobbles SET created_at = ?`, fifteenDaysAgo); err != nil {
		t.Fatalf("age the scrobble: %v", err)
	}
	newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	eventually(t, "the scrobble is dropped", func() bool {
		return len(pendingScrobbles(t, mgr)) == 0
	})
	expectNoCall(t, calls, scrobbleMethod)
}

// lastfmLinks is a Last.fm where lowfan authorizes the token it hands out,
// and which takes scrobbles.
func lastfmLinks(method string, params url.Values) string {
	switch method {
	case "auth.gettoken":
		return `<lfm status="ok"><token>tok</token></lfm>`
	case "auth.getsession":
		if params.Get("token") != "tok" {
			return ""
		}
		return `<lfm status="ok"><session><name>lowfan</name><key>fresh</key><subscriber>0</subscriber></session></lfm>`
	case "user.getinfo":
		return `<lfm status="ok"><user><name>lowfan</name></user></lfm>`
	}
	return acceptScrobbles(method, params)
}

func TestLink_TheCoreScrobblesAsTheAccountFromThenOn(t *testing.T) {
	calls := lastfmtest.Serve(t, lastfmLinks)
	mgr := openTestState(t)
	c, mock := newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})
	lfm := c.Lastfm()

	if username, linked := lfm.Account(); linked {
		t.Fatalf("account = %q before linking, want none", username)
	}
	// A track plays while the account is linked: the core must take the
	// session without being restarted.
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)

	link, err := lfm.BeginLink()
	if err != nil {
		t.Fatalf("begin link: %v", err)
	}
	if want := "https://www.last.fm/api/auth/?api_key=key&token=tok"; link.Token != "tok" || link.URL != want {
		t.Errorf("link = %+v, want token tok to authorize at %s", link, want)
	}
	username, err := lfm.CompleteLink(link.Token)
	if err != nil || username != "lowfan" {
		t.Fatalf("complete link = %q, %v, want lowfan", username, err)
	}

	if username, linked := lfm.Account(); !linked || username != "lowfan" {
		t.Errorf("account = %q, linked %v, want lowfan", username, linked)
	}
	if session, err := mgr.GetLastfmSession(); err != nil || session == nil || session.Username != "lowfan" || session.SessionKey != "fresh" {
		t.Errorf("saved session = %+v, %v, want lowfan's", session, err)
	}

	mock.SetPosition(2 * time.Minute)
	if sk := waitCall(t, calls, scrobbleMethod).Params.Get("sk"); sk != "fresh" {
		t.Errorf("scrobbled with session %q, want the one just linked", sk)
	}
}

// link links lowfan's account the way a client does.
func link(t *testing.T, c *Core) {
	t.Helper()
	started, err := c.Lastfm().BeginLink()
	if err != nil {
		t.Fatalf("begin link: %v", err)
	}
	if _, err := c.Lastfm().CompleteLink(started.Token); err != nil {
		t.Fatalf("complete link: %v", err)
	}
}

// What was played before the account was linked again, or while its session
// was lost, goes out as soon as there is an account to send it as.
func TestLink_SendsThePendingScrobblesAtOnce(t *testing.T) {
	calls := lastfmtest.Serve(t, lastfmLinks)
	mgr := openTestState(t)
	addPending(t, mgr, "Lullaby")
	c, _ := newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})
	expectNoCall(t, calls, scrobbleMethod) // nobody to scrobble as

	link(t, c)
	linked := time.Now()

	waitCall(t, calls, scrobbleMethod)
	if waited, pass := time.Since(linked), scrobbleRetryTicks*testTick; waited > pass/2 {
		t.Errorf("the scrobble waited %v for the core's clock (a pass every %v): linking asked for none", waited, pass)
	}
}

// The user comes back from the browser without having authorized waves.
func TestLink_ATokenLastfmRefusesLinksNothing(t *testing.T) {
	lastfmtest.Serve(t, lastfmLinks)
	mgr := openTestState(t)
	c, _ := newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})

	if username, err := c.Lastfm().CompleteLink("never authorized"); err == nil {
		t.Fatalf("complete link = %q, want Last.fm's refusal", username)
	}

	if username, linked := c.Lastfm().Account(); linked {
		t.Errorf("account = %q, want none", username)
	}
	if session, err := mgr.GetLastfmSession(); err != nil || session != nil {
		t.Errorf("saved session = %+v, %v, want none", session, err)
	}
}

func TestLink_NeedsAnAPIKeyAndItsSecret(t *testing.T) {
	calls := lastfmtest.Serve(t, lastfmLinks)
	for _, cfg := range []config.LastfmConfig{{}, {APIKey: "key"}} {
		c, _ := newTestCore(t, Options{Lastfm: cfg})
		lfm := c.Lastfm()

		if lfm.Configured() {
			t.Errorf("%+v: configured, want not", cfg)
		}
		if _, err := lfm.BeginLink(); !errors.Is(err, ErrLastfmNotConfigured) {
			t.Errorf("%+v: begin link = %v, want ErrLastfmNotConfigured", cfg, err)
		}
		if _, err := lfm.CompleteLink("tok"); !errors.Is(err, ErrLastfmNotConfigured) {
			t.Errorf("%+v: complete link = %v, want ErrLastfmNotConfigured", cfg, err)
		}
	}
	expectNoCall(t, calls, "auth.gettoken")
}

func TestUnlink_TheCoreNoLongerScrobbles(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptScrobbles)
	mgr := linkedState(t)
	c, mock := newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})
	if username, linked := c.Lastfm().Account(); !linked || username != "lowfan" {
		t.Fatalf("account = %q, linked %v, want lowfan, linked by an earlier run", username, linked)
	}

	if err := c.Lastfm().Unlink(); err != nil {
		t.Fatalf("unlink: %v", err)
	}

	if username, linked := c.Lastfm().Account(); linked {
		t.Errorf("account = %q, want none", username)
	}
	if session, err := mgr.GetLastfmSession(); err != nil || session != nil {
		t.Errorf("saved session = %+v, %v, want none: the next run would be linked again", session, err)
	}
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	mock.SetPosition(2 * time.Minute)
	expectNoCall(t, calls, scrobbleMethod)
}

// Similar artists are public: no account, and no secret, is needed for them.
func TestGetSimilarArtists_NeedsOnlyAnAPIKey(t *testing.T) {
	lastfmtest.Serve(t, codeineIsSimilar)

	c, _ := newTestCore(t, Options{Lastfm: config.LastfmConfig{APIKey: "key"}})
	similar, err := c.Lastfm().GetSimilarArtists("Low", 30)
	if err != nil || len(similar) != 1 || similar[0].Name != "Codeine" || similar[0].MatchScore != 0.9 {
		t.Errorf("similar to Low = %+v, %v, want Codeine at 0.9", similar, err)
	}

	c, _ = newTestCore(t, Options{})
	if _, err := c.Lastfm().GetSimilarArtists("Low", 30); !errors.Is(err, ErrLastfmNotConfigured) {
		t.Errorf("with no API key: %v, want ErrLastfmNotConfigured", err)
	}
}

// A scrobble Last.fm took and the core did not write down would be sent
// again by the next run.
func TestClose_WaitsForTheScrobbleBeingSentThenStops(t *testing.T) {
	answered := make(chan struct{})
	answer := sync.OnceFunc(func() { close(answered) }) // Last.fm answers at last
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		<-answered
		return acceptScrobbles(method, params)
	})
	mgr := linkedState(t)
	addPending(t, mgr, "Lullaby")
	addPending(t, mgr, "Words")
	c, _ := newTestCore(t, Options{state: mgr, Lastfm: lastfmConfigured})
	t.Cleanup(answer)                  // before the core closes: a failed test must not leave it waiting for Last.fm
	waitCall(t, calls, scrobbleMethod) // the first one is with Last.fm

	closed := make(chan struct{})
	go func() {
		c.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned while a scrobble was being sent")
	case <-time.After(quiet):
	}

	answer()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close still waits after Last.fm answered")
	}
	if pending := pendingScrobbles(t, mgr); len(pending) != 1 {
		t.Errorf("pending = %+v, want the one Last.fm took gone, the other left for the next run", pending)
	}
}

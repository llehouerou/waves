package app

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// lowfanAuthorizes is a Last.fm where lowfan authorizes the token it hands
// out.
func lowfanAuthorizes(method string, params url.Values) string {
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
	return ""
}

// lastfmKeys are Last.fm credentials: an account can be linked, and radio
// mode is offered.
var lastfmKeys = config.LastfmConfig{APIKey: "key", APISecret: "secret"}

// newLastfmTestModel builds a model whose core can link a Last.fm account,
// and returns the URLs the model opens in the browser.
func newLastfmTestModel(t *testing.T) (m Model, opened *[]string) {
	t.Helper()
	m = newCoreTestModel(t, nil, core.Options{Lastfm: lastfmKeys})
	opened = new([]string)
	m.openBrowser = func(url string) error {
		*opened = append(*opened, url)
		return nil
	}
	return m, opened
}

// press sends keys to the model one after the other, each followed by
// everything the commands it sets off produce, as the running program does.
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, key := range keys {
		pending := []tea.Msg{keyMsg(key)}
		for len(pending) > 0 {
			var cmd tea.Cmd
			m, cmd = updateModel(t, m, pending[0])
			msgs, _ := drain(t, cmd)
			pending = append(pending[1:], msgs...)
		}
	}
	return m
}

// lastfmPopup returns what the Last.fm popup shows.
func lastfmPopup(t *testing.T, m Model) string {
	t.Helper()
	pop := m.Popups.Get(popupctl.LastfmAuth)
	if pop == nil {
		t.Fatal("the Last.fm popup is not open")
	}
	return testutil.StripANSI(pop.View())
}

func TestLastfm_TheTUILinksAndUnlinksAnAccountThroughTheCore(t *testing.T) {
	lastfmtest.Serve(t, lowfanAuthorizes)
	m, opened := newLastfmTestModel(t)

	m, _ = updateModel(t, m, keyMsg("f"))
	m, _ = updateModel(t, m, keyMsg("l"))
	if view := lastfmPopup(t, m); !strings.Contains(view, "Not linked") {
		t.Fatalf("the popup does not say the account is not linked:\n%s", view)
	}

	m = press(t, m, keyEnter) // link: the core asks for a token
	if want := "https://www.last.fm/api/auth/?api_key=key&token=tok"; len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("opened in the browser: %v, want %s", *opened, want)
	}
	if view := lastfmPopup(t, m); !strings.Contains(view, "Authorizing") {
		t.Fatalf("the popup does not wait for the authorization:\n%s", view)
	}

	m = press(t, m, keyEnter) // authorized: the core takes the session
	if view := lastfmPopup(t, m); !strings.Contains(view, "Linked") || !strings.Contains(view, "lowfan") {
		t.Fatalf("the popup does not show lowfan's account linked:\n%s", view)
	}
	if username, linked := m.Core.Lastfm().Account(); !linked || username != "lowfan" {
		t.Errorf("the core's account = %q, linked %v, want lowfan", username, linked)
	}

	m = press(t, m, "u")
	if view := lastfmPopup(t, m); !strings.Contains(view, "Not linked") {
		t.Errorf("the popup does not say the account is unlinked:\n%s", view)
	}
	if username, linked := m.Core.Lastfm().Account(); linked {
		t.Errorf("the core's account = %q, want none", username)
	}
}

// The user presses Enter without having authorized waves in the browser.
func TestLastfm_ARefusedLinkIsToldInThePopup(t *testing.T) {
	lastfmtest.Serve(t, refusesTheSession)
	m, _ := newLastfmTestModel(t)
	m, _ = updateModel(t, m, keyMsg("f"))
	m, _ = updateModel(t, m, keyMsg("l"))
	m = press(t, m, keyEnter)

	m = press(t, m, keyEnter)

	if view := lastfmPopup(t, m); !strings.Contains(view, "not been authorized") {
		t.Errorf("the popup does not tell why the account was not linked:\n%s", view)
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want the failure left to the Last.fm popup", msg)
	}
}

// startLink presses Enter in the Last.fm popup: it returns the link start the
// model asked of the core, its answer not delivered.
func startLink(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	m, cmd := updateModel(t, m, keyMsg(keyEnter))
	if cmd == nil {
		t.Fatal("Enter asked nothing")
	}
	m, start := updateModel(t, m, cmd()) // the popup's action
	if start == nil {
		t.Fatal("the model did not begin a link")
	}
	return m, start
}

// A popup closed before the link it started began, then opened again: the
// link start is dropped, its failure too. The popup still says "Not linked",
// no token is kept, the browser is not opened.
func TestLastfm_ALinkStartedByAClosedPopupIsDropped(t *testing.T) {
	cases := map[string]func(method string, params url.Values) string{
		"started": lowfanAuthorizes,
		"failed": func(string, url.Values) string {
			return `<lfm status="failed"><error code="11">Service Offline</error></lfm>`
		},
	}
	for name, lastfm := range cases {
		t.Run(name, func(t *testing.T) {
			lastfmtest.Serve(t, lastfm)
			m, opened := newLastfmTestModel(t)
			m = press(t, m, "f", "l")
			m, start := startLink(t, m)
			m = press(t, m, keyEscape, "f", "l")

			m = feed(t, m, start())

			if view := lastfmPopup(t, m); !strings.Contains(view, "Not linked") || strings.Contains(view, "Offline") {
				t.Errorf("the popup changed:\n%s", view)
			}
			if m.lastfmAuthToken != "" || len(*opened) != 0 {
				t.Errorf("token %q kept, opened %v; want neither", m.lastfmAuthToken, *opened)
			}
			if msg := m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error popup = %q, want none", msg)
			}
		})
	}
}

// Enter pressed twice before the first link start came: the later one is
// taken once, the earlier dropped. One token is kept, the browser opened
// once.
func TestLastfm_TheEarlierOfTwoLinkStartsIsDropped(t *testing.T) {
	lastfmtest.Serve(t, lowfanAuthorizes)
	m, opened := newLastfmTestModel(t)
	m = press(t, m, "f", "l")
	m, first := startLink(t, m)
	m, second := startLink(t, m)
	earlier, later := first(), second()

	m = feed(t, m, later)
	m = feed(t, m, earlier)
	m = feed(t, m, later) // delivered again

	if len(*opened) != 1 || m.lastfmAuthToken != "tok" {
		t.Errorf("opened %v, token %q; want the browser opened once, the token kept", *opened, m.lastfmAuthToken)
	}
	if view := lastfmPopup(t, m); !strings.Contains(view, "Authorizing") {
		t.Errorf("the popup does not wait for the authorization:\n%s", view)
	}
}

// feed hands msg to the model, and everything the commands it sets off
// produce.
func feed(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	pending := []tea.Msg{msg}
	for len(pending) > 0 {
		var cmd tea.Cmd
		m, cmd = updateModel(t, m, pending[0])
		msgs, _ := drain(t, cmd)
		pending = append(pending[1:], msgs...)
	}
	return m
}

// refusesTheSession is a Last.fm that hands out tokens and authorizes none.
func refusesTheSession(method string, params url.Values) string {
	if method == "auth.getsession" {
		return `<lfm status="failed"><error code="14">This token has not been authorized</error></lfm>`
	}
	return lowfanAuthorizes(method, params)
}

// askCompletion starts a link in a Last.fm popup just opened and presses
// Enter on "I've authorized": it returns the completion the model asked of
// the core, its answer not delivered.
func askCompletion(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	m = press(t, m, keyEnter)
	return startLink(t, m) // in "Authorizing...", Enter asks the completion
}

// A popup closed while the completion it asked runs: the failure is told by
// the error popup, and a Last.fm popup opened since is left as it is.
func TestLastfm_AFailedCompletionOfAClosedPopupIsToldByTheErrorPopup(t *testing.T) {
	cases := map[string]struct {
		reopen func(t *testing.T, m Model) Model
		shows  string
		token  string
	}{
		"closed": {
			reopen: func(_ *testing.T, m Model) Model { return m },
		},
		"opened again": {
			reopen: func(t *testing.T, m Model) Model { return press(t, m, "f", "l") },
			shows:  "Not linked",
		},
		"linking again": {
			reopen: func(t *testing.T, m Model) Model { return press(t, m, "f", "l", keyEnter) },
			shows:  "Authorizing",
			token:  "tok",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			lastfmtest.Serve(t, refusesTheSession)
			m, _ := newLastfmTestModel(t)
			m = press(t, m, "f", "l")
			m, complete := askCompletion(t, m)
			m = press(t, m, keyEscape)
			m = c.reopen(t, m)

			m = feed(t, m, complete())

			if msg := m.Popups.ErrorMsg(); !strings.Contains(msg, "authenticate with Last.fm") || !strings.Contains(msg, "not been authorized") {
				t.Errorf("error popup = %q, want the Last.fm authentication failure", msg)
			}
			if c.shows != "" {
				if view := lastfmPopup(t, m); !strings.Contains(view, c.shows) || strings.Contains(view, "not been authorized") {
					t.Errorf("the popup does not still show %q:\n%s", c.shows, view)
				}
			}
			if m.lastfmAuthToken != c.token {
				t.Errorf("token kept = %q, want %q", m.lastfmAuthToken, c.token)
			}
		})
	}
}

// A popup closed while the completion it asked runs, the account linked by
// it: the Last.fm popup opened since shows the account linked, and the link
// it had in progress ends there.
func TestLastfm_ACompletionOfAClosedPopupEndsTheLinkOfTheNextOne(t *testing.T) {
	t.Run("link started", func(t *testing.T) {
		lastfmtest.Serve(t, lowfanAuthorizes)
		m, _ := newLastfmTestModel(t)
		m = press(t, m, "f", "l")
		m, complete := askCompletion(t, m)
		m = press(t, m, keyEscape, "f", "l", keyEnter) // "Authorizing...", a token kept

		m = feed(t, m, complete())

		if view := lastfmPopup(t, m); !strings.Contains(view, "Linked") || !strings.Contains(view, "lowfan") {
			t.Errorf("the popup does not show lowfan's account linked:\n%s", view)
		}
		if m.lastfmAuthToken != "" {
			t.Errorf("token kept = %q, want none", m.lastfmAuthToken)
		}
	})

	t.Run("link start answered after", func(t *testing.T) {
		lastfmtest.Serve(t, lowfanAuthorizes)
		m, opened := newLastfmTestModel(t)
		m = press(t, m, "f", "l")
		m, complete := askCompletion(t, m)
		m = press(t, m, keyEscape, "f", "l")
		m, start := startLink(t, m)

		m = feed(t, m, complete())
		m = feed(t, m, start())

		if view := lastfmPopup(t, m); !strings.Contains(view, "Linked") || !strings.Contains(view, "lowfan") {
			t.Errorf("the popup does not show lowfan's account linked:\n%s", view)
		}
		if m.lastfmAuthToken != "" || len(*opened) != 1 {
			t.Errorf("token %q kept, opened %v; want no token, the browser opened by the first link only", m.lastfmAuthToken, *opened)
		}
		if username, linked := m.Core.Lastfm().Account(); !linked || username != "lowfan" {
			t.Errorf("the core's account = %q, linked %v, want lowfan", username, linked)
		}
	})
	// The popup opened since asked a completion of its own, which fails
	// after the account was linked: the error popup tells it, the popup
	// still shows the account linked.
	t.Run("own completion failing after", func(t *testing.T) {
		var tokens atomic.Int32
		lastfmtest.Serve(t, func(method string, params url.Values) string {
			switch method {
			case "auth.gettoken":
				return fmt.Sprintf(`<lfm status="ok"><token>tok%d</token></lfm>`, tokens.Add(1))
			case "auth.getsession":
				if params.Get("token") != "tok1" {
					return refusesTheSession(method, params)
				}
				return lowfanAuthorizes(method, url.Values{"token": {"tok"}})
			}
			return lowfanAuthorizes(method, params)
		})
		m, _ := newLastfmTestModel(t)
		m = press(t, m, "f", "l")
		m, first := askCompletion(t, m)
		m = press(t, m, keyEscape, "f", "l")
		m, own := askCompletion(t, m)

		m = feed(t, m, first())
		m = feed(t, m, own())

		if view := lastfmPopup(t, m); !strings.Contains(view, "Linked") || !strings.Contains(view, "lowfan") {
			t.Errorf("the popup does not show lowfan's account linked:\n%s", view)
		}
		if msg := m.Popups.ErrorMsg(); !strings.Contains(msg, "authenticate with Last.fm") || !strings.Contains(msg, "not been authorized") {
			t.Errorf("error popup = %q, want the failure of the popup's own completion", msg)
		}
		if username, linked := m.Core.Lastfm().Account(); !linked || username != "lowfan" {
			t.Errorf("the core's account = %q, linked %v, want lowfan", username, linked)
		}
	})
}

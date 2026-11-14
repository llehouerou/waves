package app

import (
	"net/url"
	"strings"
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

// lastfmKeys are Last.fm credentials: an account can be linked, radio mode
// offered once it is.
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

// press sends a key to the model, then everything the commands it sets off
// produce, as the running program does.
func press(t *testing.T, m Model, key string) Model {
	t.Helper()
	pending := []tea.Msg{keyMsg(key)}
	for len(pending) > 0 {
		var cmd tea.Cmd
		m, cmd = updateModel(t, m, pending[0])
		msgs, _ := drain(t, cmd)
		pending = append(pending[1:], msgs...)
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
	lastfmtest.Serve(t, func(method string, params url.Values) string {
		if method == "auth.getsession" {
			return `<lfm status="failed"><error code="14">This token has not been authorized</error></lfm>`
		}
		return lowfanAuthorizes(method, params)
	})
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

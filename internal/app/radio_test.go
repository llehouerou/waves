package app

import (
	"database/sql"
	"strings"
	"testing"
	"testing/synctest"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// wantRadioSkipLine is the line the brief of #148 asks for when R skips radio
// mode for want of a linked Last.fm account.
const wantRadioSkipLine = "Radio mode needs a linked Last.fm account — link it with f l"

// cycleRepeat presses R and returns the model, the repeat mode it left and
// whether the view tells radio mode needs a linked account.
func cycleRepeat(t *testing.T, m Model) (Model, tea.Cmd, core.RepeatMode, bool) {
	t.Helper()
	m, cmd := updateModel(t, m, keyMsg("R"))
	told := strings.Contains(testutil.StripANSI(m.View()), wantRadioSkipLine)
	return m, cmd, m.Core.Playback().RepeatMode(), told
}

// It runs in a synctest bubble: the message's clear timer fires in virtual
// time.
func TestRadio_RSkippingRadioForWantOfAnAccountIsToldOnce(t *testing.T) {
	synctest.Test(t, rSkippingRadioIsToldOnce)
}

func rSkippingRadioIsToldOnce(t *testing.T) {
	m := newCoreTestModel(t, nil, core.Options{Lastfm: lastfmKeys})

	want := []core.RepeatMode{core.RepeatAll, core.RepeatOne}
	for _, w := range want {
		var mode core.RepeatMode
		var told bool
		m, _, mode, told = cycleRepeat(t, m)
		if mode != w {
			t.Fatalf("repeat mode = %v, want %v", mode, w)
		}
		if told {
			t.Fatalf("radio mode told on the way to %v", w)
		}
	}

	m, cmd, mode, told := cycleRepeat(t, m)
	if mode != core.RepeatOff {
		t.Fatalf("repeat mode = %v, want off: radio is skipped", mode)
	}
	if !told {
		t.Fatalf("the view does not tell radio mode needs a linked account:\n%s", testutil.StripANSI(m.View()))
	}
	if len(m.Notifications) != 1 {
		t.Fatalf("notifications = %+v, want the message alone", m.Notifications)
	}
	if cmd == nil || cmd() != (NotificationClearMsg{ID: m.Notifications[0].ID}) {
		t.Error("the press does not clear its message after a while")
	}
	m, _ = updateModel(t, m, NotificationClearMsg{ID: m.Notifications[0].ID})

	for range 3 {
		m, _, _, told = cycleRepeat(t, m)
		if told || len(m.Notifications) != 0 {
			t.Fatalf("radio mode told again in the same run: %+v", m.Notifications)
		}
	}
	if mode := m.Core.Playback().RepeatMode(); mode != core.RepeatOff {
		t.Errorf("repeat mode = %v after a second cycle, want off", mode)
	}
}

func TestRadio_RWithoutLastfmTellsNothing(t *testing.T) {
	m := newCoreTestModel(t, nil, core.Options{})

	for _, want := range []core.RepeatMode{core.RepeatAll, core.RepeatOne, core.RepeatOff} {
		var mode core.RepeatMode
		m, _, mode, _ = cycleRepeat(t, m)
		if mode != want {
			t.Fatalf("repeat mode = %v, want %v", mode, want)
		}
		if len(m.Notifications) != 0 {
			t.Fatalf("notifications = %+v on the way to %v, want none", m.Notifications, want)
		}
	}
}

func TestRadio_RWithALinkedAccountReachesRadioAndTellsNothing(t *testing.T) {
	linked := func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO lastfm_session (id, username, session_key, linked_at)
			VALUES (1, 'lowfan', 'session', 0)`); err != nil {
			t.Fatalf("save session: %v", err)
		}
	}
	m := newCoreTestModel(t, linked, core.Options{Lastfm: lastfmKeys})

	for _, want := range []core.RepeatMode{core.RepeatAll, core.RepeatOne, core.RepeatRadio, core.RepeatOff} {
		var mode core.RepeatMode
		m, _, mode, _ = cycleRepeat(t, m)
		if mode != want {
			t.Fatalf("repeat mode = %v, want %v", mode, want)
		}
		if len(m.Notifications) != 0 {
			t.Fatalf("notifications = %+v on the way to %v, want none", m.Notifications, want)
		}
	}
}

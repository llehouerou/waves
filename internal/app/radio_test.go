package app

import (
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// Radio mode needs Last.fm configured, not a linked account (#197): R reaches
// it with or without an account, and says nothing on the way.
func TestRadio_RCyclesThroughRadioWhenLastfmIsConfigured(t *testing.T) {
	linked := func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO lastfm_session (id, username, session_key, linked_at)
			VALUES (1, 'lowfan', 'session', 0)`); err != nil {
			t.Fatalf("save session: %v", err)
		}
	}
	tests := []struct {
		name string
		seed func(*sql.DB)
		opts core.Options
		want []core.RepeatMode
	}{
		{"no account", nil, core.Options{Lastfm: lastfmKeys},
			[]core.RepeatMode{core.RepeatAll, core.RepeatOne, core.RepeatRadio, core.RepeatOff}},
		{"linked account", linked, core.Options{Lastfm: lastfmKeys},
			[]core.RepeatMode{core.RepeatAll, core.RepeatOne, core.RepeatRadio, core.RepeatOff}},
		{"no Last.fm", nil, core.Options{},
			[]core.RepeatMode{core.RepeatAll, core.RepeatOne, core.RepeatOff}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newCoreTestModel(t, tt.seed, tt.opts)
			for _, want := range tt.want {
				var cmd tea.Cmd
				m, cmd = updateModel(t, m, keyMsg("R"))
				if mode := m.Core.Playback().RepeatMode(); mode != want {
					t.Fatalf("repeat mode = %v, want %v", mode, want)
				}
				if len(m.Notifications) != 0 || cmd != nil {
					t.Fatalf("on the way to %v: notifications %+v, command %v, want none", want, m.Notifications, cmd)
				}
			}
		})
	}
}

// radioOffLine is the line the brief of #197 asks for when the queue saved in
// radio mode is restored on a core with no Last.fm config.
const radioOffLine = "Radio mode turned off — Last.fm is not configured"

// seedRadioQueue saves in database a queue in radio mode, as an earlier run
// with Last.fm configured would have left it.
func seedRadioQueue(t *testing.T, database string) {
	t.Helper()
	mgr, err := state.OpenAt(database)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	err = mgr.SaveQueue(state.QueueState{
		CurrentIndex: 0,
		RepeatMode:   int(core.RepeatRadio),
		Tracks:       []state.QueueTrack{{Path: "/music/low.flac", Artist: "Low", Title: "Lullaby"}},
	})
	if err != nil {
		t.Fatalf("save queue: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("close state: %v", err)
	}
}

// The core with no Last.fm config restores the queue with repeat off, and the
// message bar says why for NotificationDuration from when the normal UI
// shows, whether the loading screen showed or not.
func TestRadio_RestoredWithNoLastfmConfigIsToldOnceTheUIShows(t *testing.T) {
	tests := []struct {
		name        string
		firstLaunch bool
		run         func(s *startup)
	}{
		{
			name: "fast start: the loading screen never shows",
			run:  func(s *startup) { s.send(s.init) },
		},
		{
			name:        "first launch: the loading screen shows for 3 s",
			firstLaunch: true,
			run: func(s *startup) {
				s.send(s.init)
				s.deliver(loadingMinTimeFirstLaunch - time.Millisecond)
				if s.m.loadingState != loadingShowing {
					s.t.Fatalf("loading state = %v, want showing", s.m.loadingState)
				}
				s.deliver(time.Millisecond)
			},
		},
		{
			name: "slow start: the loading screen showed before the initialization",
			run: func(s *startup) {
				s.send(ShowLoadingMsg{})
				time.Sleep(200 * time.Millisecond)
				s.send(s.init)
				s.deliver(600 * time.Millisecond)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				database := filepath.Join(t.TempDir(), "waves.db")
				seedRadioQueue(t, database)
				s := newStartupOn(t, database, tt.firstLaunch)
				s.eventsWaiting = true // RadioTurnedOff, emitted by the restore
				tt.run(s)
				s.deliver(time.Millisecond) // the event the core had waiting
				if s.m.loadingState != loadingDone {
					t.Fatalf("loading state = %v, want done", s.m.loadingState)
				}

				view := testutil.StripANSI(s.m.View())
				if !testutil.ContainsLine(view, radioOffLine) {
					t.Fatalf("the view does not say radio mode was turned off:\n%s", view)
				}
				if strings.Contains(view, icons.Radio()) {
					t.Errorf("the view shows the radio icon %q:\n%s", icons.Radio(), view)
				}
				if mode := s.m.Core.Playback().RepeatMode(); mode != core.RepeatOff {
					t.Errorf("repeat mode = %v, want Off", mode)
				}

				s.deliver(NotificationDuration - 10*time.Millisecond)
				if !strings.Contains(testutil.StripANSI(s.m.View()), radioOffLine) {
					t.Fatal("the message is cleared before NotificationDuration")
				}
				s.deliver(20 * time.Millisecond)
				if view := testutil.StripANSI(s.m.View()); strings.Contains(view, radioOffLine) {
					t.Fatalf("the message is still there after NotificationDuration:\n%s", view)
				}

				// The events that follow still reach the model.
				s.m.Core.Playback().AddTracks(core.Track{Path: "/music/codeine.flac", Artist: "Codeine", Title: "Pea"})
				if !slices.Contains(s.deliver(100*time.Millisecond), tea.Msg(ServiceQueueChangedMsg{})) {
					t.Error("the queue edit that follows does not reach the model")
				}

				// Playing, the player bar shows: no radio icon in it either.
				if err := s.m.Core.Playback().Play(); err != nil {
					t.Fatalf("play: %v", err)
				}
				s.deliver(100 * time.Millisecond)
				view = testutil.StripANSI(s.m.View())
				if !strings.Contains(view, icons.Play()) {
					t.Fatalf("no player bar on screen:\n%s", view)
				}
				if strings.Contains(view, icons.Radio()) {
					t.Errorf("the player bar shows the radio icon %q:\n%s", icons.Radio(), view)
				}
			})
		})
	}
}

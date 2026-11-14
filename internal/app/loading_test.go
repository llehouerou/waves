package app

import (
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// startup is a model that waits for its initialization, as New builds it,
// driven through the loading screen. It runs the commands Update returns the
// way Bubble Tea does, each in its own goroutine, and keeps those still
// running. Use it inside a synctest bubble: the loading screen's timers fire
// in virtual time, and synctest.Wait tells a command that waits from one that
// returned.
type startup struct {
	t       *testing.T
	m       Model
	init    InitResult // the model's real initialization, run up front
	running []runningCmd
}

// runningCmd is a command started by the message named step.
type runningCmd struct {
	step string
	msg  chan tea.Msg
}

// newStartup builds a model on a core of its own whose initialization has
// not been applied. firstLaunch leaves its database without saved navigation.
func newStartup(t *testing.T, firstLaunch bool) *startup {
	t.Helper()
	database := filepath.Join(t.TempDir(), "waves.db")
	if !firstLaunch {
		// As an earlier run would have left it: the core opens it after.
		mgr, err := state.OpenAt(database)
		if err != nil {
			t.Fatalf("open state: %v", err)
		}
		mgr.SaveNavigation(state.NavigationState{ViewMode: string(navctl.ViewLibrary)})
		if err := mgr.Close(); err != nil {
			t.Fatalf("close state: %v", err)
		}
	}
	c, err := core.New(core.Options{Player: player.NewMock(), Database: database})
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	m := Model{
		Navigation:    navctl.New(),
		Core:          c,
		Layout:        NewLayoutManager(queuepanel.New(c.Playback())),
		Popups:        popupctl.New(),
		Keys:          keymap.NewResolver(keymap.Bindings),
		loadingState:  loadingWaiting,
		LoadingStatus: "Loading navigators...",
		initConfig:    &initConfig{cfg: &config.Config{DefaultFolder: t.TempDir()}},
	}
	init, ok := m.startInitialization()().(InitResult)
	if !ok || init.Err != nil {
		t.Fatalf("initialization = %+v", init)
	}
	if init.IsFirstLaunch != firstLaunch {
		t.Fatalf("IsFirstLaunch = %v, want %v", init.IsFirstLaunch, firstLaunch)
	}
	s := &startup{t: t, m: m, init: init}
	s.send(tea.WindowSizeMsg{Width: 120, Height: 40}) // the program's first message
	return s
}

// send hands msg to Update and starts the commands it returns.
func (s *startup) send(msg tea.Msg) {
	s.t.Helper()
	var cmd tea.Cmd
	s.m, cmd = updateModel(s.t, s.m, msg)
	s.start(stepName(msg), cmd)
}

func (s *startup) start(step string, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	r := runningCmd{step: step, msg: make(chan tea.Msg, 1)}
	go func() { r.msg <- cmd() }()
	synctest.Wait()
	select {
	case msg := <-r.msg:
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			s.t.Fatalf("%s: a command returned %T at once", step, msg)
		}
		for _, c := range batch {
			s.start(step, c)
		}
	default:
		s.running = append(s.running, r)
	}
}

// watchers closes the core and counts, by the message that started them, the
// commands that waited on its events: each returns once, the event it read
// or ServiceClosedMsg. The other commands are the loading screen's timers.
func (s *startup) watchers() map[string]int {
	s.t.Helper()
	if err := s.m.Core.Close(); err != nil {
		s.t.Fatalf("close core: %v", err)
	}
	synctest.Wait()
	got := map[string]int{}
	for _, r := range s.running {
		select {
		case msg := <-r.msg:
			if _, timer := msg.(LoadingMessage); !timer {
				got[r.step]++
			}
		default:
			// A timer that has not fired yet.
		}
	}
	return got
}

func stepName(msg tea.Msg) string {
	switch msg.(type) {
	case InitResult:
		return "InitResult"
	case ShowLoadingMsg:
		return "ShowLoadingMsg"
	case HideLoadingMsg:
		return "HideLoadingMsg"
	case tea.WindowSizeMsg:
		return "WindowSizeMsg"
	}
	return "other"
}

// From startup to quit, one command waits on the core's events: the one the
// initialization starts, whatever the order and timing of the loading
// screen's messages (issue #139).
func TestLoading_OneEventWatcher(t *testing.T) {
	tests := []struct {
		name        string
		firstLaunch bool
		run         func(s *startup)
	}{
		{
			name:        "first launch: the screen shows for 3 s",
			firstLaunch: true,
			run: func(s *startup) {
				s.send(s.init)
				if s.m.loadingState != loadingShowing {
					s.t.Fatalf("after InitResult: loading state = %v, want showing", s.m.loadingState)
				}
				time.Sleep(3 * time.Second)
				s.send(HideLoadingMsg{})
			},
		},
		{
			name: "slow start: InitResult before the minimum display time",
			run: func(s *startup) {
				s.send(ShowLoadingMsg{})
				s.send(s.init)
				if s.m.loadingState != loadingShowing {
					s.t.Fatalf("after InitResult: loading state = %v, want showing", s.m.loadingState)
				}
				time.Sleep(800 * time.Millisecond)
				s.send(HideLoadingMsg{})
			},
		},
		{
			name: "fast start: InitResult before the screen shows",
			run: func(s *startup) {
				s.send(s.init)
				s.send(ShowLoadingMsg{})
			},
		},
		{
			name: "slower start: InitResult after the minimum display time",
			run: func(s *startup) {
				s.send(ShowLoadingMsg{})
				time.Sleep(800 * time.Millisecond)
				s.send(HideLoadingMsg{})
				if s.m.loadingState != loadingShowing {
					s.t.Fatalf("hidden before InitResult: loading state = %v", s.m.loadingState)
				}
				s.send(s.init)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newStartup(t, tt.firstLaunch)
				tt.run(s)

				if s.m.loadingState != loadingDone {
					t.Errorf("loading state = %v, want done", s.m.loadingState)
				}
				got := s.watchers()
				if len(got) != 1 || got["InitResult"] != 1 {
					t.Errorf("event watchers started by = %v, want one, by InitResult", got)
				}
			})
		})
	}
}

package app

import (
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/player"
)

// Keys and mouse events while the normal UI is not on screen: the quit keys
// quit, the others are dropped (issue #276).

var (
	rightClick = tea.MouseMsg{X: 60, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	ctrlC      = tea.KeyMsg{Type: tea.KeyCtrlC}
	f2         = tea.KeyMsg{Type: tea.KeyF2}
	quitKeys   = []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("q")}, ctrlC}
)

// input hands msg, a key or a mouse event, to Update and returns the command
// it gives, without starting it. A panic fails the test.
func (s *startup) input(msg tea.Msg) tea.Cmd {
	s.t.Helper()
	defer func() {
		if r := recover(); r != nil {
			s.t.Fatalf("%v panics: %v", msg, r)
		}
	}()
	var cmd tea.Cmd
	s.m, cmd = updateModel(s.t, s.m, msg)
	return cmd
}

// runUntilDone lets the loading screen's timers run until the normal UI shows.
func (s *startup) runUntilDone() {
	s.t.Helper()
	s.runFor(4 * time.Second)
	if s.m.loadingState != loadingDone {
		s.t.Fatalf("loading state = %v, want done", s.m.loadingState)
	}
}

// waitingSituations are the screens of a model that waits for its
// initialization.
var waitingSituations = []struct {
	name        string
	firstLaunch bool
	showing     bool
}{
	{"first launch, blank screen", true, false},
	{"first launch, loading screen", true, true},
	{"later launch, blank screen", false, false},
	{"later launch, loading screen", false, true},
}

// newWaitingStartup is a startup in situation s, its initialization not applied.
func newWaitingStartup(t *testing.T, firstLaunch, showing bool) *startup {
	t.Helper()
	s := newStartup(t, firstLaunch)
	if showing {
		s.send(ShowLoadingMsg{})
	}
	return s
}

func TestLoading_InputBeforeInitializationIsDropped(t *testing.T) {
	inputs := []struct {
		name string
		msg  tea.Msg
	}{
		{"h", keyMsg("h")},
		{"left", tea.KeyMsg{Type: tea.KeyLeft}},
		{"V", keyMsg("V")},
		{"f2", f2},
		{"f3", tea.KeyMsg{Type: tea.KeyF3}},
		{"f4", tea.KeyMsg{Type: tea.KeyF4}},
		{"right click", rightClick},
	}
	for _, sit := range waitingSituations {
		for _, in := range inputs {
			t.Run(sit.name+"/"+in.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					s := newWaitingStartup(t, sit.firstLaunch, sit.showing)
					before := s.m
					view := before.View()

					if cmd := s.input(in.msg); cmd != nil {
						t.Error("returns a command")
					}
					if !reflect.DeepEqual(s.m, before) {
						t.Error("changes the model")
					}
					if got := s.m.View(); got != view {
						t.Errorf("changes the screen:\n%s\nwant:\n%s", got, view)
					}
				})
			})
		}
	}
}

// What a key opens before the initialization is applied is not open when
// the normal UI shows.
func TestLoading_KeysBeforeInitializationOpenNothing(t *testing.T) {
	for _, sit := range waitingSituations {
		for _, key := range []string{"?", "/"} {
			t.Run(sit.name+"/"+key, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					s := newWaitingStartup(t, sit.firstLaunch, sit.showing)
					s.input(keyMsg(key))
					s.send(s.init)
					s.runUntilDone()

					if p := s.m.Popups.ActivePopup(); p != popupctl.None {
						t.Errorf("popup %v open, want none", p)
					}
					if s.m.Input.IsSearchActive() {
						t.Error("the search is active")
					}
					s.checkDone()
				})
			})
		}
	}
}

// On a first launch the loading screen shows 3 s after the initialization
// is applied: the keys and the mouse do nothing under it.
func TestLoading_InputUnderTheLoadingScreenIsDropped(t *testing.T) {
	inputs := []struct {
		name  string
		input func(s *startup) tea.Msg
	}{
		{"f2", func(*startup) tea.Msg { return f2 }},
		{"?", func(*startup) tea.Msg { return keyMsg("?") }},
		{"-", func(*startup) tea.Msg { return keyMsg("-") }},
		{"wheel over the player bar", func(s *startup) tea.Msg {
			return tea.MouseMsg{Y: s.m.PlayerBarRow() - 1, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
		}},
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newStartup(t, true)
				s.send(s.init)
				if s.m.loadingState != loadingShowing {
					t.Fatalf("loading state = %v, want showing", s.m.loadingState)
				}
				testPlayer(s.m.Core).SetState(player.Playing) // the player bar shows
				mode := s.m.Navigation.ViewMode()
				volume := s.m.Core.Playback().Volume()

				if cmd := s.input(in.input(s)); cmd != nil {
					t.Error("returns a command")
				}

				if got := s.m.Navigation.ViewMode(); got != mode {
					t.Errorf("view mode = %v, want %v", got, mode)
				}
				if p := s.m.Popups.ActivePopup(); p != popupctl.None {
					t.Errorf("popup %v open, want none", p)
				}
				if got := s.m.Core.Playback().Volume(); got != volume {
					t.Errorf("volume = %v, want %v", got, volume)
				}
				s.runUntilDone()
				s.checkDone()
			})
		})
	}
}

// quitsAtOnce checks that key, the first time it is pressed, quits.
func (s *startup) quitsAtOnce(key tea.Msg) {
	s.t.Helper()
	cmd := s.input(key)
	if !s.m.closing {
		s.t.Fatalf("%v: the model is not closing", key)
	}
	if cmd == nil {
		s.t.Fatalf("%v returns no command", key)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		s.t.Errorf("%v: the command does not end the program", key)
	}
}

func TestLoading_QuitKeysQuit(t *testing.T) {
	situations := []struct {
		name  string
		start func(t *testing.T) *startup
	}{
		{"blank screen", func(t *testing.T) *startup {
			return newWaitingStartup(t, false, false)
		}},
		{"loading screen before the initialization", func(t *testing.T) *startup {
			return newWaitingStartup(t, false, true)
		}},
		{"loading screen after the initialization", func(t *testing.T) *startup {
			s := newStartup(t, true)
			s.send(s.init)
			return s
		}},
	}
	for _, sit := range situations {
		for _, key := range quitKeys {
			t.Run(sit.name+"/"+key.String(), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					sit.start(t).quitsAtOnce(key)
				})
			})
		}
	}
}

// An error popup raised under the loading screen does not take the quit key.
func TestLoading_QuitKeysQuitOverAnErrorPopup(t *testing.T) {
	for _, key := range quitKeys {
		t.Run(key.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newWaitingStartup(t, false, true)
				s.input(StderrMsg{Line: "minimp3: decoding failed"})
				if s.m.Popups.ErrorMsg() == "" {
					t.Fatal("no error popup")
				}
				s.quitsAtOnce(key)
			})
		})
	}
}

// The keys dropped during the startup dismiss no popup: an error raised then
// is still open when the normal UI shows.
func TestLoading_ErrorPopupStaysOpenUntilTheUIShows(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newWaitingStartup(t, false, true)
		s.input(StderrMsg{Line: "minimp3: decoding failed"})
		s.input(keyMsg("x"))
		s.send(s.init)
		s.runUntilDone()

		if s.m.Popups.ErrorMsg() == "" {
			t.Error("the error popup is closed")
		}
		s.checkDone()
	})
}

// Once the normal UI shows, the keys act.
func TestLoading_KeysActOnceTheUIShows(t *testing.T) {
	for _, firstLaunch := range []bool{true, false} {
		t.Run(launchName(firstLaunch), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := newStartup(t, firstLaunch)
				s.send(s.init)
				s.runUntilDone()
				if s.m.Navigation.ViewMode() == navctl.ViewFileBrowser {
					t.Fatal("the startup view is already the file browser")
				}

				s.input(f2)
				if got := s.m.Navigation.ViewMode(); got != navctl.ViewFileBrowser {
					t.Errorf("after f2: view mode = %v, want the file browser", got)
				}
				s.input(keyMsg("?"))
				if !s.m.Popups.IsVisible(popupctl.Help) {
					t.Error("? opens no help popup")
				}
				s.checkDone()
			})
		})
	}
}

func launchName(firstLaunch bool) string {
	if firstLaunch {
		return "first launch"
	}
	return "later launch"
}

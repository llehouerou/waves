// internal/app/integration_test.go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// Key constants for tests.
const (
	keyEnter  = "enter"
	keyEscape = "escape"
)

// These integration tests verify cross-component interactions and user workflows.

// updateModel is a helper that calls Update and returns the Model.
func updateModel(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	newModel, cmd := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatalf("Update should return Model, got %T", newModel)
	}
	return result, cmd
}

// --- Queue + Playback Flow Tests ---

func TestIntegration_QueuePlaybackFlow(t *testing.T) {
	t.Run("add tracks then play from beginning", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Playback.Queue().Add(
			playlist.Track{Path: "/a.mp3", Artist: "A", Title: "Track A"},
			playlist.Track{Path: "/b.mp3", Artist: "B", Title: "Track B"},
			playlist.Track{Path: "/c.mp3", Artist: "C", Title: "Track C"},
		)

		// Press Home to go to first track
		m, _ = updateModel(t, m, keyMsg("home"))
		if m.Playback.Queue().CurrentIndex() != 0 {
			t.Errorf("after home: index = %d, want 0", m.Playback.Queue().CurrentIndex())
		}

		// Press End to go to last track
		m, _ = updateModel(t, m, keyMsg("end"))
		if m.Playback.Queue().CurrentIndex() != 2 {
			t.Errorf("after end: index = %d, want 2", m.Playback.Queue().CurrentIndex())
		}
	})

	t.Run("skip through tracks with pgdown", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Playback.Queue().Add(
			playlist.Track{Path: "/1.mp3"},
			playlist.Track{Path: "/2.mp3"},
			playlist.Track{Path: "/3.mp3"},
		)
		m.Playback.Queue().JumpTo(0)

		mock, _ := m.Playback.Player().(*player.Mock)
		mock.SetState(player.Playing)

		// Skip to next track
		m, _ = updateModel(t, m, keyMsg("pgdown"))
		if m.Playback.Queue().CurrentIndex() != 1 {
			t.Errorf("after pgdown: index = %d, want 1", m.Playback.Queue().CurrentIndex())
		}
	})

	t.Run("cannot skip past beginning with pgup", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Playback.Queue().Add(playlist.Track{Path: "/1.mp3"})
		m.Playback.Queue().JumpTo(0)

		mock, _ := m.Playback.Player().(*player.Mock)
		mock.SetState(player.Playing)

		// Try to skip before first track
		m, _ = updateModel(t, m, keyMsg("pgup"))
		if m.Playback.Queue().CurrentIndex() != 0 {
			t.Errorf("after pgup at start: index = %d, want 0", m.Playback.Queue().CurrentIndex())
		}
	})
}

// --- Focus Cycling Tests ---

func TestIntegration_FocusCycling(t *testing.T) {
	t.Run("tab cycles navigator to queue and back", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Layout.ShowQueue()
		m.Navigation.SetFocus(FocusNavigator)

		// Tab to queue
		m, _ = updateModel(t, m, keyMsg("tab"))
		if m.Navigation.Focus() != FocusQueue {
			t.Errorf("after first tab: focus = %v, want FocusQueue", m.Navigation.Focus())
		}

		// Tab back to navigator
		m, _ = updateModel(t, m, keyMsg("tab"))
		if m.Navigation.Focus() != FocusNavigator {
			t.Errorf("after second tab: focus = %v, want FocusNavigator", m.Navigation.Focus())
		}
	})

	t.Run("hiding queue resets focus to navigator", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Layout.ShowQueue()
		m.Navigation.SetFocus(FocusQueue)

		// Hide queue with 'p'
		m, _ = updateModel(t, m, keyMsg("p"))

		if m.Layout.IsQueueVisible() {
			t.Error("queue should be hidden")
		}
		if m.Navigation.Focus() != FocusNavigator {
			t.Errorf("focus = %v, want FocusNavigator", m.Navigation.Focus())
		}
	})

	t.Run("tab is noop when queue hidden", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Layout.HideQueue()
		m.Navigation.SetFocus(FocusNavigator)

		m, _ = updateModel(t, m, keyMsg("tab"))

		if m.Navigation.Focus() != FocusNavigator {
			t.Errorf("focus = %v, want FocusNavigator", m.Navigation.Focus())
		}
	})
}

// --- Repeat Mode Tests ---

func TestIntegration_RepeatModes(t *testing.T) {
	t.Run("cycle through repeat modes with R", func(t *testing.T) {
		m := newIntegrationTestModel()
		initial := m.Playback.Queue().RepeatMode()

		// First R cycles to next mode
		m, _ = updateModel(t, m, keyMsg("R"))
		mode1 := m.Playback.Queue().RepeatMode()
		if mode1 == initial {
			t.Error("repeat mode should change after first R")
		}

		// Second R cycles again
		m, _ = updateModel(t, m, keyMsg("R"))
		mode2 := m.Playback.Queue().RepeatMode()
		if mode2 == mode1 {
			t.Error("repeat mode should change after second R")
		}

		// Third R should cycle back to initial
		m, _ = updateModel(t, m, keyMsg("R"))
		if m.Playback.Queue().RepeatMode() != initial {
			t.Errorf("repeat mode = %v, want %v (back to initial)", m.Playback.Queue().RepeatMode(), initial)
		}
	})

	t.Run("toggle shuffle with S", func(t *testing.T) {
		m := newIntegrationTestModel()
		initial := m.Playback.Queue().Shuffle()

		m, _ = updateModel(t, m, keyMsg("S"))
		if m.Playback.Queue().Shuffle() == initial {
			t.Error("shuffle should toggle")
		}

		m, _ = updateModel(t, m, keyMsg("S"))
		if m.Playback.Queue().Shuffle() != initial {
			t.Error("shuffle should toggle back")
		}
	})
}

// --- Space Key Tests ---

func TestIntegration_SpaceKey(t *testing.T) {
	t.Run("space toggles play/pause immediately", func(t *testing.T) {
		m := newIntegrationTestModel()
		mock, _ := m.Playback.Player().(*player.Mock)
		mock.SetState(player.Playing)

		// Press space - should immediately toggle play/pause
		m, _ = updateModel(t, m, keyMsg(" "))

		// Should trigger pause immediately
		if mock.State() != player.Paused {
			t.Errorf("player state = %v, want Paused", mock.State())
		}
	})

	t.Run("space resumes paused playback", func(t *testing.T) {
		m := newIntegrationTestModel()
		mock, _ := m.Playback.Player().(*player.Mock)
		mock.SetState(player.Paused)

		// Press space
		m, _ = updateModel(t, m, keyMsg(" "))

		// Should resume
		if mock.State() != player.Playing {
			t.Errorf("player state = %v, want Playing", mock.State())
		}
	})
}

// --- Stop Behavior Tests ---

func TestIntegration_StopBehavior(t *testing.T) {
	t.Run("s stops playback from any state", func(t *testing.T) {
		states := []player.State{player.Playing, player.Paused}

		for _, initialState := range states {
			t.Run(initialState.String(), func(t *testing.T) {
				m := newIntegrationTestModel()
				mock, _ := m.Playback.Player().(*player.Mock)
				mock.SetState(initialState)

				m, _ = updateModel(t, m, keyMsg("s"))

				if mock.State() != player.Stopped {
					t.Errorf("player state = %v, want Stopped", mock.State())
				}
			})
		}
	})
}

// --- Queue Panel Interaction Tests ---

func TestIntegration_QueuePanelInteraction(t *testing.T) {
	t.Run("jump to track from queue panel", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Playback.Queue().Add(
			playlist.Track{Path: "/a.mp3"},
			playlist.Track{Path: "/b.mp3"},
			playlist.Track{Path: "/c.mp3"},
		)
		m.Layout.ShowQueue()
		m.Navigation.SetFocus(FocusQueue)

		// Simulate JumpToTrack action (normally sent by queue panel)
		m, _ = updateModel(t, m, queuepanel.ActionMsg(queuepanel.JumpToTrack{Index: 2}))

		if m.Playback.Queue().CurrentIndex() != 2 {
			t.Errorf("queue index = %d, want 2", m.Playback.Queue().CurrentIndex())
		}
		// Note: PlayTrackAtIndex now uses PlaybackService.Play() which triggers
		// async events instead of returning commands directly
		mock, _ := m.Playback.Player().(*player.Mock)
		if mock.State() != player.Playing {
			t.Errorf("player state = %v, want Playing", mock.State())
		}
	})
}

// --- Error Handling Tests ---

func TestIntegration_ErrorHandling(t *testing.T) {
	t.Run("error overlay blocks all keys until dismissed", func(t *testing.T) {
		m := newIntegrationTestModel()
		m.Popups.ShowError("Test error")
		initialFocus := m.Navigation.Focus()

		// Try to toggle queue - should be blocked by error overlay
		m, _ = updateModel(t, m, keyMsg("p"))

		// Error should be dismissed (any key dismisses)
		if m.Popups.ErrorMsg() != "" {
			t.Error("error should be dismissed after key press")
		}
		// Focus should be unchanged (key was consumed by error dismissal)
		if m.Navigation.Focus() != initialFocus {
			t.Errorf("focus = %v, want %v", m.Navigation.Focus(), initialFocus)
		}
	})

	t.Run("any key dismisses error overlay", func(t *testing.T) {
		keys := []string{"x", "enter", "escape", " ", "tab"}

		for _, key := range keys {
			t.Run(key, func(t *testing.T) {
				m := newIntegrationTestModel()
				m.Popups.ShowError("Test error")

				m, _ = updateModel(t, m, keyMsg(key))

				if m.Popups.ErrorMsg() != "" {
					t.Errorf("ErrorMsg = %q, want empty after %q", m.Popups.ErrorMsg(), key)
				}
			})
		}
	})
}

// --- Quit Tests ---

func TestIntegration_QuitBehavior(t *testing.T) {
	t.Run("q stops player and closes state", func(t *testing.T) {
		m := newIntegrationTestModel()
		mock, _ := m.Playback.Player().(*player.Mock)
		mock.SetState(player.Playing)

		_, cmd := m.Update(keyMsg("q"))

		if mock.State() != player.Stopped {
			t.Error("player should be stopped")
		}
		if cmd == nil {
			t.Error("expected quit command")
		}
	})

	t.Run("ctrl+c stops player and closes state", func(t *testing.T) {
		m := newIntegrationTestModel()
		mock, _ := m.Playback.Player().(*player.Mock)
		mock.SetState(player.Playing)

		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

		if mock.State() != player.Stopped {
			t.Error("player should be stopped")
		}
		if cmd == nil {
			t.Error("expected quit command")
		}
	})
}

// keyMsg creates a tea.KeyMsg for testing.
func keyMsg(key string) tea.Msg {
	if len(key) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	// Handle special keys
	switch key {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case keyEnter:
		return tea.KeyMsg{Type: tea.KeyEnter}
	case keyEscape:
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

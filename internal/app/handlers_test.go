// internal/app/handlers_test.go
package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
)

func TestHandleQuitKeys(t *testing.T) {
	tests := []struct {
		key      string
		wantQuit bool
	}{
		{"q", true},
		{"ctrl+c", true},
		{"x", false},
		{"Q", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := newTestModel()
			handled, cmd := m.handleQuitKeys(tt.key)

			if tt.wantQuit {
				if !handled {
					t.Errorf("expected %q to be handled", tt.key)
				}
				if cmd == nil {
					t.Error("expected quit command")
				}
			} else if handled {
				t.Errorf("expected %q to not be handled", tt.key)
			}
		})
	}
}

func TestHandleViewKeys(t *testing.T) {
	t.Run("unhandled key returns false", func(t *testing.T) {
		m := newTestModel()

		handled, _ := m.handleViewKeys("f3")

		if handled {
			t.Error("expected 'f3' to not be handled")
		}
	})

	// Note: f1/f2 tests require initialized navigators for SaveNavigationState
	// These are covered by integration tests
}

func TestHandleFocusKeys(t *testing.T) {
	t.Run("toggle queue visibility", func(t *testing.T) {
		m := newTestModel()
		m.QueueVisible = true

		handled, _ := m.handleFocusKeys("p")

		if !handled {
			t.Error("expected 'p' to be handled")
		}
		if m.QueueVisible {
			t.Error("expected queue to be hidden")
		}
	})

	t.Run("hide queue resets focus", func(t *testing.T) {
		m := newTestModel()
		m.QueueVisible = true
		m.Focus = FocusQueue

		m.handleFocusKeys("p")

		if m.Focus != FocusNavigator {
			t.Errorf("Focus = %v, want FocusNavigator", m.Focus)
		}
	})

	t.Run("tab switches focus when queue visible", func(t *testing.T) {
		m := newTestModel()
		m.QueueVisible = true
		m.Focus = FocusNavigator

		handled, _ := m.handleFocusKeys("tab")

		if !handled {
			t.Error("expected 'tab' to be handled")
		}
		if m.Focus != FocusQueue {
			t.Errorf("Focus = %v, want FocusQueue", m.Focus)
		}
	})

	t.Run("tab switches back from queue", func(t *testing.T) {
		m := newTestModel()
		m.QueueVisible = true
		m.Focus = FocusQueue

		m.handleFocusKeys("tab")

		if m.Focus != FocusNavigator {
			t.Errorf("Focus = %v, want FocusNavigator", m.Focus)
		}
	})

	t.Run("tab does nothing when queue hidden", func(t *testing.T) {
		m := newTestModel()
		m.QueueVisible = false
		m.Focus = FocusNavigator

		m.handleFocusKeys("tab")

		if m.Focus != FocusNavigator {
			t.Errorf("Focus = %v, want FocusNavigator", m.Focus)
		}
	})
}

func TestHandlePlaybackKeys(t *testing.T) {
	t.Run("space sets pending keys", func(t *testing.T) {
		m := newTestModel()

		handled, cmd := m.handlePlaybackKeys(" ")

		if !handled {
			t.Error("expected space to be handled")
		}
		if m.PendingKeys != " " {
			t.Errorf("PendingKeys = %q, want %q", m.PendingKeys, " ")
		}
		if cmd == nil {
			t.Error("expected timeout command")
		}
	})

	t.Run("s stops player", func(t *testing.T) {
		m := newTestModel()
		mock, ok := m.Player.(*player.Mock)
		if !ok {
			t.Fatal("expected mock player")
		}
		mock.SetState(player.Playing)

		handled, _ := m.handlePlaybackKeys("s")

		if !handled {
			t.Error("expected 's' to be handled")
		}
		if mock.State() != player.Stopped {
			t.Errorf("player state = %v, want Stopped", mock.State())
		}
	})

	t.Run("R cycles repeat mode", func(t *testing.T) {
		m := newTestModel()
		initialMode := m.Queue.RepeatMode()

		handled, _ := m.handlePlaybackKeys("R")

		if !handled {
			t.Error("expected 'R' to be handled")
		}
		if m.Queue.RepeatMode() == initialMode {
			t.Error("expected repeat mode to change")
		}
	})

	t.Run("S toggles shuffle", func(t *testing.T) {
		m := newTestModel()
		initialShuffle := m.Queue.Shuffle()

		handled, _ := m.handlePlaybackKeys("S")

		if !handled {
			t.Error("expected 'S' to be handled")
		}
		if m.Queue.Shuffle() == initialShuffle {
			t.Error("expected shuffle to toggle")
		}
	})

	t.Run("v toggles player display", func(t *testing.T) {
		m := newTestModel()

		handled, _ := m.handlePlaybackKeys("v")

		if !handled {
			t.Error("expected 'v' to be handled")
		}
	})
}

func TestHandleNavigatorActionKeys(t *testing.T) {
	t.Run("unhandled key returns false", func(t *testing.T) {
		m := newTestModel()

		handled, _ := m.handleNavigatorActionKeys("x")

		if handled {
			t.Error("expected 'x' to not be handled")
		}
	})

	// Note: Tests for /, enter, a, r require initialized navigators/library
	// These are covered by integration tests
}

// newTestModel creates a minimal model for testing handlers.
func newTestModel() *Model {
	return &Model{
		ViewMode:     ViewLibrary,
		QueueVisible: true,
		Focus:        FocusNavigator,
		Player:       player.NewMock(),
		Queue:        playlist.NewQueue(),
		StateMgr:     state.NewMock(),
	}
}

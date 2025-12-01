// internal/app/handlers_test.go
package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
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
	queue := playlist.NewQueue()
	return &Model{
		ViewMode:     ViewLibrary,
		QueueVisible: true,
		Focus:        FocusNavigator,
		Player:       player.NewMock(),
		Queue:        queue,
		QueuePanel:   queuepanel.New(queue),
		StateMgr:     state.NewMock(),
	}
}

// --- Key Sequence Tests ---

func TestIsValidSequencePrefix(t *testing.T) {
	tests := []struct {
		pending string
		want    bool
	}{
		{" ", true},     // Start of " ff" or " lr"
		{" f", true},    // Prefix of " ff"
		{" ff", true},   // Complete sequence
		{" l", true},    // Prefix of " lr"
		{" lr", true},   // Complete sequence
		{" x", false},   // Invalid second key
		{" fx", false},  // Invalid third key
		{" lx", false},  // Invalid third key
		{"x", false},    // Not starting with space
		{"", true},      // Empty is valid (nothing pending)
		{" fff", false}, // Too long
	}

	for _, tt := range tests {
		t.Run(tt.pending, func(t *testing.T) {
			got := IsValidSequencePrefix(tt.pending)
			if got != tt.want {
				t.Errorf("IsValidSequencePrefix(%q) = %v, want %v", tt.pending, got, tt.want)
			}
		})
	}
}

func TestHandlePendingKeys_InvalidSecondKey(t *testing.T) {
	m := newTestModel()
	m.PendingKeys = " "

	// Press invalid key 'x' - should clear pending and trigger space action
	result, _ := m.handlePendingKeys("x")

	resultModel, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}
	// Should clear pending keys
	if resultModel.PendingKeys != "" {
		t.Errorf("PendingKeys should be cleared on invalid sequence, got %q", resultModel.PendingKeys)
	}
}

func TestHandlePendingKeys_ValidPrefix(t *testing.T) {
	m := newTestModel()
	m.PendingKeys = " "

	// Press 'f' - valid prefix for " ff"
	result, _ := m.handlePendingKeys("f")

	resultModel, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}
	// Should keep building the sequence
	if resultModel.PendingKeys != " f" {
		t.Errorf("PendingKeys = %q, want ' f'", resultModel.PendingKeys)
	}
}

func TestHandlePendingKeys_TooLong(t *testing.T) {
	m := newTestModel()
	m.PendingKeys = " fx" // Already invalid

	// Press another key - sequence too long
	result, _ := m.handlePendingKeys("x")

	resultModel, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}
	if resultModel.PendingKeys != "" {
		t.Errorf("PendingKeys should be cleared when too long, got %q", resultModel.PendingKeys)
	}
}

// Note: View mode transition tests (F1/F2) require initialized navigators
// because handleViewKeys calls SaveNavigationState. These are covered by
// integration tests in integration_test.go

// --- Focus Transition Tests ---

func TestFocusCycling(t *testing.T) {
	m := newTestModel()
	m.QueueVisible = true
	m.Focus = FocusNavigator

	// Navigator -> Queue
	m.handleFocusKeys("tab")
	if m.Focus != FocusQueue {
		t.Errorf("after first tab: Focus = %v, want FocusQueue", m.Focus)
	}

	// Queue -> Navigator
	m.handleFocusKeys("tab")
	if m.Focus != FocusNavigator {
		t.Errorf("after second tab: Focus = %v, want FocusNavigator", m.Focus)
	}
}

// --- Playback State Transition Tests ---

func TestPlaybackStateTransitions(t *testing.T) {
	tests := []struct {
		name         string
		initialState player.State
		action       func(m *Model)
		wantState    player.State
	}{
		{
			name:         "stop from playing",
			initialState: player.Playing,
			action:       func(m *Model) { m.handlePlaybackKeys("s") },
			wantState:    player.Stopped,
		},
		{
			name:         "stop from paused",
			initialState: player.Paused,
			action:       func(m *Model) { m.handlePlaybackKeys("s") },
			wantState:    player.Stopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			mock, ok := m.Player.(*player.Mock)
			if !ok {
				t.Fatal("expected mock player")
			}
			mock.SetState(tt.initialState)

			tt.action(m)

			if mock.State() != tt.wantState {
				t.Errorf("player state = %v, want %v", mock.State(), tt.wantState)
			}
		})
	}
}

// --- Home/End Navigation Tests ---

func TestHandlePlaybackKeys_Home(t *testing.T) {
	m := newTestModel()
	m.Queue.Add(playlist.Track{Path: "/1.mp3"})
	m.Queue.Add(playlist.Track{Path: "/2.mp3"})
	m.Queue.Add(playlist.Track{Path: "/3.mp3"})
	m.Queue.JumpTo(2) // Start at last track

	handled, _ := m.handlePlaybackKeys("home")

	if !handled {
		t.Error("expected 'home' to be handled")
	}
	if m.Queue.CurrentIndex() != 0 {
		t.Errorf("CurrentIndex = %d, want 0", m.Queue.CurrentIndex())
	}
}

func TestHandlePlaybackKeys_HomeReturnsCmd_WhenPlaying(t *testing.T) {
	m := newTestModel()
	m.Queue.Add(playlist.Track{Path: "/1.mp3"})
	m.Queue.Add(playlist.Track{Path: "/2.mp3"})
	m.Queue.JumpTo(1)
	mock, ok := m.Player.(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	_, cmd := m.handlePlaybackKeys("home")

	if cmd == nil {
		t.Error("expected command when playing")
	}
}

func TestHandlePlaybackKeys_End(t *testing.T) {
	m := newTestModel()
	m.Queue.Add(playlist.Track{Path: "/1.mp3"})
	m.Queue.Add(playlist.Track{Path: "/2.mp3"})
	m.Queue.Add(playlist.Track{Path: "/3.mp3"})
	m.Queue.JumpTo(0) // Start at first track

	handled, _ := m.handlePlaybackKeys("end")

	if !handled {
		t.Error("expected 'end' to be handled")
	}
	if m.Queue.CurrentIndex() != 2 {
		t.Errorf("CurrentIndex = %d, want 2", m.Queue.CurrentIndex())
	}
}

func TestHandlePlaybackKeys_EndReturnsCmd_WhenPlaying(t *testing.T) {
	m := newTestModel()
	m.Queue.Add(playlist.Track{Path: "/1.mp3"})
	m.Queue.Add(playlist.Track{Path: "/2.mp3"})
	m.Queue.JumpTo(0)
	mock, ok := m.Player.(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	_, cmd := m.handlePlaybackKeys("end")

	if cmd == nil {
		t.Error("expected command when playing")
	}
}

func TestHandlePlaybackKeys_HomeEmptyQueue(t *testing.T) {
	m := newTestModel()
	// Queue is empty

	handled, cmd := m.handlePlaybackKeys("home")

	if !handled {
		t.Error("expected 'home' to be handled even with empty queue")
	}
	if cmd != nil {
		t.Error("expected no command for empty queue")
	}
}

func TestHandlePlaybackKeys_EndEmptyQueue(t *testing.T) {
	m := newTestModel()
	// Queue is empty

	handled, cmd := m.handlePlaybackKeys("end")

	if !handled {
		t.Error("expected 'end' to be handled even with empty queue")
	}
	if cmd != nil {
		t.Error("expected no command for empty queue")
	}
}

// --- Unhandled Key Tests ---

func TestHandlePlaybackKeys_UnhandledReturnsNotHandled(t *testing.T) {
	unhandledKeys := []string{"x", "y", "1", "enter", "escape"}

	for _, key := range unhandledKeys {
		t.Run(key, func(t *testing.T) {
			m := newTestModel()
			handled, _ := m.handlePlaybackKeys(key)
			if handled {
				t.Errorf("expected %q to not be handled by playback handler", key)
			}
		})
	}
}

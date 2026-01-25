// internal/app/handlers_test.go
package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/playback"
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
			result := m.handleQuitKeys(tt.key)

			if tt.wantQuit {
				if !result.Handled {
					t.Errorf("expected %q to be handled", tt.key)
				}
				if result.Cmd == nil {
					t.Error("expected quit command")
				}
			} else if result.Handled {
				t.Errorf("expected %q to not be handled", tt.key)
			}
		})
	}
}

func TestHandleViewKeys(t *testing.T) {
	t.Run("unhandled key returns false", func(t *testing.T) {
		m := newTestModel()

		result := m.handleViewKeys("x") // Random unbound key

		if result.Handled {
			t.Error("expected 'x' to not be handled")
		}
	})

	// Note: f1/f2/f3/f4 tests require initialized navigators for SaveNavigationState
	// These are covered by integration tests
}

func TestHandleFocusKeys(t *testing.T) {
	t.Run("toggle queue visibility", func(t *testing.T) {
		m := newTestModel()
		m.Layout.ShowQueue()

		result := m.handleFocusKeys("p")

		if !result.Handled {
			t.Error("expected 'p' to be handled")
		}
		if m.Layout.IsQueueVisible() {
			t.Error("expected queue to be hidden")
		}
	})

	t.Run("hide queue resets focus", func(t *testing.T) {
		m := newTestModel()
		m.Layout.ShowQueue()
		m.Navigation.SetFocus(navctl.FocusQueue)

		m.handleFocusKeys("p")

		if m.Navigation.Focus() != navctl.FocusNavigator {
			t.Errorf("Focus = %v, want navctl.FocusNavigator", m.Navigation.Focus())
		}
	})

	t.Run("tab switches focus when queue visible", func(t *testing.T) {
		m := newTestModel()
		m.Layout.ShowQueue()
		m.Navigation.SetFocus(navctl.FocusNavigator)

		result := m.handleFocusKeys("tab")

		if !result.Handled {
			t.Error("expected 'tab' to be handled")
		}
		if m.Navigation.Focus() != navctl.FocusQueue {
			t.Errorf("Focus = %v, want navctl.FocusQueue", m.Navigation.Focus())
		}
	})

	t.Run("tab switches back from queue", func(t *testing.T) {
		m := newTestModel()
		m.Layout.ShowQueue()
		m.Navigation.SetFocus(navctl.FocusQueue)

		m.handleFocusKeys("tab")

		if m.Navigation.Focus() != navctl.FocusNavigator {
			t.Errorf("Focus = %v, want navctl.FocusNavigator", m.Navigation.Focus())
		}
	})

	t.Run("tab does nothing when queue hidden", func(t *testing.T) {
		m := newTestModel()
		m.Layout.HideQueue()
		m.Navigation.SetFocus(navctl.FocusNavigator)

		m.handleFocusKeys("tab")

		if m.Navigation.Focus() != navctl.FocusNavigator {
			t.Errorf("Focus = %v, want navctl.FocusNavigator", m.Navigation.Focus())
		}
	})
}

func TestHandlePlaybackKeys(t *testing.T) {
	t.Run("space toggles play/pause", func(t *testing.T) {
		m := newTestModel()
		mock, ok := m.PlaybackService.Player().(*player.Mock)
		if !ok {
			t.Fatal("expected mock player")
		}
		mock.SetState(player.Playing)

		result := m.handlePlaybackKeys(" ")

		if !result.Handled {
			t.Error("expected space to be handled")
		}
		if mock.State() != player.Paused {
			t.Errorf("player state = %v, want Paused", mock.State())
		}
	})

	t.Run("s stops player", func(t *testing.T) {
		m := newTestModel()
		mock, ok := m.PlaybackService.Player().(*player.Mock)
		if !ok {
			t.Fatal("expected mock player")
		}
		mock.SetState(player.Playing)

		result := m.handlePlaybackKeys("s")

		if !result.Handled {
			t.Error("expected 's' to be handled")
		}
		if mock.State() != player.Stopped {
			t.Errorf("player state = %v, want Stopped", mock.State())
		}
	})

	t.Run("R cycles repeat mode", func(t *testing.T) {
		m := newTestModel()
		initialMode := m.PlaybackService.RepeatMode()

		result := m.handlePlaybackKeys("R")

		if !result.Handled {
			t.Error("expected 'R' to be handled")
		}
		if m.PlaybackService.RepeatMode() == initialMode {
			t.Error("expected repeat mode to change")
		}
	})

	t.Run("S toggles shuffle", func(t *testing.T) {
		m := newTestModel()
		initialShuffle := m.PlaybackService.Shuffle()

		result := m.handlePlaybackKeys("S")

		if !result.Handled {
			t.Error("expected 'S' to be handled")
		}
		if m.PlaybackService.Shuffle() == initialShuffle {
			t.Error("expected shuffle to toggle")
		}
	})

	t.Run("v toggles player display", func(t *testing.T) {
		m := newTestModel()

		result := m.handlePlaybackKeys("v")

		if !result.Handled {
			t.Error("expected 'v' to be handled")
		}
	})
}

func TestHandleNavigatorActionKeys(t *testing.T) {
	t.Run("unhandled key returns false", func(t *testing.T) {
		m := newTestModel()

		result := m.handleNavigatorActionKeys("x")

		if result.Handled {
			t.Error("expected 'x' to not be handled")
		}
	})

	// Note: Tests for /, enter, a, r require initialized navigators/library
	// These are covered by integration tests
}

// newTestModel creates a minimal model for testing handlers.
func newTestModel() *Model {
	queue := playlist.NewQueue()
	p := player.NewMock()
	svc := playback.New(p, queue)
	return &Model{
		Navigation:      navctl.New(),
		Layout:          NewLayoutManager(queuepanel.New(queue)),
		Popups:          popupctl.New(),
		PlaybackService: svc,
		playbackSub:     svc.Subscribe(),
		Keys:            keymap.NewResolver(keymap.Bindings),
		StateMgr:        state.NewMock(),
	}
}

// Note: View mode transition tests (F1/F2) require initialized navigators
// because handleViewKeys calls SaveNavigationState. These are covered by
// integration tests in integration_test.go

// --- Focus Transition Tests ---

func TestFocusCycling(t *testing.T) {
	m := newTestModel()
	m.Layout.ShowQueue()
	m.Navigation.SetFocus(navctl.FocusNavigator)

	// Navigator -> Queue
	m.handleFocusKeys("tab")
	if m.Navigation.Focus() != navctl.FocusQueue {
		t.Errorf("after first tab: Focus = %v, want navctl.FocusQueue", m.Navigation.Focus())
	}

	// Queue -> Navigator
	m.handleFocusKeys("tab")
	if m.Navigation.Focus() != navctl.FocusNavigator {
		t.Errorf("after second tab: Focus = %v, want navctl.FocusNavigator", m.Navigation.Focus())
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
			mock, ok := m.PlaybackService.Player().(*player.Mock)
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
	m.PlaybackService.AddTracks(playback.Track{Path: "/1.mp3"})
	m.PlaybackService.AddTracks(playback.Track{Path: "/2.mp3"})
	m.PlaybackService.AddTracks(playback.Track{Path: "/3.mp3"})
	m.PlaybackService.QueueMoveTo(2) // Start at last track

	result := m.handlePlaybackKeys("home")

	if !result.Handled {
		t.Error("expected 'home' to be handled")
	}
	if m.PlaybackService.QueueCurrentIndex() != 0 {
		t.Errorf("CurrentIndex = %d, want 0", m.PlaybackService.QueueCurrentIndex())
	}
}

func TestHandlePlaybackKeys_HomeReturnsCmd_WhenPlaying(t *testing.T) {
	m := newTestModel()
	m.PlaybackService.AddTracks(playback.Track{Path: "/1.mp3"})
	m.PlaybackService.AddTracks(playback.Track{Path: "/2.mp3"})
	m.PlaybackService.QueueMoveTo(1)
	mock, ok := m.PlaybackService.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	result := m.handlePlaybackKeys("home")

	if result.Cmd == nil {
		t.Error("expected command when playing")
	}
}

func TestHandlePlaybackKeys_End(t *testing.T) {
	m := newTestModel()
	m.PlaybackService.AddTracks(playback.Track{Path: "/1.mp3"})
	m.PlaybackService.AddTracks(playback.Track{Path: "/2.mp3"})
	m.PlaybackService.AddTracks(playback.Track{Path: "/3.mp3"})
	m.PlaybackService.QueueMoveTo(0) // Start at first track

	result := m.handlePlaybackKeys("end")

	if !result.Handled {
		t.Error("expected 'end' to be handled")
	}
	if m.PlaybackService.QueueCurrentIndex() != 2 {
		t.Errorf("CurrentIndex = %d, want 2", m.PlaybackService.QueueCurrentIndex())
	}
}

func TestHandlePlaybackKeys_EndReturnsCmd_WhenPlaying(t *testing.T) {
	m := newTestModel()
	m.PlaybackService.AddTracks(playback.Track{Path: "/1.mp3"})
	m.PlaybackService.AddTracks(playback.Track{Path: "/2.mp3"})
	m.PlaybackService.QueueMoveTo(0)
	mock, ok := m.PlaybackService.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	result := m.handlePlaybackKeys("end")

	if result.Cmd == nil {
		t.Error("expected command when playing")
	}
}

func TestHandlePlaybackKeys_HomeEmptyQueue(t *testing.T) {
	m := newTestModel()
	// Queue is empty

	result := m.handlePlaybackKeys("home")

	if !result.Handled {
		t.Error("expected 'home' to be handled even with empty queue")
	}
	if result.Cmd != nil {
		t.Error("expected no command for empty queue")
	}
}

func TestHandlePlaybackKeys_EndEmptyQueue(t *testing.T) {
	m := newTestModel()
	// Queue is empty

	result := m.handlePlaybackKeys("end")

	if !result.Handled {
		t.Error("expected 'end' to be handled even with empty queue")
	}
	if result.Cmd != nil {
		t.Error("expected no command for empty queue")
	}
}

// --- Unhandled Key Tests ---

func TestHandlePlaybackKeys_UnhandledReturnsNotHandled(t *testing.T) {
	unhandledKeys := []string{"x", "y", "1", "enter", "escape"}

	for _, key := range unhandledKeys {
		t.Run(key, func(t *testing.T) {
			m := newTestModel()
			result := m.handlePlaybackKeys(key)
			if result.Handled {
				t.Errorf("expected %q to not be handled by playback handler", key)
			}
		})
	}
}

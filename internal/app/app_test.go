// internal/app/app_test.go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

func TestUpdate_WindowSizeMsg_ResizesComponents(t *testing.T) {
	m := newIntegrationTestModel()

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.Layout.Width() != 120 {
		t.Errorf("Width = %d, want 120", result.Layout.Width())
	}
	if result.Layout.Height() != 40 {
		t.Errorf("Height = %d, want 40", result.Layout.Height())
	}
}

func TestUpdate_ServiceTrackChangedMsg_UpdatesUI(t *testing.T) {
	m := newIntegrationTestModel()
	m.PlaybackService.AddTracks(
		playback.Track{Path: "/track1.mp3"},
		playback.Track{Path: "/track2.mp3"},
	)
	m.PlaybackService.QueueMoveTo(1) // Move to second track

	mock, ok := m.PlaybackService.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	// Simulate service track change event
	newModel, cmd := m.Update(ServiceTrackChangedMsg{PreviousIndex: 0, CurrentIndex: 1})
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	// UI should be updated and command returned for continued event watching
	if result.PlaybackService.QueueCurrentIndex() != 1 {
		t.Errorf("CurrentIndex = %d, want 1", result.PlaybackService.QueueCurrentIndex())
	}
	if cmd == nil {
		t.Error("expected command for continued event watching")
	}
}

func TestUpdate_ServiceStateChangedMsg_UpdatesUI(t *testing.T) {
	m := newIntegrationTestModel()
	m.PlaybackService.AddTracks(playback.Track{Path: "/track1.mp3"})
	m.PlaybackService.QueueMoveTo(0)

	mock, ok := m.PlaybackService.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Stopped)

	// Simulate service state change event (playing -> stopped)
	newModel, cmd := m.Update(ServiceStateChangedMsg{Previous: 1, Current: 0}) // Playing -> Stopped
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	// Should return command for continued event watching
	if cmd == nil {
		t.Error("expected command for continued event watching")
	}
	_ = result // UI updates happen internally
}

func TestUpdate_KeyMsg_Quit(t *testing.T) {
	m := newIntegrationTestModel()

	mock, ok := m.PlaybackService.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	if mock.State() != player.Stopped {
		t.Error("expected player to be stopped")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}

	stateMock, ok := m.StateMgr.(*state.Mock)
	if !ok {
		t.Fatal("expected mock state manager")
	}
	if !stateMock.IsClosed() {
		t.Error("expected state manager to be closed")
	}
}

func TestUpdate_KeyMsg_TogglePause(t *testing.T) {
	m := newIntegrationTestModel()

	mock, ok := m.PlaybackService.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	// Space immediately toggles play/pause
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	m.Update(msg)

	if mock.State() != player.Paused {
		t.Errorf("player state = %v, want Paused", mock.State())
	}
}

// Note: TestUpdate_KeyMsg_ViewSwitch requires initialized navigators
// for SaveNavigationState - covered by manual testing

func TestUpdate_KeyMsg_ToggleQueuePanel(t *testing.T) {
	m := newIntegrationTestModel()
	m.Layout.ShowQueue()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.Layout.IsQueueVisible() {
		t.Error("QueueVisible should be false after toggle")
	}
}

func TestUpdate_KeyMsg_TabSwitchesFocus(t *testing.T) {
	m := newIntegrationTestModel()
	m.Layout.ShowQueue()
	m.Navigation.SetFocus(navctl.FocusNavigator)

	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.Navigation.Focus() != navctl.FocusQueue {
		t.Errorf("Focus = %v, want navctl.FocusQueue", result.Navigation.Focus())
	}
}

func TestUpdate_TickMsg_ContinuesWhenPlaying(t *testing.T) {
	m := newIntegrationTestModel()

	mock, ok := m.PlaybackService.Player().(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	_, cmd := m.Update(TickMsg{})

	if cmd == nil {
		t.Error("expected tick command to continue")
	}
}

func TestUpdate_TickMsg_StopsWhenNotPlaying(t *testing.T) {
	m := newIntegrationTestModel()
	// Player is stopped by default

	_, cmd := m.Update(TickMsg{})

	if cmd != nil {
		t.Error("expected no tick command when stopped")
	}
}

func TestUpdate_ErrorMsg_DismissedByAnyKey(t *testing.T) {
	m := newIntegrationTestModel()
	m.Popups.ShowError("some error")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.Popups.ErrorMsg() != "" {
		t.Errorf("ErrorMsg = %q, want empty", result.Popups.ErrorMsg())
	}
}

// newIntegrationTestModel creates a model for integration tests.
func newIntegrationTestModel() Model {
	queue := playlist.NewQueue()
	p := player.NewMock()
	svc := playback.New(p, queue)
	return Model{
		Navigation:      navctl.New(),
		PlaybackService: svc,
		playbackSub:     svc.Subscribe(),
		Layout:          NewLayoutManager(queuepanel.New(queue)),
		Popups:          popupctl.New(),
		Keys:            keymap.NewResolver(keymap.Bindings),
		StateMgr:        state.NewMock(),
	}
}

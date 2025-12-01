// internal/app/app_test.go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

	if result.Width != 120 {
		t.Errorf("Width = %d, want 120", result.Width)
	}
	if result.Height != 40 {
		t.Errorf("Height = %d, want 40", result.Height)
	}
}

func TestUpdate_TrackFinishedMsg_AdvancesQueue(t *testing.T) {
	m := newIntegrationTestModel()
	m.Queue.Add(
		playlist.Track{Path: "/track1.mp3"},
		playlist.Track{Path: "/track2.mp3"},
	)
	m.Queue.JumpTo(0)

	mock, ok := m.Player.(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	newModel, cmd := m.Update(TrackFinishedMsg{})
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.Queue.CurrentIndex() != 1 {
		t.Errorf("CurrentIndex = %d, want 1", result.Queue.CurrentIndex())
	}
	if cmd == nil {
		t.Error("expected command for continued playback")
	}
}

func TestUpdate_TrackFinishedMsg_StopsAtEndOfQueue(t *testing.T) {
	m := newIntegrationTestModel()
	m.Queue.Add(playlist.Track{Path: "/track1.mp3"})
	m.Queue.JumpTo(0)

	mock, ok := m.Player.(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	newModel, _ := m.Update(TrackFinishedMsg{})
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	resultMock, ok := result.Player.(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	if resultMock.State() != player.Stopped {
		t.Errorf("player state = %v, want Stopped", resultMock.State())
	}
}

func TestUpdate_KeyMsg_Quit(t *testing.T) {
	m := newIntegrationTestModel()

	mock, ok := m.Player.(*player.Mock)
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

	mock, ok := m.Player.(*player.Mock)
	if !ok {
		t.Fatal("expected mock player")
	}
	mock.SetState(player.Playing)

	// First space sets pending keys, then timeout triggers pause
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.PendingKeys != " " {
		t.Errorf("PendingKeys = %q, want %q", result.PendingKeys, " ")
	}
}

// Note: TestUpdate_KeyMsg_ViewSwitch requires initialized navigators
// for SaveNavigationState - covered by manual testing

func TestUpdate_KeyMsg_ToggleQueuePanel(t *testing.T) {
	m := newIntegrationTestModel()
	m.QueueVisible = true

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.QueueVisible {
		t.Error("QueueVisible should be false after toggle")
	}
}

func TestUpdate_KeyMsg_TabSwitchesFocus(t *testing.T) {
	m := newIntegrationTestModel()
	m.QueueVisible = true
	m.Focus = FocusNavigator

	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.Focus != FocusQueue {
		t.Errorf("Focus = %v, want FocusQueue", result.Focus)
	}
}

func TestUpdate_TickMsg_ContinuesWhenPlaying(t *testing.T) {
	m := newIntegrationTestModel()

	mock, ok := m.Player.(*player.Mock)
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
	m.ErrorMsg = "some error"

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	newModel, _ := m.Update(msg)
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	if result.ErrorMsg != "" {
		t.Errorf("ErrorMsg = %q, want empty", result.ErrorMsg)
	}
}

// newIntegrationTestModel creates a model for integration tests.
func newIntegrationTestModel() Model {
	queue := playlist.NewQueue()
	return Model{
		Player:       player.NewMock(),
		Queue:        queue,
		QueuePanel:   queuepanel.New(queue),
		StateMgr:     state.NewMock(),
		ViewMode:     ViewLibrary,
		QueueVisible: true,
		Focus:        FocusNavigator,
	}
}

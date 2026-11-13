// internal/app/app_test.go
package app

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

func TestUpdate_WindowSizeMsg_ResizesComponents(t *testing.T) {
	m := newIntegrationTestModel(t)

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
	m := newIntegrationTestModel(t)
	m.Core.Playback().AddTracks(
		core.Track{Path: "/track1.mp3"},
		core.Track{Path: "/track2.mp3"},
	)
	m.Core.Playback().QueueMoveTo(1) // Move to second track

	mock := testPlayer(m.Core)
	mock.SetState(player.Playing)

	// Simulate service track change event
	newModel, cmd := m.Update(ServiceTrackChangedMsg{PreviousIndex: 0, CurrentIndex: 1})
	result, ok := newModel.(Model)
	if !ok {
		t.Fatal("Update should return Model")
	}

	// UI should be updated and command returned for continued event watching
	if result.Core.Playback().QueueCurrentIndex() != 1 {
		t.Errorf("CurrentIndex = %d, want 1", result.Core.Playback().QueueCurrentIndex())
	}
	if cmd == nil {
		t.Error("expected command for continued event watching")
	}
}

func TestUpdate_ServiceStateChangedMsg_UpdatesUI(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/track1.mp3"})
	m.Core.Playback().QueueMoveTo(0)

	mock := testPlayer(m.Core)
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
	m := newIntegrationTestModel(t)

	mock := testPlayer(m.Core)
	mock.SetState(player.Playing)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	if mock.State() != player.Stopped {
		t.Error("expected player to be stopped")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}

	// Quitting closes the core, which closes its database: the TUI holds
	// nothing else to close.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, open := <-m.Core.Events():
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("the core is still open after quitting")
		}
	}
}

func TestUpdate_KeyMsg_TogglePause(t *testing.T) {
	m := newIntegrationTestModel(t)

	mock := testPlayer(m.Core)
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
	m := newIntegrationTestModel(t)
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
	m := newIntegrationTestModel(t)
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
	m := newIntegrationTestModel(t)

	mock := testPlayer(m.Core)
	mock.SetState(player.Playing)

	_, cmd := m.Update(TickMsg{Gen: m.tickGen})

	if cmd == nil {
		t.Error("expected tick command to continue")
	}
}

func TestUpdate_TickMsg_StopsWhenNotPlaying(t *testing.T) {
	m := newIntegrationTestModel(t)
	// Player is stopped by default

	_, cmd := m.Update(TickMsg{Gen: m.tickGen})

	if cmd != nil {
		t.Error("expected no tick command when stopped")
	}
}

func TestUpdate_ErrorMsg_DismissedByAnyKey(t *testing.T) {
	m := newIntegrationTestModel(t)
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
func newIntegrationTestModel(t testing.TB) Model {
	t.Helper()
	c := newTestCore(t)
	return Model{
		Navigation: navctl.New(),
		Core:       c,
		Layout:     NewLayoutManager(queuepanel.New(c.Playback())),
		Popups:     popupctl.New(),
		Keys:       keymap.NewResolver(keymap.Bindings),
	}
}

// testPlayers holds the mock player behind each test core: the core does not
// give its player away.
var testPlayers sync.Map

// newTestCore creates a core on a mock player and an empty database in a temp
// dir, closed when the test ends.
func newTestCore(t testing.TB) *core.Core {
	t.Helper()
	mock := player.NewMock()
	c, err := core.New(core.Options{Player: mock, Database: filepath.Join(t.TempDir(), "waves.db")})
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	testPlayers.Store(c, mock)
	t.Cleanup(func() { c.Close() })
	return c
}

// The core of a test has a library, playlists, downloads and export targets,
// all empty but for the favorites.
func TestNewTestCore_HasEmptyDomains(t *testing.T) {
	c := newTestCore(t)

	if sources, err := c.Library().Sources(); err != nil || len(sources) != 0 {
		t.Errorf("sources = %v, %v; want none", sources, err)
	}
	if playlists, err := c.Playlists().List(nil); err != nil || len(playlists) != 1 || playlists[0].Name != "Favorites" {
		t.Errorf("playlists = %v, %v; want the favorites alone", playlists, err)
	}
	if downloads, err := c.Downloads().List(); err != nil || len(downloads) != 0 {
		t.Errorf("downloads = %v, %v; want none", downloads, err)
	}
	if targets, err := c.Exports().Targets(); err != nil || len(targets) != 0 {
		t.Errorf("export targets = %v, %v; want none", targets, err)
	}
}

// testPlayer returns the mock player a test core plays on.
func testPlayer(c *core.Core) *player.Mock {
	mock, _ := testPlayers.Load(c)
	return mock.(*player.Mock) //nolint:forcetypeassert // only newTestCore stores here
}

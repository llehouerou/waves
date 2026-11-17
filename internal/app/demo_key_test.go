package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
)

// ctrlAltD is the message Bubble Tea delivers for Ctrl+Alt+D, the key of the
// secret demo binding. The test builds the message, never a key name: a name
// no terminal produces would match nothing (issue #213).
var ctrlAltD = tea.KeyMsg{Type: tea.KeyCtrlD, Alt: true}

// newDemoKeyModel returns a model in the given view, with the queue visible
// and focused.
func newDemoKeyModel(t *testing.T, view navctl.ViewMode) Model {
	t.Helper()
	m := newIntegrationTestModel(t)
	m.Navigation.SetViewMode(view)
	m.Layout.ShowQueue()
	m.SetFocus(navctl.FocusQueue)
	return m
}

func TestDemoKey_ForcesAlbumViewWithNavigatorFocus(t *testing.T) {
	for _, view := range []navctl.ViewMode{
		navctl.ViewFileBrowser, navctl.ViewPlaylists, navctl.ViewDownloads, navctl.ViewLibrary,
	} {
		t.Run(string(view), func(t *testing.T) {
			m := newDemoKeyModel(t, view)

			m, _ = updateModel(t, m, ctrlAltD)

			if got := m.Navigation.ViewMode(); got != navctl.ViewLibrary {
				t.Errorf("view mode = %v, want %v", got, navctl.ViewLibrary)
			}
			if !m.Navigation.IsAlbumViewActive() {
				t.Error("the album view is not active")
			}
			if !m.Navigation.IsNavigatorFocused() {
				t.Error("the navigator does not have the focus")
			}
			if m.Layout.QueuePanel().IsFocused() {
				t.Error("the queue panel still reports itself focused")
			}
		})
	}
}

func TestDemoKey_SwitchesLibraryBrowserToAlbumView(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeBrowser)

	m, _ = updateModel(t, m, ctrlAltD)

	if !m.Navigation.IsAlbumViewActive() {
		t.Error("the album view is not active")
	}
}

func TestDemoKey_OtherKeysDoNotTriggerIt(t *testing.T) {
	for name, msg := range map[string]tea.KeyMsg{
		"ctrl+d": {Type: tea.KeyCtrlD},
		"alt+d":  {Type: tea.KeyRunes, Runes: []rune{'d'}, Alt: true},
	} {
		t.Run(name, func(t *testing.T) {
			m := newDemoKeyModel(t, navctl.ViewFileBrowser)

			m, _ = updateModel(t, m, msg)

			if m.Navigation.IsAlbumViewActive() {
				t.Error("the album view is active")
			}
		})
	}
}

func TestDemoKey_PopupOrSearchGetsTheKeyFirst(t *testing.T) {
	t.Run("popup", func(t *testing.T) {
		m := newDemoKeyModel(t, navctl.ViewFileBrowser)
		m.Popups.ShowError("Oops")

		m, _ = updateModel(t, m, ctrlAltD)

		if got := m.Navigation.ViewMode(); got != navctl.ViewFileBrowser {
			t.Errorf("view mode = %v, want %v", got, navctl.ViewFileBrowser)
		}
	})

	t.Run("search", func(t *testing.T) {
		m := newDemoKeyModel(t, navctl.ViewFileBrowser)
		m.Input = NewInputManager()
		m.Input.StartLocalSearch(nil)

		m, _ = updateModel(t, m, ctrlAltD)

		if got := m.Navigation.ViewMode(); got != navctl.ViewFileBrowser {
			t.Errorf("view mode = %v, want %v", got, navctl.ViewFileBrowser)
		}
	})
}

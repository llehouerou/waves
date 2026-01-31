package albumview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/albumpreset"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestPresetsPopup(presets []Preset, current albumpreset.Settings) *testutil.PopupHarness {
	p := NewPresetsPopup()
	p.Show(presets, current, 80, 24)
	return testutil.NewPopupHarness(p)
}

func getPresetsAction(t *testing.T, h *testutil.PopupHarness) action.Action {
	t.Helper()
	cmd := h.LastCommand()
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	msg := testutil.ExecuteCmd(cmd)
	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}
	return actionMsg.Action
}

// Sample presets for testing
func samplePresets() []Preset {
	return []Preset{
		{ID: 1, Name: "By Artist", Settings: albumpreset.Settings{
			GroupFields: []GroupField{GroupFieldArtist},
		}},
		{ID: 2, Name: "By Year", Settings: albumpreset.Settings{
			GroupFields: []GroupField{GroupFieldYear},
		}},
		{ID: 3, Name: "Recent", Settings: albumpreset.Settings{
			SortCriteria: []SortCriterion{{Field: SortFieldAddedAt, Order: SortDesc}},
		}},
	}
}

// List mode tests

func TestPresetsPopup_Close(t *testing.T) {
	h := newTestPresetsPopup(samplePresets(), albumpreset.Settings{})

	h.SendEscape()

	act := getPresetsAction(t, h)
	if _, ok := act.(PresetsClosed); !ok {
		t.Fatalf("expected PresetsClosed, got %T", act)
	}
}

func TestPresetsPopup_NavigateDown(t *testing.T) {
	presets := samplePresets()
	h := newTestPresetsPopup(presets, albumpreset.Settings{})

	h.SendDown()
	h.SendEnter()

	act := getPresetsAction(t, h)
	loaded, ok := act.(PresetLoaded)
	if !ok {
		t.Fatalf("expected PresetLoaded, got %T", act)
	}
	if loaded.PresetID != 2 {
		t.Errorf("PresetID = %d, want 2 (By Year)", loaded.PresetID)
	}
}

func TestPresetsPopup_NavigateWithJK(t *testing.T) {
	presets := samplePresets()
	h := newTestPresetsPopup(presets, albumpreset.Settings{})

	h.SendKey("j") // -> By Year
	h.SendKey("j") // -> Recent
	h.SendKey("k") // -> By Year
	h.SendEnter()

	act := getPresetsAction(t, h)
	loaded, ok := act.(PresetLoaded)
	if !ok {
		t.Fatalf("expected PresetLoaded, got %T", act)
	}
	if loaded.PresetID != 2 {
		t.Errorf("PresetID = %d, want 2", loaded.PresetID)
	}
}

func TestPresetsPopup_LoadPreset(t *testing.T) {
	presets := samplePresets()
	h := newTestPresetsPopup(presets, albumpreset.Settings{})

	h.SendEnter() // Load first preset

	act := getPresetsAction(t, h)
	loaded, ok := act.(PresetLoaded)
	if !ok {
		t.Fatalf("expected PresetLoaded, got %T", act)
	}
	if loaded.PresetID != 1 {
		t.Errorf("PresetID = %d, want 1 (By Artist)", loaded.PresetID)
	}
	if loaded.Settings.PresetName != "By Artist" {
		t.Errorf("PresetName = %q, want 'By Artist'", loaded.Settings.PresetName)
	}
}

func TestPresetsPopup_DeletePreset(t *testing.T) {
	presets := samplePresets()
	h := newTestPresetsPopup(presets, albumpreset.Settings{})

	h.SendDown() // Select "By Year"
	h.SendKey("d")

	act := getPresetsAction(t, h)
	deleted, ok := act.(PresetDeleted)
	if !ok {
		t.Fatalf("expected PresetDeleted, got %T", act)
	}
	if deleted.ID != 2 {
		t.Errorf("ID = %d, want 2", deleted.ID)
	}
}

func TestPresetsPopup_DeleteOnEmptyDoesNothing(t *testing.T) {
	h := newTestPresetsPopup(nil, albumpreset.Settings{})
	h.ClearCommands()

	h.SendKey("d")

	if len(h.Commands()) != 0 {
		t.Error("delete on empty list should not produce command")
	}
}

func TestPresetsPopup_NavigationBounds(t *testing.T) {
	presets := samplePresets()
	h := newTestPresetsPopup(presets, albumpreset.Settings{})

	// Try to go above first
	h.SendUp()
	h.SendUp()
	h.SendEnter()

	act := getPresetsAction(t, h)
	loaded, ok := act.(PresetLoaded)
	if !ok {
		t.Fatalf("expected PresetLoaded, got %T", act)
	}
	if loaded.PresetID != 1 {
		t.Errorf("PresetID = %d, want 1 (should stay at first)", loaded.PresetID)
	}
}

func TestPresetsPopup_NavigationBoundsBottom(t *testing.T) {
	presets := samplePresets()
	h := newTestPresetsPopup(presets, albumpreset.Settings{})

	// Try to go below last
	h.SendDown()
	h.SendDown()
	h.SendDown()
	h.SendDown()
	h.SendEnter()

	act := getPresetsAction(t, h)
	loaded, ok := act.(PresetLoaded)
	if !ok {
		t.Fatalf("expected PresetLoaded, got %T", act)
	}
	if loaded.PresetID != 3 {
		t.Errorf("PresetID = %d, want 3 (should stay at last)", loaded.PresetID)
	}
}

// Save mode tests

func TestPresetsPopup_EnterSaveMode(t *testing.T) {
	h := newTestPresetsPopup(samplePresets(), albumpreset.Settings{})

	h.SendKey("s")

	if err := h.AssertViewContains("Save Preset"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Name:"); err != "" {
		t.Error(err)
	}
}

func TestPresetsPopup_SavePreset(t *testing.T) {
	current := albumpreset.Settings{
		GroupFields: []GroupField{GroupFieldGenre},
	}
	h := newTestPresetsPopup(nil, current)

	h.SendKey("s") // Enter save mode
	h.SendKey("M")
	h.SendKey("y")
	h.SendKey(" ")
	h.SendKey("P")
	h.SendKey("r")
	h.SendKey("e")
	h.SendKey("s")
	h.SendKey("e")
	h.SendKey("t")
	h.SendEnter()

	act := getPresetsAction(t, h)
	saved, ok := act.(PresetSaved)
	if !ok {
		t.Fatalf("expected PresetSaved, got %T", act)
	}
	if saved.Name != "My Preset" {
		t.Errorf("Name = %q, want 'My Preset'", saved.Name)
	}
	if len(saved.Settings.GroupFields) != 1 || saved.Settings.GroupFields[0] != GroupFieldGenre {
		t.Errorf("Settings not preserved: %+v", saved.Settings)
	}
}

func TestPresetsPopup_SaveEmptyNameDoesNothing(t *testing.T) {
	h := newTestPresetsPopup(nil, albumpreset.Settings{})
	h.ClearCommands()

	h.SendKey("s") // Enter save mode
	h.SendEnter()  // Try to save with empty name

	// Should still be in save mode
	if err := h.AssertViewContains("Save Preset"); err != "" {
		t.Error(err)
	}
}

func TestPresetsPopup_CancelSaveMode(t *testing.T) {
	h := newTestPresetsPopup(samplePresets(), albumpreset.Settings{})

	h.SendKey("s") // Enter save mode
	h.SendKey("t")
	h.SendKey("e")
	h.SendKey("s")
	h.SendKey("t")
	h.SendEscape() // Cancel

	// Should be back in list mode
	if err := h.AssertViewContains("Album View Presets"); err != "" {
		t.Error(err)
	}
}

func TestPresetsPopup_BackspaceInSaveMode(t *testing.T) {
	h := newTestPresetsPopup(nil, albumpreset.Settings{})

	h.SendKey("s") // Enter save mode
	h.SendKey("a")
	h.SendKey("b")
	h.SendKey("c")
	h.SendSpecialKey(tea.KeyBackspace)
	h.SendSpecialKey(tea.KeyBackspace)
	h.SendEnter()

	act := getPresetsAction(t, h)
	saved, ok := act.(PresetSaved)
	if !ok {
		t.Fatalf("expected PresetSaved, got %T", act)
	}
	if saved.Name != "a" {
		t.Errorf("Name = %q, want 'a'", saved.Name)
	}
}

// View tests

func TestPresetsPopup_ViewShowsTitle(t *testing.T) {
	h := newTestPresetsPopup(samplePresets(), albumpreset.Settings{})

	if err := h.AssertViewContains("Album View Presets"); err != "" {
		t.Error(err)
	}
}

func TestPresetsPopup_ViewShowsPresets(t *testing.T) {
	h := newTestPresetsPopup(samplePresets(), albumpreset.Settings{})

	if err := h.AssertViewContains("By Artist"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("By Year"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Recent"); err != "" {
		t.Error(err)
	}
}

func TestPresetsPopup_ViewShowsEmptyMessage(t *testing.T) {
	h := newTestPresetsPopup(nil, albumpreset.Settings{})

	if err := h.AssertViewContains("No saved presets"); err != "" {
		t.Error(err)
	}
}

func TestPresetsPopup_ViewShowsHints(t *testing.T) {
	h := newTestPresetsPopup(samplePresets(), albumpreset.Settings{})

	if err := h.AssertViewContains("navigate"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("load"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("save"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("delete"); err != "" {
		t.Error(err)
	}
}

func TestPresetsPopup_EmptyViewWhenNoSize(t *testing.T) {
	p := NewPresetsPopup()
	p.Show(samplePresets(), albumpreset.Settings{}, 0, 0)
	h := testutil.NewPopupHarness(p)

	if h.View() != "" {
		t.Errorf("view = %q, want empty when no size", h.View())
	}
}

// Inactive state tests

func TestPresetsPopup_InactiveIgnoresInput(t *testing.T) {
	p := NewPresetsPopup() // Not shown
	h := testutil.NewPopupHarness(p)
	h.ClearCommands()

	h.SendKey("j")
	h.SendEnter()

	if len(h.Commands()) != 0 {
		t.Error("inactive popup should not produce commands")
	}
}

func TestPresetsPopup_InactiveEmptyView(t *testing.T) {
	p := NewPresetsPopup() // Not shown
	h := testutil.NewPopupHarness(p)

	if h.View() != "" {
		t.Errorf("inactive popup view = %q, want empty", h.View())
	}
}

// Reset test

func TestPresetsPopup_Reset(t *testing.T) {
	p := NewPresetsPopup()
	p.Show(samplePresets(), albumpreset.Settings{}, 80, 24)

	if !p.Active() {
		t.Error("expected Active=true after Show")
	}

	p.Reset()

	if p.Active() {
		t.Error("expected Active=false after Reset")
	}
}

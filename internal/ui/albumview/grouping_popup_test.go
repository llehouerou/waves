package albumview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestGroupingPopup(fields []GroupField, sortOrder SortOrder, dateField DateFieldType) *testutil.PopupHarness {
	p := NewGroupingPopup()
	p.Show(fields, sortOrder, dateField, 80, 24)
	return testutil.NewPopupHarness(p)
}

func getGroupingApplied(t *testing.T, h *testutil.PopupHarness) GroupingApplied {
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
	result, ok := actionMsg.Action.(GroupingApplied)
	if !ok {
		t.Fatalf("expected GroupingApplied, got %T", actionMsg.Action)
	}
	return result
}

func assertGroupingCanceled(t *testing.T, h *testutil.PopupHarness) {
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
	if _, ok := actionMsg.Action.(GroupingCanceled); !ok {
		t.Fatalf("expected GroupingCanceled, got %T", actionMsg.Action)
	}
}

// Navigation tests

func TestGroupingPopup_NavigateDown(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	// Navigate down to second field (Genre)
	h.SendDown()

	// Select it and apply
	h.SendKey(" ")
	h.SendEnter()

	result := getGroupingApplied(t, h)
	if len(result.Fields) != 1 || result.Fields[0] != GroupFieldGenre {
		t.Errorf("Fields = %v, want [GroupFieldGenre]", result.Fields)
	}
}

func TestGroupingPopup_NavigateWithJK(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	// Navigate with j (down) and k (up)
	h.SendKey("j") // -> Genre
	h.SendKey("j") // -> Label
	h.SendKey("j") // -> Year
	h.SendKey("k") // -> Label

	h.SendKey(" ") // Select Label
	h.SendEnter()

	result := getGroupingApplied(t, h)
	if len(result.Fields) != 1 || result.Fields[0] != GroupFieldLabel {
		t.Errorf("Fields = %v, want [GroupFieldLabel]", result.Fields)
	}
}

func TestGroupingPopup_NavigationBoundsTop(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	// Try to go above first
	h.SendUp()
	h.SendUp()

	// Should still be at Artist (first field)
	h.SendKey(" ")
	h.SendEnter()

	result := getGroupingApplied(t, h)
	if len(result.Fields) != 1 || result.Fields[0] != GroupFieldArtist {
		t.Errorf("Fields = %v, want [GroupFieldArtist]", result.Fields)
	}
}

// Field selection tests

func TestGroupingPopup_ToggleSelection(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	// Select first field (Artist)
	h.SendKey(" ")

	// Move down and select another (Genre)
	h.SendDown()
	h.SendKey(" ")

	h.SendEnter()

	result := getGroupingApplied(t, h)
	if len(result.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result.Fields))
	}
	if result.Fields[0] != GroupFieldArtist {
		t.Errorf("first field = %v, want GroupFieldArtist", result.Fields[0])
	}
	if result.Fields[1] != GroupFieldGenre {
		t.Errorf("second field = %v, want GroupFieldGenre", result.Fields[1])
	}
}

func TestGroupingPopup_DeselectField(t *testing.T) {
	initial := []GroupField{GroupFieldArtist, GroupFieldGenre}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Deselect Artist (cursor starts at 0)
	h.SendKey(" ")

	h.SendEnter()

	result := getGroupingApplied(t, h)
	if len(result.Fields) != 1 {
		t.Fatalf("expected 1 field after deselect, got %d", len(result.Fields))
	}
	if result.Fields[0] != GroupFieldGenre {
		t.Errorf("remaining field = %v, want GroupFieldGenre", result.Fields[0])
	}
}

// Sort order tests

func TestGroupingPopup_PreservesSortOrder(t *testing.T) {
	initial := []GroupField{GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortAsc, DateFieldBest)

	h.SendEnter()

	result := getGroupingApplied(t, h)
	if result.SortOrder != SortAsc {
		t.Errorf("SortOrder = %v, want SortAsc", result.SortOrder)
	}
}

func TestGroupingPopup_ChangeSortOrder(t *testing.T) {
	initial := []GroupField{GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Navigate to sort order option row (past all fields)
	for range GroupFieldCount {
		h.SendDown()
	}

	// Toggle sort order with space
	h.SendKey(" ")

	h.SendEnter()

	result := getGroupingApplied(t, h)
	if result.SortOrder != SortAsc {
		t.Errorf("SortOrder = %v, want SortAsc", result.SortOrder)
	}
}

func TestGroupingPopup_ChangeSortOrderWithArrows(t *testing.T) {
	initial := []GroupField{GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortAsc, DateFieldBest)

	// Navigate to sort order option row
	for range GroupFieldCount {
		h.SendDown()
	}

	// Toggle with right arrow
	h.SendKey("l")

	h.SendEnter()

	result := getGroupingApplied(t, h)
	if result.SortOrder != SortDesc {
		t.Errorf("SortOrder = %v, want SortDesc after toggle", result.SortOrder)
	}
}

// Date field tests

func TestGroupingPopup_DateFieldShownForDateGrouping(t *testing.T) {
	initial := []GroupField{GroupFieldYear}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// View should show date field option since Year is a date-based field
	if err := h.AssertViewContains("Date field"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_DateFieldHiddenForNonDateGrouping(t *testing.T) {
	initial := []GroupField{GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// View should not show date field option since Artist is not date-based
	if err := h.AssertViewNotContains("Date field"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_CycleDateField(t *testing.T) {
	initial := []GroupField{GroupFieldMonth}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Navigate to date field option (past fields and sort order)
	for range GroupFieldCount {
		h.SendDown()
	}
	h.SendDown() // Past sort order to date field

	// Cycle with space
	h.SendKey(" ")

	h.SendEnter()

	result := getGroupingApplied(t, h)
	// DateFieldBest -> DateFieldOriginal
	if result.DateField != DateFieldOriginal {
		t.Errorf("DateField = %v, want DateFieldOriginal", result.DateField)
	}
}

func TestGroupingPopup_CycleDateFieldBackward(t *testing.T) {
	initial := []GroupField{GroupFieldWeek}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldOriginal)

	// Navigate to date field option
	for range GroupFieldCount {
		h.SendDown()
	}
	h.SendDown() // Past sort order to date field

	// Cycle backward with left arrow
	h.SendKey("h")

	h.SendEnter()

	result := getGroupingApplied(t, h)
	// DateFieldOriginal -> DateFieldBest (wraps around)
	if result.DateField != DateFieldBest {
		t.Errorf("DateField = %v, want DateFieldBest", result.DateField)
	}
}

// Field reordering tests

func TestGroupingPopup_ReorderUp(t *testing.T) {
	initial := []GroupField{GroupFieldArtist, GroupFieldGenre}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Move to Genre (position 1 in UI)
	h.SendDown()

	// Move it up with K
	h.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})

	h.SendEnter()

	result := getGroupingApplied(t, h)
	if len(result.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result.Fields))
	}
	// Order should now be swapped
	if result.Fields[0] != GroupFieldGenre {
		t.Errorf("first = %v, want GroupFieldGenre", result.Fields[0])
	}
	if result.Fields[1] != GroupFieldArtist {
		t.Errorf("second = %v, want GroupFieldArtist", result.Fields[1])
	}
}

func TestGroupingPopup_ReorderDown(t *testing.T) {
	initial := []GroupField{GroupFieldArtist, GroupFieldGenre}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Cursor is at Artist (position 0)
	// Move it down with J
	h.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})

	h.SendEnter()

	result := getGroupingApplied(t, h)
	// Order should now be swapped
	if result.Fields[0] != GroupFieldGenre {
		t.Errorf("first = %v, want GroupFieldGenre", result.Fields[0])
	}
	if result.Fields[1] != GroupFieldArtist {
		t.Errorf("second = %v, want GroupFieldArtist", result.Fields[1])
	}
}

func TestGroupingPopup_ReorderOnlyIfSelected(t *testing.T) {
	initial := []GroupField{GroupFieldGenre} // Only Genre is selected
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Cursor is at Artist (position 0, not selected)
	// Try to reorder - should do nothing
	h.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})

	h.SendEnter()

	result := getGroupingApplied(t, h)
	// Should still be just Genre
	if len(result.Fields) != 1 || result.Fields[0] != GroupFieldGenre {
		t.Errorf("Fields = %v, want [GroupFieldGenre]", result.Fields)
	}
}

// Cancel test

func TestGroupingPopup_Cancel(t *testing.T) {
	initial := []GroupField{GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Make some changes
	h.SendKey(" ") // Deselect
	h.SendDown()   // Move down
	h.SendKey(" ") // Select different field
	h.SendEscape() // Cancel

	assertGroupingCanceled(t, h)
}

// Apply without changes test

func TestGroupingPopup_ApplyWithoutChanges(t *testing.T) {
	initial := []GroupField{GroupFieldYear, GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortAsc, DateFieldRelease)

	h.SendEnter() // Apply immediately without changes

	result := getGroupingApplied(t, h)
	if len(result.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result.Fields))
	}
	if result.Fields[0] != GroupFieldYear {
		t.Errorf("first = %v, want GroupFieldYear", result.Fields[0])
	}
	if result.Fields[1] != GroupFieldArtist {
		t.Errorf("second = %v, want GroupFieldArtist", result.Fields[1])
	}
	if result.SortOrder != SortAsc {
		t.Errorf("SortOrder = %v, want SortAsc", result.SortOrder)
	}
	if result.DateField != DateFieldRelease {
		t.Errorf("DateField = %v, want DateFieldRelease", result.DateField)
	}
}

// Navigation between sections tests

func TestGroupingPopup_NavigateToOptions(t *testing.T) {
	initial := []GroupField{GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Navigate past all fields to options section
	for range GroupFieldCount {
		h.SendDown()
	}

	// Should be on sort order option - view should show cursor there
	if err := h.AssertViewContains("Order:"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_NavigateBackFromOptions(t *testing.T) {
	initial := []GroupField{GroupFieldArtist}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Navigate to options
	for range GroupFieldCount {
		h.SendDown()
	}

	// Navigate back up
	h.SendUp()

	// Should be back in field list at last field (Added)
	h.SendKey(" ") // Toggle Added
	h.SendEnter()

	result := getGroupingApplied(t, h)
	// Should have Artist and Added
	if len(result.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result.Fields))
	}
}

func TestGroupingPopup_NoOptionsWhenNoSelection(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	// Try to navigate past fields - should stay at last field
	for range GroupFieldCount + 5 {
		h.SendDown()
	}

	// Select last field and apply
	h.SendKey(" ")
	h.SendEnter()

	result := getGroupingApplied(t, h)
	// Last field is Added
	if len(result.Fields) != 1 || result.Fields[0] != GroupFieldAddedAt {
		t.Errorf("Fields = %v, want [GroupFieldAddedAt]", result.Fields)
	}
}

// View tests

func TestGroupingPopup_ViewShowsTitle(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	if err := h.AssertViewContains("Album Grouping"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_ViewShowsFields(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	if err := h.AssertViewContains("Artist"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Genre"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Year"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_ViewShowsSelectionOrder(t *testing.T) {
	initial := []GroupField{GroupFieldGenre, GroupFieldYear}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	// Selected fields should show order numbers
	if err := h.AssertViewContains("[1]"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("[2]"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_ViewShowsSummary(t *testing.T) {
	initial := []GroupField{GroupFieldArtist, GroupFieldYear}
	h := newTestGroupingPopup(initial, SortDesc, DateFieldBest)

	if err := h.AssertViewContains("Group by:"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Artist"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_ViewShowsNoGroupingWhenEmpty(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	if err := h.AssertViewContains("No grouping"); err != "" {
		t.Error(err)
	}
}

func TestGroupingPopup_ViewShowsHints(t *testing.T) {
	h := newTestGroupingPopup(nil, SortDesc, DateFieldBest)

	if err := h.AssertViewContains("navigate"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("toggle"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("reorder"); err != "" {
		t.Error(err)
	}
}

// Inactive state tests

func TestGroupingPopup_InactiveIgnoresInput(t *testing.T) {
	p := NewGroupingPopup() // Not shown
	h := testutil.NewPopupHarness(p)
	h.ClearCommands()

	h.SendKey(" ")
	h.SendEnter()

	if len(h.Commands()) != 0 {
		t.Error("inactive popup should not produce commands")
	}
}

func TestGroupingPopup_InactiveEmptyView(t *testing.T) {
	p := NewGroupingPopup() // Not shown
	h := testutil.NewPopupHarness(p)

	if h.View() != "" {
		t.Errorf("inactive popup view = %q, want empty", h.View())
	}
}

func TestGroupingPopup_EmptyViewWhenNoSize(t *testing.T) {
	p := NewGroupingPopup()
	p.Show(nil, SortDesc, DateFieldBest, 0, 0)
	h := testutil.NewPopupHarness(p)

	if h.View() != "" {
		t.Errorf("view = %q, want empty when no size", h.View())
	}
}

// Reset test

func TestGroupingPopup_Reset(t *testing.T) {
	p := NewGroupingPopup()
	p.Show([]GroupField{GroupFieldArtist}, SortDesc, DateFieldBest, 80, 24)

	if !p.Active() {
		t.Error("expected Active=true after Show")
	}

	p.Reset()

	if p.Active() {
		t.Error("expected Active=false after Reset")
	}
}

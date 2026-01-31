package albumview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestSortingPopup(initial []SortCriterion) *testutil.PopupHarness {
	p := NewSortingPopup()
	p.Show(initial, 80, 24)
	return testutil.NewPopupHarness(p)
}

func getSortingApplied(t *testing.T, h *testutil.PopupHarness) SortingApplied {
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
	result, ok := actionMsg.Action.(SortingApplied)
	if !ok {
		t.Fatalf("expected SortingApplied, got %T", actionMsg.Action)
	}
	return result
}

func assertCanceled(t *testing.T, h *testutil.PopupHarness) {
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
	if _, ok := actionMsg.Action.(SortingCanceled); !ok {
		t.Fatalf("expected SortingCanceled, got %T", actionMsg.Action)
	}
}

// Navigation tests

func TestSortingPopup_NavigateDown(t *testing.T) {
	h := newTestSortingPopup(nil)

	// Cursor starts at 0, move down
	h.SendDown()
	h.SendDown()

	// Select current field and verify cursor moved
	h.SendKey(" ") // Toggle selection at cursor position 2
	h.SendEnter()

	result := getSortingApplied(t, h)
	if len(result.Criteria) != 1 {
		t.Fatalf("expected 1 criterion, got %d", len(result.Criteria))
	}
	// Position 2 is SortFieldAddedAt (0=OriginalDate, 1=ReleaseDate, 2=AddedAt)
	if result.Criteria[0].Field != SortFieldAddedAt {
		t.Errorf("Field = %v, want SortFieldAddedAt", result.Criteria[0].Field)
	}
}

func TestSortingPopup_NavigateWithJK(t *testing.T) {
	h := newTestSortingPopup(nil)

	// Navigate with j (down) and k (up)
	h.SendKey("j") // -> 1
	h.SendKey("j") // -> 2
	h.SendKey("j") // -> 3
	h.SendKey("k") // -> 2

	h.SendKey(" ") // Toggle at position 2
	h.SendEnter()

	result := getSortingApplied(t, h)
	if len(result.Criteria) != 1 || result.Criteria[0].Field != SortFieldAddedAt {
		t.Errorf("expected SortFieldAddedAt, got %v", result.Criteria)
	}
}

func TestSortingPopup_NavigationBounds(t *testing.T) {
	h := newTestSortingPopup(nil)

	// Try to go above first
	h.SendUp()
	h.SendUp()

	h.SendKey(" ") // Should still be at position 0
	h.SendEnter()

	result := getSortingApplied(t, h)
	if len(result.Criteria) != 1 || result.Criteria[0].Field != SortFieldOriginalDate {
		t.Errorf("expected SortFieldOriginalDate at position 0, got %v", result.Criteria)
	}
}

// Selection tests

func TestSortingPopup_ToggleSelection(t *testing.T) {
	h := newTestSortingPopup(nil)

	// Select first field
	h.SendKey(" ")

	// Move down and select another
	h.SendDown()
	h.SendKey(" ")

	h.SendEnter()

	result := getSortingApplied(t, h)
	if len(result.Criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(result.Criteria))
	}
	if result.Criteria[0].Field != SortFieldOriginalDate {
		t.Errorf("first criterion = %v, want SortFieldOriginalDate", result.Criteria[0].Field)
	}
	if result.Criteria[1].Field != SortFieldReleaseDate {
		t.Errorf("second criterion = %v, want SortFieldReleaseDate", result.Criteria[1].Field)
	}
}

func TestSortingPopup_DeselectField(t *testing.T) {
	initial := []SortCriterion{
		{Field: SortFieldOriginalDate, Order: SortDesc},
		{Field: SortFieldArtist, Order: SortAsc},
	}
	h := newTestSortingPopup(initial)

	// Deselect the first field (cursor starts at 0)
	h.SendKey(" ")

	h.SendEnter()

	result := getSortingApplied(t, h)
	if len(result.Criteria) != 1 {
		t.Fatalf("expected 1 criterion after deselect, got %d", len(result.Criteria))
	}
	if result.Criteria[0].Field != SortFieldArtist {
		t.Errorf("remaining criterion = %v, want SortFieldArtist", result.Criteria[0].Field)
	}
}

// Order tests

func TestSortingPopup_DefaultOrderIsDesc(t *testing.T) {
	h := newTestSortingPopup(nil)

	h.SendKey(" ") // Select first field
	h.SendEnter()

	result := getSortingApplied(t, h)
	if result.Criteria[0].Order != SortDesc {
		t.Errorf("Order = %v, want SortDesc", result.Criteria[0].Order)
	}
}

func TestSortingPopup_ChangeToAsc(t *testing.T) {
	initial := []SortCriterion{{Field: SortFieldOriginalDate, Order: SortDesc}}
	h := newTestSortingPopup(initial)

	h.SendKey("a") // Change to ascending
	h.SendEnter()

	result := getSortingApplied(t, h)
	if result.Criteria[0].Order != SortAsc {
		t.Errorf("Order = %v, want SortAsc", result.Criteria[0].Order)
	}
}

func TestSortingPopup_ChangeToDesc(t *testing.T) {
	initial := []SortCriterion{{Field: SortFieldOriginalDate, Order: SortAsc}}
	h := newTestSortingPopup(initial)

	h.SendKey("d") // Change to descending
	h.SendEnter()

	result := getSortingApplied(t, h)
	if result.Criteria[0].Order != SortDesc {
		t.Errorf("Order = %v, want SortDesc", result.Criteria[0].Order)
	}
}

func TestSortingPopup_OrderChangeOnlyIfSelected(t *testing.T) {
	h := newTestSortingPopup(nil) // Nothing selected

	h.SendKey("a") // Should do nothing since field not selected
	h.SendKey(" ") // Now select it
	h.SendEnter()

	result := getSortingApplied(t, h)
	// Should still be default (desc) since 'a' was pressed before selection
	if result.Criteria[0].Order != SortDesc {
		t.Errorf("Order = %v, want SortDesc (order change before selection should be ignored)", result.Criteria[0].Order)
	}
}

// Reorder tests

func TestSortingPopup_ReorderUp(t *testing.T) {
	initial := []SortCriterion{
		{Field: SortFieldOriginalDate, Order: SortDesc},
		{Field: SortFieldReleaseDate, Order: SortAsc},
	}
	h := newTestSortingPopup(initial)

	// Move to second field (ReleaseDate is at position 1)
	h.SendDown()

	// Move it up with K
	h.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})

	h.SendEnter()

	result := getSortingApplied(t, h)
	if len(result.Criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(result.Criteria))
	}
	// Order should now be swapped
	if result.Criteria[0].Field != SortFieldReleaseDate {
		t.Errorf("first = %v, want SortFieldReleaseDate", result.Criteria[0].Field)
	}
	if result.Criteria[1].Field != SortFieldOriginalDate {
		t.Errorf("second = %v, want SortFieldOriginalDate", result.Criteria[1].Field)
	}
}

func TestSortingPopup_ReorderDown(t *testing.T) {
	initial := []SortCriterion{
		{Field: SortFieldOriginalDate, Order: SortDesc},
		{Field: SortFieldReleaseDate, Order: SortAsc},
	}
	h := newTestSortingPopup(initial)

	// Cursor is at first field (OriginalDate at position 0)
	// Move it down with J
	h.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})

	h.SendEnter()

	result := getSortingApplied(t, h)
	// Order should be swapped
	if result.Criteria[0].Field != SortFieldReleaseDate {
		t.Errorf("first = %v, want SortFieldReleaseDate", result.Criteria[0].Field)
	}
	if result.Criteria[1].Field != SortFieldOriginalDate {
		t.Errorf("second = %v, want SortFieldOriginalDate", result.Criteria[1].Field)
	}
}

// Cancel test

func TestSortingPopup_Cancel(t *testing.T) {
	initial := []SortCriterion{{Field: SortFieldOriginalDate, Order: SortDesc}}
	h := newTestSortingPopup(initial)

	// Make some changes
	h.SendKey(" ") // Deselect
	h.SendDown()   // Move down
	h.SendKey(" ") // Select different field
	h.SendEscape() // Cancel

	assertCanceled(t, h)
}

// Apply with initial values

func TestSortingPopup_ApplyWithoutChanges(t *testing.T) {
	initial := []SortCriterion{
		{Field: SortFieldArtist, Order: SortAsc},
		{Field: SortFieldAlbum, Order: SortDesc},
	}
	h := newTestSortingPopup(initial)

	h.SendEnter() // Apply immediately without changes

	result := getSortingApplied(t, h)
	if len(result.Criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(result.Criteria))
	}
	if result.Criteria[0].Field != SortFieldArtist || result.Criteria[0].Order != SortAsc {
		t.Errorf("first = %+v, want Artist Asc", result.Criteria[0])
	}
	if result.Criteria[1].Field != SortFieldAlbum || result.Criteria[1].Order != SortDesc {
		t.Errorf("second = %+v, want Album Desc", result.Criteria[1])
	}
}

// View tests

func TestSortingPopup_View(t *testing.T) {
	h := newTestSortingPopup(nil)

	if err := h.AssertViewContains("Album Sorting"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Original Date"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Artist"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("navigate"); err != "" {
		t.Error(err)
	}
}

func TestSortingPopup_ViewShowsSelectedOrder(t *testing.T) {
	initial := []SortCriterion{{Field: SortFieldOriginalDate, Order: SortAsc}}
	h := newTestSortingPopup(initial)

	if err := h.AssertViewContains("asc"); err != "" {
		t.Error(err)
	}
}

func TestSortingPopup_ViewShowsSummary(t *testing.T) {
	initial := []SortCriterion{
		{Field: SortFieldArtist, Order: SortAsc},
	}
	h := newTestSortingPopup(initial)

	if err := h.AssertViewContains("Sort by:"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Artist"); err != "" {
		t.Error(err)
	}
}

func TestSortingPopup_ViewDefaultSummaryWhenEmpty(t *testing.T) {
	h := newTestSortingPopup(nil)

	if err := h.AssertViewContains("Default sort order"); err != "" {
		t.Error(err)
	}
}

// Inactive state

func TestSortingPopup_InactiveIgnoresInput(t *testing.T) {
	p := NewSortingPopup() // Not shown
	h := testutil.NewPopupHarness(p)
	h.ClearCommands()

	h.SendKey(" ")
	h.SendEnter()

	if len(h.Commands()) != 0 {
		t.Error("inactive popup should not produce commands")
	}
}

func TestSortingPopup_InactiveEmptyView(t *testing.T) {
	p := NewSortingPopup() // Not shown
	h := testutil.NewPopupHarness(p)

	if h.View() != "" {
		t.Errorf("inactive popup view = %q, want empty", h.View())
	}
}

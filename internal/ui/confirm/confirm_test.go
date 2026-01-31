package confirm

import (
	"testing"

	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

const testContext = "ctx"

func newTestConfirm(title, message string, context any) *testutil.PopupHarness {
	m := New()
	m.Show(title, message, context, 80, 24)
	return testutil.NewPopupHarness(&m)
}

func newTestConfirmWithOptions(title, message string, options []string, context any) *testutil.PopupHarness {
	m := New()
	m.ShowWithOptions(title, message, options, context, 80, 24)
	return testutil.NewPopupHarness(&m)
}

func getResult(t *testing.T, h *testutil.PopupHarness) Result {
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
	result, ok := actionMsg.Action.(Result)
	if !ok {
		t.Fatalf("expected Result, got %T", actionMsg.Action)
	}
	return result
}

// Yes/No mode tests

func TestYesNoMode_ConfirmWithEnter(t *testing.T) {
	h := newTestConfirm("Delete?", "Are you sure?", testContext)

	h.SendEnter()

	result := getResult(t, h)
	if !result.Confirmed {
		t.Error("expected Confirmed=true")
	}
	if result.Context != testContext {
		t.Errorf("Context = %v, want %q", result.Context, testContext)
	}
}

func TestYesNoMode_ConfirmWithY(t *testing.T) {
	h := newTestConfirm("Delete?", "Are you sure?", nil)

	h.SendKey("y")

	result := getResult(t, h)
	if !result.Confirmed {
		t.Error("expected Confirmed=true with 'y'")
	}
}

func TestYesNoMode_ConfirmWithUpperY(t *testing.T) {
	h := newTestConfirm("Delete?", "Are you sure?", nil)

	h.SendKey("Y")

	result := getResult(t, h)
	if !result.Confirmed {
		t.Error("expected Confirmed=true with 'Y'")
	}
}

func TestYesNoMode_CancelWithEscape(t *testing.T) {
	h := newTestConfirm("Delete?", "Are you sure?", testContext)

	h.SendEscape()

	result := getResult(t, h)
	if result.Confirmed {
		t.Error("expected Confirmed=false")
	}
	if result.Context != testContext {
		t.Errorf("Context = %v, want %q", result.Context, testContext)
	}
}

func TestYesNoMode_CancelWithN(t *testing.T) {
	h := newTestConfirm("Delete?", "Are you sure?", nil)

	h.SendKey("n")

	result := getResult(t, h)
	if result.Confirmed {
		t.Error("expected Confirmed=false with 'n'")
	}
}

func TestYesNoMode_CancelWithUpperN(t *testing.T) {
	h := newTestConfirm("Delete?", "Are you sure?", nil)

	h.SendKey("N")

	result := getResult(t, h)
	if result.Confirmed {
		t.Error("expected Confirmed=false with 'N'")
	}
}

func TestYesNoMode_View(t *testing.T) {
	h := newTestConfirm("Delete File?", "This cannot be undone", nil)

	if err := h.AssertViewContains("Delete File?"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("This cannot be undone"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Enter/Y: confirm"); err != "" {
		t.Error(err)
	}
}

// Multi-option mode tests

func TestMultiOption_SelectFirstOption(t *testing.T) {
	options := []string{"Delete", "Move to Trash", "Cancel"}
	h := newTestConfirmWithOptions("Action", "Choose action", options, testContext)

	// First option is selected by default
	h.SendEnter()

	result := getResult(t, h)
	if !result.Confirmed {
		t.Error("expected Confirmed=true for non-cancel option")
	}
	if result.SelectedOption != 0 {
		t.Errorf("SelectedOption = %d, want 0", result.SelectedOption)
	}
	if result.Context != testContext {
		t.Errorf("Context = %v, want %q", result.Context, testContext)
	}
}

func TestMultiOption_NavigateAndSelect(t *testing.T) {
	options := []string{"Delete", "Move to Trash", "Cancel"}
	h := newTestConfirmWithOptions("Action", "Choose action", options, nil)

	// Navigate down to second option
	h.SendDown()
	h.SendEnter()

	result := getResult(t, h)
	if !result.Confirmed {
		t.Error("expected Confirmed=true for non-cancel option")
	}
	if result.SelectedOption != 1 {
		t.Errorf("SelectedOption = %d, want 1", result.SelectedOption)
	}
}

func TestMultiOption_NavigateWithJK(t *testing.T) {
	options := []string{"A", "B", "C", "Cancel"}
	h := newTestConfirmWithOptions("Test", "msg", options, nil)

	// Navigate with j (down) and k (up)
	h.SendKey("j") // -> B
	h.SendKey("j") // -> C
	h.SendKey("k") // -> B
	h.SendEnter()

	result := getResult(t, h)
	if result.SelectedOption != 1 {
		t.Errorf("SelectedOption = %d, want 1 (B)", result.SelectedOption)
	}
}

func TestMultiOption_SelectCancel(t *testing.T) {
	options := []string{"Delete", "Cancel"}
	h := newTestConfirmWithOptions("Action", "msg", options, nil)

	// Navigate to Cancel (last option)
	h.SendDown()
	h.SendEnter()

	result := getResult(t, h)
	if result.Confirmed {
		t.Error("expected Confirmed=false for Cancel option")
	}
	if result.SelectedOption != 1 {
		t.Errorf("SelectedOption = %d, want 1", result.SelectedOption)
	}
}

func TestMultiOption_EscapeSelectsCancel(t *testing.T) {
	options := []string{"Delete", "Move", "Cancel"}
	h := newTestConfirmWithOptions("Action", "msg", options, nil)

	h.SendEscape()

	result := getResult(t, h)
	if result.Confirmed {
		t.Error("expected Confirmed=false on escape")
	}
	// Escape should select the last option (Cancel)
	if result.SelectedOption != 2 {
		t.Errorf("SelectedOption = %d, want 2 (Cancel)", result.SelectedOption)
	}
}

func TestMultiOption_NavigationBounds(t *testing.T) {
	options := []string{"A", "B", "Cancel"}
	h := newTestConfirmWithOptions("Test", "msg", options, nil)

	// Try to go above first option
	h.SendUp()
	h.SendUp()
	h.SendEnter()

	result := getResult(t, h)
	if result.SelectedOption != 0 {
		t.Errorf("SelectedOption = %d, want 0 (should stay at first)", result.SelectedOption)
	}
}

func TestMultiOption_NavigationBoundsBottom(t *testing.T) {
	options := []string{"A", "Cancel"}
	h := newTestConfirmWithOptions("Test", "msg", options, nil)

	// Try to go below last option
	h.SendDown()
	h.SendDown()
	h.SendDown()
	h.SendEnter()

	result := getResult(t, h)
	if result.SelectedOption != 1 {
		t.Errorf("SelectedOption = %d, want 1 (should stay at last)", result.SelectedOption)
	}
}

func TestMultiOption_View(t *testing.T) {
	options := []string{"Delete", "Move to Trash", "Cancel"}
	h := newTestConfirmWithOptions("Choose Action", "Pick one", options, nil)

	if err := h.AssertViewContains("Choose Action"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Pick one"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Delete"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Move to Trash"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Cancel"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("navigate"); err != "" {
		t.Error(err)
	}
}

// Inactive state tests

func TestInactive_NoCommandOnKey(t *testing.T) {
	m := New() // Not shown, inactive
	h := testutil.NewPopupHarness(&m)
	h.ClearCommands()

	h.SendEnter()
	h.SendKey("y")

	if len(h.Commands()) != 0 {
		t.Error("inactive popup should not produce commands")
	}
}

func TestInactive_EmptyView(t *testing.T) {
	m := New() // Not shown, inactive
	h := testutil.NewPopupHarness(&m)

	if h.View() != "" {
		t.Errorf("inactive popup view = %q, want empty", h.View())
	}
}

func TestReset(t *testing.T) {
	m := New()
	m.Show("Title", "Message", "context", 80, 24)

	if !m.Active() {
		t.Error("expected Active=true after Show")
	}

	m.Reset()

	if m.Active() {
		t.Error("expected Active=false after Reset")
	}
}

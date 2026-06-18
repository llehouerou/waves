package textinput

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

const testContext = "test-ctx"

func newTestInput(title, initialText string, context any) *testutil.PopupHarness {
	m := New()
	m.Start(title, initialText, context, 80, 24)
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

// Basic input tests

func TestTextInput_TypeCharacters(t *testing.T) {
	h := newTestInput("Name", "", nil)

	h.SendKey("h")
	h.SendKey("e")
	h.SendKey("l")
	h.SendKey("l")
	h.SendKey("o")
	h.SendEnter()

	result := getResult(t, h)
	if result.Text != "hello" {
		t.Errorf("Text = %q, want %q", result.Text, "hello")
	}
	if result.Canceled {
		t.Error("expected Canceled=false")
	}
}

func TestTextInput_InitialText(t *testing.T) {
	h := newTestInput("Edit", "initial", nil)

	h.SendEnter()

	result := getResult(t, h)
	if result.Text != "initial" {
		t.Errorf("Text = %q, want %q", result.Text, "initial")
	}
}

func TestTextInput_AppendToInitialText(t *testing.T) {
	h := newTestInput("Edit", "hello", nil)

	h.SendKey(" ")
	h.SendKey("w")
	h.SendKey("o")
	h.SendKey("r")
	h.SendKey("l")
	h.SendKey("d")
	h.SendEnter()

	result := getResult(t, h)
	if result.Text != "hello world" {
		t.Errorf("Text = %q, want %q", result.Text, "hello world")
	}
}

func TestTextInput_Backspace(t *testing.T) {
	h := newTestInput("Edit", "hello", nil)

	h.SendSpecialKey(tea.KeyBackspace)
	h.SendSpecialKey(tea.KeyBackspace)
	h.SendEnter()

	result := getResult(t, h)
	if result.Text != "hel" {
		t.Errorf("Text = %q, want %q", result.Text, "hel")
	}
}

func TestTextInput_BackspaceOnEmpty(t *testing.T) {
	h := newTestInput("Name", "", nil)

	// Backspace on empty should not panic
	h.SendSpecialKey(tea.KeyBackspace)
	h.SendSpecialKey(tea.KeyBackspace)
	h.SendEnter()

	result := getResult(t, h)
	if result.Text != "" {
		t.Errorf("Text = %q, want empty", result.Text)
	}
}

// Cancel tests

func TestTextInput_Cancel(t *testing.T) {
	h := newTestInput("Name", "typed", testContext)

	h.SendEscape()

	result := getResult(t, h)
	if !result.Canceled {
		t.Error("expected Canceled=true")
	}
	if result.Context != testContext {
		t.Errorf("Context = %v, want %q", result.Context, testContext)
	}
}

// Context passthrough tests

func TestTextInput_ContextPassthrough(t *testing.T) {
	h := newTestInput("Title", "", testContext)

	h.SendKey("x")
	h.SendEnter()

	result := getResult(t, h)
	if result.Context != testContext {
		t.Errorf("Context = %v, want %q", result.Context, testContext)
	}
}

// View tests

func TestTextInput_View(t *testing.T) {
	h := newTestInput("Enter Name", "", nil)

	if err := h.AssertViewContains("Enter Name"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains(">"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Enter: confirm"); err != "" {
		t.Error(err)
	}
}

func TestTextInput_ViewShowsText(t *testing.T) {
	h := newTestInput("Name", "", nil)

	h.SendKey("t")
	h.SendKey("e")
	h.SendKey("s")
	h.SendKey("t")

	if err := h.AssertViewContains("test"); err != "" {
		t.Error(err)
	}
}

func TestTextInput_ViewShowsInitialText(t *testing.T) {
	h := newTestInput("Edit", "preset", nil)

	if err := h.AssertViewContains("preset"); err != "" {
		t.Error(err)
	}
}

func TestTextInput_EmptyViewWhenNoSize(t *testing.T) {
	m := New()
	m.Start("Title", "", nil, 0, 0) // No size set
	h := testutil.NewPopupHarness(&m)

	if h.View() != "" {
		t.Errorf("View = %q, want empty when size is 0", h.View())
	}
}

// Reset test

func TestTextInput_Reset(t *testing.T) {
	m := New()
	m.Start("Title", "text", "context", 80, 24)

	m.Reset()

	// After reset, view should be empty (no title set)
	h := testutil.NewPopupHarness(&m)
	// The title should be empty
	if err := h.AssertViewNotContains("Title"); err != "" {
		t.Error(err)
	}
}

// Non-printable characters test

func TestTextInput_IgnoresControlCharacters(t *testing.T) {
	h := newTestInput("Name", "", nil)

	h.SendKey("a")
	h.SendTab() // Should be ignored
	h.SendKey("b")
	h.SendEnter()

	result := getResult(t, h)
	// Tab should not appear in the text
	if result.Text != "ab" {
		t.Errorf("Text = %q, want %q", result.Text, "ab")
	}
}

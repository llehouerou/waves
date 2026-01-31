package testutil

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// mockPopup is a simple popup implementation for testing the harness.
type mockPopup struct {
	content    string
	width      int
	height     int
	keyHistory []string
}

var _ popup.Popup = (*mockPopup)(nil)

func newMockPopup(content string) *mockPopup {
	return &mockPopup{content: content}
}

func (m *mockPopup) Init() tea.Cmd {
	return func() tea.Msg { return "init" }
}

func (m *mockPopup) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		m.keyHistory = append(m.keyHistory, key.String())
		if key.Type == tea.KeyEnter {
			return m, func() tea.Msg { return "enter-pressed" }
		}
	}
	return m, nil
}

func (m *mockPopup) View() string {
	return m.content
}

func (m *mockPopup) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func TestNewPopupHarness(t *testing.T) {
	mock := newMockPopup("test content")
	h := NewPopupHarness(mock)

	if h.Popup() != mock {
		t.Error("Popup() should return the underlying popup")
	}

	// Init command should be captured
	if len(h.Commands()) != 1 {
		t.Errorf("expected 1 init command, got %d", len(h.Commands()))
	}
}

func TestPopupHarness_SetSize(t *testing.T) {
	mock := newMockPopup("test")
	h := NewPopupHarness(mock)

	h.SetSize(80, 24)

	if mock.width != 80 || mock.height != 24 {
		t.Errorf("SetSize not propagated: got %dx%d, want 80x24", mock.width, mock.height)
	}
}

func TestPopupHarness_View(t *testing.T) {
	const content = "test content view"
	mock := newMockPopup(content)
	h := NewPopupHarness(mock)

	if h.View() != content {
		t.Errorf("View() = %q, want %q", h.View(), content)
	}
}

func TestPopupHarness_SendKey(t *testing.T) {
	mock := newMockPopup("test")
	h := NewPopupHarness(mock)
	h.ClearCommands()

	h.SendKey("a")
	h.SendKey("b")

	if len(mock.keyHistory) != 2 {
		t.Errorf("expected 2 keys, got %d", len(mock.keyHistory))
	}
	if mock.keyHistory[0] != "a" || mock.keyHistory[1] != "b" {
		t.Errorf("key history = %v, want [a, b]", mock.keyHistory)
	}
}

func TestPopupHarness_SendSpecialKeys(t *testing.T) {
	mock := newMockPopup("test")
	h := NewPopupHarness(mock)
	h.ClearCommands()

	h.SendEnter()
	h.SendEscape()
	h.SendUp()
	h.SendDown()
	h.SendTab()

	if len(mock.keyHistory) != 5 {
		t.Errorf("expected 5 keys, got %d", len(mock.keyHistory))
	}

	expected := []string{"enter", "esc", "up", "down", "tab"}
	for i, exp := range expected {
		if mock.keyHistory[i] != exp {
			t.Errorf("key %d = %q, want %q", i, mock.keyHistory[i], exp)
		}
	}
}

func TestPopupHarness_Commands(t *testing.T) {
	mock := newMockPopup("test")
	h := NewPopupHarness(mock)
	h.ClearCommands()

	// SendEnter should trigger a command from our mock
	h.SendEnter()

	if len(h.Commands()) != 1 {
		t.Errorf("expected 1 command, got %d", len(h.Commands()))
	}

	last := h.LastCommand()
	if last == nil {
		t.Fatal("LastCommand() returned nil")
	}

	// Execute the command to verify it works
	msg := ExecuteCmd(last)
	if msg != "enter-pressed" {
		t.Errorf("command result = %v, want 'enter-pressed'", msg)
	}
}

func TestPopupHarness_ClearCommands(t *testing.T) {
	mock := newMockPopup("test")
	h := NewPopupHarness(mock)

	if len(h.Commands()) == 0 {
		t.Error("expected init command before clear")
	}

	h.ClearCommands()

	if len(h.Commands()) != 0 {
		t.Error("expected no commands after clear")
	}
	if h.LastCommand() != nil {
		t.Error("LastCommand() should be nil after clear")
	}
}

func TestPopupHarness_ExecuteAndSend(t *testing.T) {
	mock := newMockPopup("test")
	h := NewPopupHarness(mock)
	h.ClearCommands()

	// Create a command that returns a key message
	cmd := func() tea.Msg {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}

	msg, resultCmd := h.ExecuteAndSend(cmd)

	if msg == nil {
		t.Error("expected message from ExecuteAndSend")
	}

	// The mock returns a command when enter is pressed
	if resultCmd == nil {
		t.Error("expected result command from enter key")
	}
}

func TestPopupHarness_ViewContains(t *testing.T) {
	mock := newMockPopup("Hello World")
	h := NewPopupHarness(mock)

	if !h.ViewContains("Hello") {
		t.Error("ViewContains should find 'Hello'")
	}
	if h.ViewContains("Goodbye") {
		t.Error("ViewContains should not find 'Goodbye'")
	}
}

func TestPopupHarness_AssertViewContains(t *testing.T) {
	mock := newMockPopup("Hello World")
	h := NewPopupHarness(mock)

	if err := h.AssertViewContains("Hello"); err != "" {
		t.Errorf("unexpected error: %s", err)
	}
	if err := h.AssertViewContains("Missing"); err == "" {
		t.Error("expected error for missing content")
	}
}

func TestPopupHarness_AssertViewNotContains(t *testing.T) {
	mock := newMockPopup("Hello World")
	h := NewPopupHarness(mock)

	if err := h.AssertViewNotContains("Missing"); err != "" {
		t.Errorf("unexpected error: %s", err)
	}
	if err := h.AssertViewNotContains("Hello"); err == "" {
		t.Error("expected error for present content")
	}
}

func TestExecuteCmd_Nil(t *testing.T) {
	msg := ExecuteCmd(nil)
	if msg != nil {
		t.Errorf("ExecuteCmd(nil) = %v, want nil", msg)
	}
}

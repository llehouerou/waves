package testutil

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// PopupHarness wraps a popup for testing, providing helpers to simulate
// user interactions and inspect state.
type PopupHarness struct {
	popup popup.Popup
	cmds  []tea.Cmd
}

// NewPopupHarness creates a test harness for any popup.Popup implementation.
// It initializes the popup and captures any init commands.
func NewPopupHarness(p popup.Popup) *PopupHarness {
	h := &PopupHarness{popup: p}
	if cmd := p.Init(); cmd != nil {
		h.cmds = append(h.cmds, cmd)
	}
	return h
}

// Popup returns the underlying popup for type assertion when needed.
func (h *PopupHarness) Popup() popup.Popup {
	return h.popup
}

// SetSize sets the popup dimensions.
func (h *PopupHarness) SetSize(width, height int) {
	h.popup.SetSize(width, height)
}

// View returns the popup's rendered content.
func (h *PopupHarness) View() string {
	return h.popup.View()
}

// SendMsg sends any message to the popup and returns the resulting command.
func (h *PopupHarness) SendMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	h.popup, cmd = h.popup.Update(msg)
	if cmd != nil {
		h.cmds = append(h.cmds, cmd)
	}
	return cmd
}

// SendKey simulates a key press by creating a tea.KeyMsg.
func (h *PopupHarness) SendKey(key string) tea.Cmd {
	return h.SendMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

// SendSpecialKey sends a special key (enter, escape, tab, etc.).
func (h *PopupHarness) SendSpecialKey(keyType tea.KeyType) tea.Cmd {
	return h.SendMsg(tea.KeyMsg{Type: keyType})
}

// SendEnter sends the enter key.
func (h *PopupHarness) SendEnter() tea.Cmd {
	return h.SendSpecialKey(tea.KeyEnter)
}

// SendEscape sends the escape key.
func (h *PopupHarness) SendEscape() tea.Cmd {
	return h.SendSpecialKey(tea.KeyEscape)
}

// SendUp sends the up arrow key.
func (h *PopupHarness) SendUp() tea.Cmd {
	return h.SendSpecialKey(tea.KeyUp)
}

// SendDown sends the down arrow key.
func (h *PopupHarness) SendDown() tea.Cmd {
	return h.SendSpecialKey(tea.KeyDown)
}

// SendTab sends the tab key.
func (h *PopupHarness) SendTab() tea.Cmd {
	return h.SendSpecialKey(tea.KeyTab)
}

// Commands returns all commands collected since creation or last ClearCommands.
func (h *PopupHarness) Commands() []tea.Cmd {
	return h.cmds
}

// LastCommand returns the most recent command, or nil if none.
func (h *PopupHarness) LastCommand() tea.Cmd {
	if len(h.cmds) == 0 {
		return nil
	}
	return h.cmds[len(h.cmds)-1]
}

// ClearCommands clears the collected commands.
func (h *PopupHarness) ClearCommands() {
	h.cmds = nil
}

// ExecuteCmd runs a command and returns the resulting message.
// This is useful for testing async command flows.
func ExecuteCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// ExecuteAndSend runs a command and sends its result back to the popup.
// Returns the message that was sent and the resulting command.
func (h *PopupHarness) ExecuteAndSend(cmd tea.Cmd) (tea.Msg, tea.Cmd) {
	msg := ExecuteCmd(cmd)
	if msg == nil {
		return nil, nil
	}
	resultCmd := h.SendMsg(msg)
	return msg, resultCmd
}

// ViewContains checks if the popup's view contains the given substring.
func (h *PopupHarness) ViewContains(substr string) bool {
	return ContainsLine(StripANSI(h.View()), substr)
}

// AssertViewContains returns an error message if view doesn't contain substr.
func (h *PopupHarness) AssertViewContains(substr string) string {
	return AssertContains(h.View(), substr)
}

// AssertViewNotContains returns an error message if view contains substr.
func (h *PopupHarness) AssertViewNotContains(substr string) string {
	return AssertNotContains(h.View(), substr)
}

package statepopup

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockPhase is a simple phase implementation for testing.
type mockPhase struct {
	name          string
	canGoBack     bool
	nextPhase     Phase
	closeOnUpdate bool
}

func (p *mockPhase) Name() string { return p.name }

func (p *mockPhase) Update(_ tea.Msg) (Phase, tea.Cmd) {
	if p.closeOnUpdate {
		return nil, nil
	}
	if p.nextPhase != nil {
		return p.nextPhase, nil
	}
	return p, nil
}

func (p *mockPhase) View() string { return "View: " + p.name }

func (p *mockPhase) CanGoBack() bool { return p.canGoBack }

func TestNewMachine(t *testing.T) {
	phase := &mockPhase{name: "initial"}
	m := NewMachine(phase)

	if m.Current() != phase {
		t.Error("Current() should return initial phase")
	}
	if m.HistoryDepth() != 0 {
		t.Errorf("HistoryDepth() = %d, want 0", m.HistoryDepth())
	}
}

func TestMachine_Advance(t *testing.T) {
	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2"}
	m := NewMachine(phase1)

	m.Advance(phase2)

	if m.Current() != phase2 {
		t.Error("Current() should be phase2 after Advance")
	}
	if m.HistoryDepth() != 1 {
		t.Errorf("HistoryDepth() = %d, want 1", m.HistoryDepth())
	}
}

func TestMachine_Back(t *testing.T) {
	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2", canGoBack: true}
	m := NewMachine(phase1)
	m.Advance(phase2)

	if !m.CanGoBack() {
		t.Error("CanGoBack() should be true")
	}

	if !m.Back() {
		t.Error("Back() should return true")
	}

	if m.Current() != phase1 {
		t.Error("Current() should be phase1 after Back")
	}
	if m.HistoryDepth() != 0 {
		t.Errorf("HistoryDepth() = %d, want 0", m.HistoryDepth())
	}
}

func TestMachine_Back_NotAllowed(t *testing.T) {
	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2", canGoBack: false} // Doesn't allow back
	m := NewMachine(phase1)
	m.Advance(phase2)

	if m.CanGoBack() {
		t.Error("CanGoBack() should be false when phase doesn't allow it")
	}

	if m.Back() {
		t.Error("Back() should return false when not allowed")
	}

	if m.Current() != phase2 {
		t.Error("Current() should still be phase2")
	}
}

func TestMachine_Back_EmptyHistory(t *testing.T) {
	phase := &mockPhase{name: "initial", canGoBack: true}
	m := NewMachine(phase)

	if m.CanGoBack() {
		t.Error("CanGoBack() should be false with empty history")
	}

	if m.Back() {
		t.Error("Back() should return false with empty history")
	}
}

func TestMachine_Update_PhaseTransition(t *testing.T) {
	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2"}
	phase1.nextPhase = phase2 // Will transition on any update

	m := NewMachine(phase1)

	closed, _ := m.Update(tea.KeyMsg{})

	if closed {
		t.Error("Update should not close")
	}
	if m.Current() != phase2 {
		t.Error("Current() should be phase2 after transition")
	}
	if m.HistoryDepth() != 1 {
		t.Error("History should contain phase1")
	}
}

func TestMachine_Update_Close(t *testing.T) {
	phase := &mockPhase{name: "phase", closeOnUpdate: true}
	m := NewMachine(phase)

	closed, _ := m.Update(tea.KeyMsg{})

	if !closed {
		t.Error("Update should signal close when phase returns nil")
	}
}

func TestMachine_Update_BackMsg(t *testing.T) {
	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2", canGoBack: true}
	m := NewMachine(phase1)
	m.Advance(phase2)

	closed, _ := m.Update(BackMsg{})

	if closed {
		t.Error("BackMsg should not close")
	}
	if m.Current() != phase1 {
		t.Error("Current() should be phase1 after BackMsg")
	}
}

func TestMachine_Update_CloseMsg(t *testing.T) {
	phase := &mockPhase{name: "phase"}
	m := NewMachine(phase)

	closed, _ := m.Update(CloseMsg{})

	if !closed {
		t.Error("CloseMsg should close the machine")
	}
}

func TestMachine_View(t *testing.T) {
	phase := &mockPhase{name: "test"}
	m := NewMachine(phase)

	view := m.View()

	if view != "View: test" {
		t.Errorf("View() = %q, want %q", view, "View: test")
	}
}

func TestMachine_View_NilPhase(t *testing.T) {
	m := NewMachine(nil)

	view := m.View()

	if view != "" {
		t.Errorf("View() = %q, want empty string", view)
	}
}

func TestMachine_Reset(t *testing.T) {
	phase1 := &mockPhase{name: "phase1"}
	phase2 := &mockPhase{name: "phase2"}
	phase3 := &mockPhase{name: "phase3"}

	m := NewMachine(phase1)
	m.Advance(phase2)
	m.Advance(phase3)

	if m.HistoryDepth() != 2 {
		t.Errorf("HistoryDepth() = %d before reset, want 2", m.HistoryDepth())
	}

	m.Reset(phase1)

	if m.Current() != phase1 {
		t.Error("Current() should be phase1 after Reset")
	}
	if m.HistoryDepth() != 0 {
		t.Errorf("HistoryDepth() = %d after reset, want 0", m.HistoryDepth())
	}
}

func TestTransitionCmd(t *testing.T) {
	phase := &mockPhase{name: "next"}
	cmd := TransitionCmd(phase, true)

	msg := cmd()

	trans, ok := msg.(TransitionMsg)
	if !ok {
		t.Fatal("TransitionCmd should return TransitionMsg")
	}
	if trans.Next != phase {
		t.Error("TransitionMsg.Next should be the phase")
	}
	if !trans.PushHistory {
		t.Error("TransitionMsg.PushHistory should be true")
	}
}

func TestBackCmd(t *testing.T) {
	cmd := BackCmd()
	msg := cmd()

	if _, ok := msg.(BackMsg); !ok {
		t.Error("BackCmd should return BackMsg")
	}
}

func TestCloseCmd(t *testing.T) {
	cmd := CloseCmd()
	msg := cmd()

	if _, ok := msg.(CloseMsg); !ok {
		t.Error("CloseCmd should return CloseMsg")
	}
}

func TestMachine_MultipleAdvanceAndBack(t *testing.T) {
	phases := make([]*mockPhase, 5)
	for i := range phases {
		phases[i] = &mockPhase{name: string(rune('A' + i)), canGoBack: true}
	}

	m := NewMachine(phases[0])
	for i := 1; i < 5; i++ {
		m.Advance(phases[i])
	}

	if m.HistoryDepth() != 4 {
		t.Errorf("HistoryDepth() = %d, want 4", m.HistoryDepth())
	}

	// Go back all the way
	for i := 4; i > 0; i-- {
		if m.Current().Name() != string(rune('A'+i)) {
			t.Errorf("Current().Name() = %s, want %s", m.Current().Name(), string(rune('A'+i)))
		}
		m.Back()
	}

	if m.Current() != phases[0] {
		t.Error("Should be back at initial phase")
	}
	if m.HistoryDepth() != 0 {
		t.Errorf("HistoryDepth() = %d, want 0", m.HistoryDepth())
	}
}

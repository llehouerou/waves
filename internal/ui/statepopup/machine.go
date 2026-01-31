package statepopup

import tea "github.com/charmbracelet/bubbletea"

// Machine manages phase transitions with history for back-navigation.
// It wraps a current phase and maintains a stack of previous phases.
type Machine struct {
	current Phase
	history []Phase
}

// NewMachine creates a state machine starting at the given phase.
func NewMachine(initial Phase) *Machine {
	return &Machine{
		current: initial,
		history: make([]Phase, 0, 4), // Pre-allocate for typical depth
	}
}

// Current returns the active phase.
func (m *Machine) Current() Phase {
	return m.current
}

// Update delegates to the current phase and handles transitions.
// Returns true if the popup should close.
func (m *Machine) Update(msg tea.Msg) (closed bool, cmd tea.Cmd) {
	if m.current == nil {
		return true, nil
	}

	// Handle back message
	if _, ok := msg.(BackMsg); ok {
		if m.Back() {
			return false, nil
		}
		// Can't go back - stay on current phase
		return false, nil
	}

	// Handle close message
	if _, ok := msg.(CloseMsg); ok {
		return true, nil
	}

	// Delegate to current phase
	next, cmd := m.current.Update(msg)

	// Handle transition message from phase
	if trans, ok := unwrapTransition(cmd); ok {
		if trans.Next == nil {
			return true, nil // Close requested
		}
		if trans.PushHistory {
			m.history = append(m.history, m.current)
		}
		m.current = trans.Next
		return false, nil
	}

	// Handle direct phase transition
	if next == nil {
		return true, nil // Close requested
	}
	if next != m.current {
		// Phase changed - push to history
		m.history = append(m.history, m.current)
		m.current = next
	}

	return false, cmd
}

// unwrapTransition checks if a command returns a TransitionMsg.
// This is a helper to handle transition commands synchronously.
func unwrapTransition(cmd tea.Cmd) (TransitionMsg, bool) {
	if cmd == nil {
		return TransitionMsg{}, false
	}
	// Execute the command to see if it's a transition
	// Note: This only works for immediate commands, not async ones
	msg := cmd()
	if trans, ok := msg.(TransitionMsg); ok {
		return trans, true
	}
	return TransitionMsg{}, false
}

// Advance moves to a new phase, pushing the current phase to history.
func (m *Machine) Advance(next Phase) {
	if m.current != nil {
		m.history = append(m.history, m.current)
	}
	m.current = next
}

// Back returns to the previous phase if history exists and current allows it.
// Returns true if back navigation occurred.
func (m *Machine) Back() bool {
	if !m.CanGoBack() {
		return false
	}
	// Pop from history
	m.current = m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	return true
}

// CanGoBack returns true if back navigation is possible.
// Requires history and current phase allowing it.
func (m *Machine) CanGoBack() bool {
	if len(m.history) == 0 {
		return false
	}
	if m.current == nil {
		return false
	}
	return m.current.CanGoBack()
}

// HistoryDepth returns the number of phases in history.
func (m *Machine) HistoryDepth() int {
	return len(m.history)
}

// View returns the current phase's view.
func (m *Machine) View() string {
	if m.current == nil {
		return ""
	}
	return m.current.View()
}

// Reset clears history and sets a new current phase.
func (m *Machine) Reset(initial Phase) {
	m.current = initial
	m.history = m.history[:0] // Clear but keep capacity
}

// TransitionCmd creates a command that signals a phase transition.
func TransitionCmd(next Phase, pushHistory bool) tea.Cmd {
	return func() tea.Msg {
		return TransitionMsg{Next: next, PushHistory: pushHistory}
	}
}

// BackCmd creates a command that signals back navigation.
func BackCmd() tea.Cmd {
	return func() tea.Msg {
		return BackMsg{}
	}
}

// CloseCmd creates a command that signals the popup should close.
func CloseCmd() tea.Cmd {
	return func() tea.Msg {
		return CloseMsg{}
	}
}

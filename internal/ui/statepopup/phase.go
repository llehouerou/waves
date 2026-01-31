// Package statepopup provides a generic state machine framework for multi-phase popups.
//
// Many popups follow a similar pattern: linear phase progression with back-navigation.
// For example: Search → Select Release Group → Select Release → Confirm.
// This package provides reusable infrastructure for such workflows.
package statepopup

import tea "github.com/charmbracelet/bubbletea"

// Phase represents a step in a multi-phase popup workflow.
// Each phase handles its own state, key input, and rendering.
type Phase interface {
	// Name returns the phase identifier for debugging/logging.
	Name() string

	// Update handles a message and returns the next phase and command.
	// Return the same phase to stay, or a different phase to transition.
	// Return nil phase to indicate the popup should close.
	Update(msg tea.Msg) (Phase, tea.Cmd)

	// View renders the phase content.
	View() string

	// CanGoBack returns true if back-navigation is allowed from this phase.
	// Loading states typically return false.
	CanGoBack() bool
}

// TransitionMsg signals a phase transition.
// Phases can return this to request the machine handle the transition.
type TransitionMsg struct {
	// Next is the phase to transition to.
	// If nil, the popup should close.
	Next Phase

	// PushHistory indicates whether to push the current phase to history.
	// Set to true for forward navigation, false for back navigation.
	PushHistory bool
}

// BackMsg signals that the user wants to go back to the previous phase.
type BackMsg struct{}

// CloseMsg signals that the popup should close.
type CloseMsg struct{}

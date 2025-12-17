// Package action defines the interface for UI component actions.
package action

import tea "github.com/charmbracelet/bubbletea"

// Action represents an action from a UI component.
// The ActionType method returns a string identifier for logging/debugging.
type Action interface {
	ActionType() string
}

// Msg wraps a UI action with its source component name.
// This is the standard way for UI components to communicate with the app.
type Msg struct {
	Source string // Component name: "queuepanel", "albumview", etc.
	Action Action
}

// Ensure Msg implements tea.Msg (compile-time check).
var _ tea.Msg = Msg{}

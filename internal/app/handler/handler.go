// Package handler provides a result type and chain function for key handlers.
package handler

import tea "github.com/charmbracelet/bubbletea"

// Result represents the outcome of a key handler.
type Result struct {
	Handled bool
	Cmd     tea.Cmd
}

// NotHandled is returned when a handler doesn't handle the key.
var NotHandled = Result{}

// Handled creates a Result indicating the key was handled with a command.
func Handled(cmd tea.Cmd) Result {
	return Result{Handled: true, Cmd: cmd}
}

// HandledNoCmd is a convenience for handlers that handle but return no command.
var HandledNoCmd = Result{Handled: true}

// Handler is a function that attempts to handle a key.
type Handler func() Result

// Chain runs handlers in order until one handles the key.
func Chain(handlers ...Handler) (bool, tea.Cmd) {
	for _, h := range handlers {
		if r := h(); r.Handled {
			return true, r.Cmd
		}
	}
	return false, nil
}

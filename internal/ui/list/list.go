// Package list provides a generic scrollable list component.
package list

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

// Action represents what happened during Update.
type Action int

const (
	ActionNone        Action = iota
	ActionEnter              // Enter key pressed
	ActionClick              // Left click (cursor moved to clicked row)
	ActionMiddleClick        // Middle click
	ActionDelete             // d or delete key
)

// Result is returned from Update to tell the parent what happened.
type Result struct {
	Action Action
	Index  int // Which item index the action applies to (-1 if none)
}

// Model is a generic scrollable list component.
// It handles navigation and mouse input, returning actions for the parent to handle.
// The parent is responsible for rendering using VisibleRange().
type Model[T any] struct {
	ui.Base
	items  []T
	cursor cursor.Cursor
}

// New creates a new list with the given scroll margin.
func New[T any](margin int) Model[T] {
	return Model[T]{
		cursor: cursor.New(margin),
	}
}

// SetItems replaces all items and clamps cursor to bounds.
func (m *Model[T]) SetItems(items []T) {
	m.items = items
	m.cursor.ClampToBounds(len(items))
}

// Items returns the current items slice.
func (m Model[T]) Items() []T {
	return m.items
}

// Len returns the number of items.
func (m Model[T]) Len() int {
	return len(m.items)
}

// Selected returns the currently selected item and true, or zero value and false if empty.
func (m Model[T]) Selected() (T, bool) {
	if len(m.items) == 0 || m.cursor.Pos() >= len(m.items) {
		var zero T
		return zero, false
	}
	return m.items[m.cursor.Pos()], true
}

// SelectedIndex returns the current cursor position.
func (m Model[T]) SelectedIndex() int {
	return m.cursor.Pos()
}

// VisibleRange returns [start, end) indices for rendering.
func (m Model[T]) VisibleRange(overhead int) (start, end int) {
	return m.cursor.VisibleRange(len(m.items), m.ListHeight(overhead))
}

// Cursor returns the underlying cursor for advanced use cases.
func (m *Model[T]) Cursor() *cursor.Cursor {
	return &m.cursor
}

// Update handles tea.Msg and returns the action that occurred.
// The listLen parameter specifies the number of items for navigation bounds.
// Use Len() if items are stored in the list, or pass an external count.
func (m *Model[T]) Update(msg tea.Msg, listLen int) Result {
	if !m.IsFocused() {
		return Result{Index: -1}
	}

	height := m.ListHeight(ui.PanelOverhead)

	switch msg := msg.(type) {
	case tea.MouseMsg:
		result, row := m.cursor.HandleMouse(msg, listLen, height, ui.PanelOverhead-1)
		switch result { //nolint:exhaustive // Only handling specific results
		case cursor.MouseScrolled:
			return Result{Index: -1}
		case cursor.MouseClicked:
			return Result{Action: ActionClick, Index: row}
		case cursor.MouseMiddleClick:
			return Result{Action: ActionMiddleClick, Index: row}
		}

	case tea.KeyMsg:
		if m.cursor.HandleKey(msg.String(), listLen, height) {
			return Result{Index: -1}
		}
		switch msg.String() {
		case "enter":
			if listLen > 0 {
				return Result{Action: ActionEnter, Index: m.cursor.Pos()}
			}
		case "d", "delete":
			if listLen > 0 {
				return Result{Action: ActionDelete, Index: m.cursor.Pos()}
			}
		}
	}

	return Result{Index: -1}
}

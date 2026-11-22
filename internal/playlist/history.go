package playlist

import "slices"

// QueueHistory maintains a history of list states for undo/redo: the queue
// records its entries in it.
type QueueHistory[T any] struct {
	states  [][]T
	current int // index of current state (-1 = before any state)
	maxSize int
}

// NewQueueHistory creates a new history with the given maximum size.
func NewQueueHistory[T any](maxSize int) *QueueHistory[T] {
	return &QueueHistory[T]{
		states:  make([][]T, 0, maxSize),
		current: -1,
		maxSize: maxSize,
	}
}

// Push saves a snapshot of the list.
// Clears any redo states and trims if over limit.
func (h *QueueHistory[T]) Push(list []T) {
	snapshot := make([]T, len(list))
	copy(snapshot, list)

	// Clear redo states (everything after current)
	if h.current < len(h.states)-1 {
		h.states = h.states[:h.current+1]
	}

	// Append new state
	h.states = append(h.states, snapshot)
	h.current = len(h.states) - 1

	// Trim if over limit
	if len(h.states) > h.maxSize {
		excess := len(h.states) - h.maxSize
		h.states = h.states[excess:]
		h.current -= excess
	}
}

// Each passes every element of every list it recorded to f.
func (h *QueueHistory[T]) Each(f func(T)) {
	for _, state := range h.states {
		for _, e := range state {
			f(e)
		}
	}
}

// Rewrite passes every element of every list it recorded through rewrite, in
// place. It is no new state: undo and redo walk the same lists, rewritten.
func (h *QueueHistory[T]) Rewrite(rewrite func(T) T) {
	for _, state := range h.states {
		for i := range state {
			state[i] = rewrite(state[i])
		}
	}
}

// Undo returns a copy of the previous list state.
// Returns nil and false if nothing to undo.
func (h *QueueHistory[T]) Undo() ([]T, bool) {
	if !h.CanUndo() {
		return nil, false
	}
	h.current--
	return slices.Clone(h.states[h.current]), true
}

// Redo returns a copy of the next list state.
// Returns nil and false if nothing to redo.
func (h *QueueHistory[T]) Redo() ([]T, bool) {
	if !h.CanRedo() {
		return nil, false
	}
	h.current++
	return slices.Clone(h.states[h.current]), true
}

// CanUndo returns true if there is a previous state to undo to.
func (h *QueueHistory[T]) CanUndo() bool {
	return h.current > 0
}

// CanRedo returns true if there is a next state to redo to.
func (h *QueueHistory[T]) CanRedo() bool {
	return h.current < len(h.states)-1
}

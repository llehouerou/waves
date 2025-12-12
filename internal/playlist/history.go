package playlist

// QueueHistory maintains a history of track list states for undo/redo.
type QueueHistory struct {
	states  [][]Track
	current int // index of current state (-1 = before any state)
	maxSize int
}

// NewQueueHistory creates a new history with the given maximum size.
func NewQueueHistory(maxSize int) *QueueHistory {
	return &QueueHistory{
		states:  make([][]Track, 0, maxSize),
		current: -1,
		maxSize: maxSize,
	}
}

// Push saves a snapshot of the track list.
// Clears any redo states and trims if over limit.
func (h *QueueHistory) Push(tracks []Track) {
	// Make a deep copy
	snapshot := make([]Track, len(tracks))
	copy(snapshot, tracks)

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

// Undo returns the previous track list state.
// Returns nil and false if nothing to undo.
func (h *QueueHistory) Undo() ([]Track, bool) {
	if !h.CanUndo() {
		return nil, false
	}
	h.current--
	// Return a copy
	snapshot := make([]Track, len(h.states[h.current]))
	copy(snapshot, h.states[h.current])
	return snapshot, true
}

// Redo returns the next track list state.
// Returns nil and false if nothing to redo.
func (h *QueueHistory) Redo() ([]Track, bool) {
	if !h.CanRedo() {
		return nil, false
	}
	h.current++
	// Return a copy
	snapshot := make([]Track, len(h.states[h.current]))
	copy(snapshot, h.states[h.current])
	return snapshot, true
}

// CanUndo returns true if there is a previous state to undo to.
func (h *QueueHistory) CanUndo() bool {
	return h.current > 0
}

// CanRedo returns true if there is a next state to redo to.
func (h *QueueHistory) CanRedo() bool {
	return h.current < len(h.states)-1
}

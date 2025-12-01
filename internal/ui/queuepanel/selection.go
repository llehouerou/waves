package queuepanel

// SyncCursor moves the cursor to the currently playing track.
func (m *Model) SyncCursor() {
	currentIdx := m.queue.CurrentIndex()
	if currentIdx >= 0 && currentIdx < m.queue.Len() {
		m.cursor = currentIdx
		m.ensureCursorVisible()
	}
}

// moveCursor moves the cursor by delta positions and ensures visibility.
func (m *Model) moveCursor(delta int) {
	if m.queue.Len() == 0 {
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= m.queue.Len() {
		m.cursor = m.queue.Len() - 1
	}

	m.ensureCursorVisible()
}

// ensureCursorVisible adjusts the scroll offset to keep the cursor in view.
func (m *Model) ensureCursorVisible() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+listHeight {
		m.offset = m.cursor - listHeight + 1
	}
}

// clearSelection removes all selected items.
func (m *Model) clearSelection() {
	m.selected = make(map[int]bool)
}

// moveSelected moves selected items (or cursor item) by delta positions.
// Returns true if the move was performed.
func (m *Model) moveSelected(delta int) bool {
	if m.queue.Len() == 0 {
		return false
	}

	// Get indices to move (selected or cursor)
	var indices []int
	if len(m.selected) > 0 {
		indices = make([]int, 0, len(m.selected))
		for idx := range m.selected {
			indices = append(indices, idx)
		}
	} else {
		indices = []int{m.cursor}
	}

	// Perform the move
	newIndices, moved := m.queue.MoveIndices(indices, delta)
	if !moved {
		return false
	}

	// Update selection with new indices
	if len(m.selected) > 0 {
		m.selected = make(map[int]bool)
		for _, idx := range newIndices {
			m.selected[idx] = true
		}
	}

	// Move cursor along with the selection
	m.cursor += delta
	m.ensureCursorVisible()
	return true
}

// deleteSelected removes selected items (or cursor item) from the queue.
func (m *Model) deleteSelected() {
	// If we have a selection, delete selected items
	// Otherwise delete just the cursor item
	if len(m.selected) == 0 {
		m.selected[m.cursor] = true
	}

	// Get sorted indices in descending order to delete from end first
	indices := make([]int, 0, len(m.selected))
	for idx := range m.selected {
		indices = append(indices, idx)
	}
	// Sort descending
	for i := range indices {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] > indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Delete from highest index first
	for _, idx := range indices {
		m.queue.RemoveAt(idx)
	}

	// Clear selection
	m.selected = make(map[int]bool)

	// Adjust cursor if it's now past the end
	if m.cursor >= m.queue.Len() && m.cursor > 0 {
		m.cursor = m.queue.Len() - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

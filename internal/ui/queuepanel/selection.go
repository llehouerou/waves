package queuepanel

// SyncCursor moves the cursor to the currently playing track.
func (m *Model) SyncCursor() {
	currentIdx := m.queue.CurrentIndex()
	if currentIdx >= 0 && currentIdx < m.queue.Len() {
		m.list.Cursor().Jump(currentIdx, m.queue.Len(), m.listHeight())
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
		indices = []int{m.list.Cursor().Pos()}
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
	m.list.Cursor().Move(delta, m.queue.Len(), m.listHeight())
	return true
}

// keepOnlySelected removes all items except selected ones from the queue.
func (m *Model) keepOnlySelected() {
	if len(m.selected) == 0 {
		return
	}

	// Get indices to delete (all unselected items)
	queueLen := m.queue.Len()
	indices := make([]int, 0, queueLen-len(m.selected))
	for i := range queueLen {
		if !m.selected[i] {
			indices = append(indices, i)
		}
	}

	// Sort descending to delete from end first
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

	// Clear selection and reset cursor
	m.selected = make(map[int]bool)
	m.list.Cursor().Reset()
}

// clearExceptPlaying removes all items except the currently playing track.
func (m *Model) clearExceptPlaying() {
	currentIdx := m.queue.CurrentIndex()
	if currentIdx < 0 {
		// No track playing, clear everything
		m.queue.Clear()
		m.list.Cursor().Reset()
		m.selected = make(map[int]bool)
		return
	}

	// Delete all items except the currently playing one
	// Delete from highest index first to avoid shifting issues
	for i := m.queue.Len() - 1; i >= 0; i-- {
		if i != currentIdx {
			m.queue.RemoveAt(i)
		}
	}

	// Reset cursor and selection
	m.list.Cursor().Reset()
	m.selected = make(map[int]bool)
}

// deleteSelected removes selected items (or cursor item) from the queue.
func (m *Model) deleteSelected() {
	// If we have a selection, delete selected items
	// Otherwise delete just the cursor item
	if len(m.selected) == 0 {
		m.selected[m.list.Cursor().Pos()] = true
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
	m.list.Cursor().ClampToBounds(m.queue.Len())
	m.list.Cursor().EnsureVisible(m.queue.Len(), m.listHeight())
}

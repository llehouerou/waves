package queuepanel

import (
	"maps"
	"slices"
)

// SyncCursor moves the cursor to the currently playing track.
func (m *Model) SyncCursor() {
	currentIdx := m.queue.QueueCurrentIndex()
	if n := m.queue.QueueLen(); currentIdx >= 0 && currentIdx < n {
		m.list.Cursor().Jump(currentIdx, n, m.listHeight())
	}
}

// clearSelection removes all selected items.
func (m *Model) clearSelection() {
	m.selected = make(map[int]bool)
}

// targetIndices returns the selected indices in order, or the cursor's when
// nothing is selected.
func (m Model) targetIndices() []int {
	if len(m.selected) > 0 {
		return slices.Sorted(maps.Keys(m.selected))
	}
	return []int{m.list.Cursor().Pos()}
}

// indicesExcept returns every queue index but the kept ones.
func (m Model) indicesExcept(keep func(int) bool) []int {
	var others []int
	for i := range m.queue.QueueLen() {
		if !keep(i) {
			others = append(others, i)
		}
	}
	return others
}

// moveSelected moves the selected items (or the cursor item) by delta,
// carrying the selection and cursor along. Returns false when the move would
// push an item out of the queue.
func (m *Model) moveSelected(delta int) (MoveTracks, bool) {
	indices := m.targetIndices()
	n := m.queue.QueueLen()
	if n == 0 || indices[0]+delta < 0 || indices[len(indices)-1]+delta >= n {
		return MoveTracks{}, false
	}

	if len(m.selected) > 0 {
		m.selected = make(map[int]bool)
		for _, idx := range indices {
			m.selected[idx+delta] = true
		}
	}
	m.list.Cursor().Move(delta, n, m.listHeight())
	return MoveTracks{Indices: indices, Delta: delta}, true
}

// keepOnlySelected removes all items except the selected ones.
func (m *Model) keepOnlySelected() RemoveTracks {
	others := m.indicesExcept(func(i int) bool { return m.selected[i] })
	m.clearSelection()
	m.list.Cursor().Reset()
	return RemoveTracks{Indices: others}
}

// clearExceptPlaying removes all items except the currently playing track.
func (m *Model) clearExceptPlaying() RemoveTracks {
	current := m.queue.QueueCurrentIndex()
	others := m.indicesExcept(func(i int) bool { return i == current })
	m.clearSelection()
	m.list.Cursor().Reset()
	return RemoveTracks{Indices: others}
}

// deleteSelected removes the selected items (or the cursor item), leaving the
// cursor where it was, within the shorter queue.
func (m *Model) deleteSelected() RemoveTracks {
	indices := m.targetIndices()
	m.clearSelection()
	remaining := m.queue.QueueLen() - len(indices)
	m.list.Cursor().ClampToBounds(remaining)
	m.list.Cursor().EnsureVisible(remaining, m.listHeight())
	return RemoveTracks{Indices: indices}
}

package queuepanel

import (
	"github.com/llehouerou/waves/internal/core"
)

// SyncCursor moves the cursor to the currently playing track.
func (m *Model) SyncCursor() {
	currentIdx := m.queue.QueueCurrentIndex()
	if n := m.queue.QueueLen(); currentIdx >= 0 && currentIdx < n {
		m.list.Cursor().Jump(currentIdx, n, m.listHeight())
	}
}

// keepCursorVisible scrolls so the cursor keeps its margin in view: tracks
// added after it, by radio, show up.
func (m *Model) keepCursorVisible() {
	m.list.Cursor().EnsureVisible(m.queue.QueueLen(), m.listHeight())
}

// QueueChanged reacts to a queue changed under the panel: it drops the marks
// of the entries that left the queue, for good (an undo or a redo that brings
// an entry back does not bring its mark back), and keeps the cursor in view.
// It runs on the queue-changed event, and right after every edit the app
// makes itself: a mark is then dropped even when its entry comes back before
// the event.
func (m *Model) QueueChanged() {
	entries := m.queue.QueueEntries()
	kept := make(map[core.EntryID]bool, len(m.selected))
	for _, row := range m.markedRows(entries) {
		kept[entries[row].ID] = true
	}
	m.selected = kept
	m.keepCursorVisible()
}

// HasSelection reports whether an entry of the queue is marked.
func (m Model) HasSelection() bool {
	return len(m.selected) > 0 && len(m.markedRows(m.queue.QueueEntries())) > 0
}

// clearSelection removes all marks.
func (m *Model) clearSelection() {
	m.selected = make(map[core.EntryID]bool)
}

// markedRows returns the rows of the marked entries, in order.
func (m Model) markedRows(entries []core.QueueEntry) []int {
	var rows []int
	for i := range entries {
		if m.selected[entries[i].ID] {
			rows = append(rows, i)
		}
	}
	return rows
}

// targetRows returns the rows of the marked entries, or the cursor's when
// none is marked.
func (m Model) targetRows(entries []core.QueueEntry) []int {
	if rows := m.markedRows(entries); len(rows) > 0 {
		return rows
	}
	return []int{m.list.Cursor().Pos()}
}

// rowsExcept returns every row of a queue of n tracks but the kept ones.
func rowsExcept(n int, keep func(int) bool) []int {
	var others []int
	for i := range n {
		if !keep(i) {
			others = append(others, i)
		}
	}
	return others
}

// moveSelected moves the marked entries (or the cursor's) by delta, the cursor
// along; the marks stay on their entries. Returns false when the move would
// push an entry out of the queue.
func (m *Model) moveSelected(delta int) (MoveTracks, bool) {
	entries := m.queue.QueueEntries()
	rows := m.targetRows(entries)
	n := len(entries)
	if n == 0 || rows[0]+delta < 0 || rows[len(rows)-1]+delta >= n {
		return MoveTracks{}, false
	}
	m.list.Cursor().Move(delta, n, m.listHeight())
	return MoveTracks{Indices: rows, Delta: delta}, true
}

// keepOnlySelected removes all entries except the marked ones.
func (m *Model) keepOnlySelected() RemoveTracks {
	entries := m.queue.QueueEntries()
	others := rowsExcept(len(entries), func(i int) bool { return m.selected[entries[i].ID] })
	m.clearSelection()
	m.list.Cursor().Reset()
	return RemoveTracks{Indices: others}
}

// clearExceptPlaying removes all items except the currently playing track.
func (m *Model) clearExceptPlaying() RemoveTracks {
	current := m.queue.QueueCurrentIndex()
	others := rowsExcept(m.queue.QueueLen(), func(i int) bool { return i == current })
	m.clearSelection()
	m.list.Cursor().Reset()
	return RemoveTracks{Indices: others}
}

// deleteSelected removes the marked entries (or the cursor's), leaving the
// cursor where it was, within the shorter queue.
func (m *Model) deleteSelected() RemoveTracks {
	entries := m.queue.QueueEntries()
	rows := m.targetRows(entries)
	m.clearSelection()
	// The cursor may predate a queue change not yet seen: never go below 0.
	remaining := max(len(entries)-len(rows), 0)
	m.list.Cursor().ClampToBounds(remaining)
	m.list.Cursor().EnsureVisible(remaining, m.listHeight())
	return RemoveTracks{Indices: rows}
}

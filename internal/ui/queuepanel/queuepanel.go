package queuepanel

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui"
)

// JumpToTrackMsg is sent when the user selects a track to jump to.
type JumpToTrackMsg struct {
	Index int
}

// QueueChangedMsg is sent when the queue is modified (delete, move).
type QueueChangedMsg struct{}

// Model represents the queue panel state.
type Model struct {
	queue    *playlist.PlayingQueue
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
	selected map[int]bool
}

// New creates a new queue panel model.
func New(queue *playlist.PlayingQueue) Model {
	return Model{
		queue:    queue,
		cursor:   0,
		offset:   0,
		selected: make(map[int]bool),
	}
}

// SetFocused sets whether the panel is focused.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the panel is focused.
func (m Model) IsFocused() bool {
	return m.focused
}

// SetSize sets the panel dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the queue panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if !m.focused {
		return m, nil
	}

	switch keyMsg.String() {
	case "x":
		// Toggle selection on current item
		if m.queue.Len() > 0 && m.cursor < m.queue.Len() {
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
		}
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "g":
		m.cursor = 0
		m.offset = 0
	case "G":
		if m.queue.Len() > 0 {
			m.cursor = m.queue.Len() - 1
			m.ensureCursorVisible()
		}
	case "enter":
		if m.queue.Len() > 0 && m.cursor < m.queue.Len() {
			m.clearSelection()
			return m, func() tea.Msg {
				return JumpToTrackMsg{Index: m.cursor}
			}
		}
	case "d", "delete":
		if m.queue.Len() > 0 {
			m.deleteSelected()
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	case "esc":
		if len(m.selected) > 0 {
			m.clearSelection()
		}
	case "shift+j", "shift+down":
		if m.moveSelected(1) {
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	case "shift+k", "shift+up":
		if m.moveSelected(-1) {
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	}

	return m, nil
}

// SyncCursor moves the cursor to the currently playing track.
func (m *Model) SyncCursor() {
	currentIdx := m.queue.CurrentIndex()
	if currentIdx >= 0 && currentIdx < m.queue.Len() {
		m.cursor = currentIdx
		m.ensureCursorVisible()
	}
}

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

func (m *Model) clearSelection() {
	m.selected = make(map[int]bool)
}

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

func (m Model) listHeight() int {
	// Account for border + header + separator
	return m.height - ui.PanelOverhead
}
